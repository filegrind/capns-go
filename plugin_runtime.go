package capns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/filegrind/cap-sdk-go/cbor"
)

// StreamEmitter allows handlers to emit chunked responses and logs
type StreamEmitter interface {
	// Emit sends a CHUNK frame with the given data
	Emit(payload []byte)
	// Log sends a LOG frame at the given level
	Log(level, message string)
	// EmitStatus sends a status update (as a LOG frame)
	EmitStatus(operation, details string)
}

// PeerInvoker allows handlers to invoke caps on the peer (host)
type PeerInvoker interface {
	// Invoke sends a REQ frame to the host and returns a channel that yields response chunks
	Invoke(capUrn string, arguments []CapArgumentValue) (<-chan InvokeResult, error)
}

// InvokeResult represents a chunk or error from peer invocation
type InvokeResult struct {
	Data  []byte
	Error error
}

// HandlerFunc is the function signature for cap handlers
// Receives request payload bytes, emitter, and peer invoker; returns response payload bytes
type HandlerFunc func(payload []byte, emitter StreamEmitter, peer PeerInvoker) ([]byte, error)

// PluginRuntime handles all I/O for plugin binaries
type PluginRuntime struct {
	handlers     map[string]HandlerFunc
	manifestData []byte
	manifest     *CapManifest
	limits       cbor.Limits
	mu           sync.RWMutex
}

// NewPluginRuntime creates a new plugin runtime with the required manifest JSON
func NewPluginRuntime(manifestJSON []byte) (*PluginRuntime, error) {
	// Try to parse the manifest for CLI mode support
	var manifest CapManifest
	parseErr := json.Unmarshal(manifestJSON, &manifest)

	runtime := &PluginRuntime{
		handlers:     make(map[string]HandlerFunc),
		manifestData: manifestJSON,
		limits:       cbor.DefaultLimits(),
	}

	if parseErr == nil {
		runtime.manifest = &manifest
	}

	return runtime, nil
}

// NewPluginRuntimeWithManifest creates a new plugin runtime with a pre-built CapManifest
func NewPluginRuntimeWithManifest(manifest *CapManifest) (*PluginRuntime, error) {
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return &PluginRuntime{
		handlers:     make(map[string]HandlerFunc),
		manifestData: manifestData,
		manifest:     manifest,
		limits:       cbor.DefaultLimits(),
	}, nil
}

// Register registers a handler for a cap URN
func (pr *PluginRuntime) Register(capUrn string, handler HandlerFunc) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.handlers[capUrn] = handler
}

// FindHandler finds a handler for a cap URN (exact match or pattern match)
func (pr *PluginRuntime) FindHandler(capUrn string) HandlerFunc {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	// First try exact match
	if handler, ok := pr.handlers[capUrn]; ok {
		return handler
	}

	// Then try pattern matching via CapUrn
	requestUrn, err := NewCapUrnFromString(capUrn)
	if err != nil {
		return nil
	}

	for pattern, handler := range pr.handlers {
		patternUrn, err := NewCapUrnFromString(pattern)
		if err != nil {
			continue
		}
		if patternUrn.Accepts(requestUrn) {
			return handler
		}
	}

	return nil
}

// Run runs the plugin runtime (automatic mode detection)
func (pr *PluginRuntime) Run() error {
	args := os.Args

	// No CLI arguments at all → Plugin CBOR mode
	if len(args) == 1 {
		return pr.runCBORMode()
	}

	// Any CLI arguments → CLI mode
	return pr.runCLIMode(args)
}

// runCBORMode runs in Plugin CBOR mode - binary frame protocol via stdin/stdout
func (pr *PluginRuntime) runCBORMode() error {
	reader := cbor.NewFrameReader(os.Stdin)
	writer := cbor.NewFrameWriter(os.Stdout)

	// Perform handshake - send our manifest in the HELLO response
	negotiatedLimits, err := cbor.HandshakeAccept(reader, writer, pr.manifestData)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	reader.SetLimits(negotiatedLimits)
	writer.SetLimits(negotiatedLimits)

	pr.mu.Lock()
	pr.limits = negotiatedLimits
	pr.mu.Unlock()

	// Track pending peer requests (plugin invoking host caps)
	pendingPeerRequests := &sync.Map{} // map[MessageId]*pendingPeerRequest

	// Track active handler goroutines for cleanup
	var activeHandlers sync.WaitGroup

	// Main event loop
	for {
		frame, err := reader.ReadFrame()
		if err != nil {
			if err == io.EOF {
				break // stdin closed, exit cleanly
			}
			return fmt.Errorf("failed to read frame: %w", err)
		}

		switch frame.FrameType {
		case cbor.FrameTypeReq:
			if frame.Cap == nil || *frame.Cap == "" {
				errFrame := cbor.NewErr(frame.Id, "INVALID_REQUEST", "Request missing cap URN")
				if writeErr := writer.WriteFrame(errFrame); writeErr != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", writeErr)
				}
				continue
			}

			handler := pr.FindHandler(*frame.Cap)
			if handler == nil {
				errFrame := cbor.NewErr(frame.Id, "NO_HANDLER", fmt.Sprintf("No handler registered for cap: %s", *frame.Cap))
				if writeErr := writer.WriteFrame(errFrame); writeErr != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", writeErr)
				}
				continue
			}

			// Clone what we need for the handler goroutine
			requestID := frame.Id
			capUrn := *frame.Cap
			rawPayload := frame.Payload
			var contentType string
			if frame.ContentType != nil {
				contentType = *frame.ContentType
			}
			maxChunk := negotiatedLimits.MaxChunk

			// Spawn handler in separate goroutine - main loop continues immediately
			activeHandlers.Add(1)
			go func() {
				defer activeHandlers.Done()

				emitter := newThreadSafeEmitter(writer, requestID)
				peerInvoker := newPeerInvokerImpl(writer, pendingPeerRequests)

				// Extract effective payload from unified arguments if content_type is CBOR
				payload, err := extractEffectivePayload(rawPayload, contentType, capUrn)
				if err != nil {
					errFrame := cbor.NewErr(requestID, "PAYLOAD_ERROR", err.Error())
					if writeErr := writer.WriteFrame(errFrame); writeErr != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", writeErr)
					}
					return
				}

				result, err := handler(payload, emitter, peerInvoker)

				// Send response with automatic chunking for large payloads
				if err != nil {
					errFrame := cbor.NewErr(requestID, "HANDLER_ERROR", err.Error())
					if writeErr := writer.WriteFrame(errFrame); writeErr != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", writeErr)
					}
					return
				}

				// Automatic chunking: split large payloads into CHUNK frames
				if len(result) <= maxChunk {
					// Small payload: send single END frame
					endFrame := cbor.NewEnd(requestID, result)
					if writeErr := writer.WriteFrame(endFrame); writeErr != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write END frame: %v\n", writeErr)
					}
				} else {
					// Large payload: send CHUNK frames + final END
					offset := 0
					seq := uint64(0)

					for offset < len(result) {
						remaining := len(result) - offset
						chunkSize := remaining
						if chunkSize > maxChunk {
							chunkSize = maxChunk
						}
						chunkData := result[offset : offset+chunkSize]
						offset += chunkSize

						if offset < len(result) {
							// Not the last chunk - send CHUNK frame
							chunkFrame := cbor.NewChunk(requestID, seq, chunkData)
							if writeErr := writer.WriteFrame(chunkFrame); writeErr != nil {
								fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write CHUNK frame: %v\n", writeErr)
								return
							}
							seq++
						} else {
							// Last chunk - send END frame with remaining data
							endFrame := cbor.NewEnd(requestID, chunkData)
							if writeErr := writer.WriteFrame(endFrame); writeErr != nil {
								fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write END frame: %v\n", writeErr)
							}
						}
					}
				}
			}()

		case cbor.FrameTypeHeartbeat:
			// Respond to heartbeat immediately - never blocked by handlers
			response := cbor.NewHeartbeat(frame.Id)
			if err := writer.WriteFrame(response); err != nil {
				return fmt.Errorf("failed to write heartbeat response: %w", err)
			}

		case cbor.FrameTypeHello:
			// Unexpected HELLO after handshake - protocol error
			errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "Unexpected HELLO after handshake")
			if err := writer.WriteFrame(errFrame); err != nil {
				return fmt.Errorf("failed to write error: %w", err)
			}

		case cbor.FrameTypeRes, cbor.FrameTypeChunk, cbor.FrameTypeEnd:
			// Response frames from host - route to pending peer request by frame.Id
			if pending, ok := pendingPeerRequests.Load(frame.Id); ok {
				pendingReq := pending.(*pendingPeerRequest)
				pendingReq.sender <- InvokeResult{Data: frame.Payload, Error: nil}
			}

			// Remove completed requests (RES or END frame marks completion)
			if frame.FrameType == cbor.FrameTypeRes || frame.FrameType == cbor.FrameTypeEnd {
				if pending, ok := pendingPeerRequests.LoadAndDelete(frame.Id); ok {
					pendingReq := pending.(*pendingPeerRequest)
					close(pendingReq.sender)
				}
			}

		case cbor.FrameTypeErr:
			// Error frame from host - could be response to peer request
			if pending, ok := pendingPeerRequests.LoadAndDelete(frame.Id); ok {
				pendingReq := pending.(*pendingPeerRequest)
				code := frame.ErrorCode()
				message := frame.ErrorMessage()
				if code == "" {
					code = "UNKNOWN"
				}
				if message == "" {
					message = "Unknown error"
				}
				pendingReq.sender <- InvokeResult{
					Error: fmt.Errorf("[%s] %s", code, message),
				}
				close(pendingReq.sender)
			}

		case cbor.FrameTypeLog:
			// Log frames from host - shouldn't normally receive these, ignore
			continue
		}
	}

	// Wait for all active handlers to complete before exiting
	activeHandlers.Wait()

	return nil
}

// runCLIMode runs in CLI mode - parse arguments and invoke handler
func (pr *PluginRuntime) runCLIMode(args []string) error {
	if pr.manifest == nil {
		return errors.New("failed to parse manifest for CLI mode")
	}

	// Handle --help at top level
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") {
		pr.printHelp()
		return nil
	}

	subcommand := args[1]

	// Handle manifest subcommand (always provided by runtime)
	if subcommand == "manifest" {
		prettyJSON, err := json.MarshalIndent(pr.manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal manifest: %w", err)
		}
		fmt.Println(string(prettyJSON))
		return nil
	}

	// Handle subcommand --help
	if len(args) == 3 && (args[2] == "--help" || args[2] == "-h") {
		if cap := pr.findCapByCommand(subcommand); cap != nil {
			pr.printCapHelp(cap)
			return nil
		}
	}

	// Find cap by command name
	cap := pr.findCapByCommand(subcommand)
	if cap == nil {
		return fmt.Errorf("unknown subcommand '%s'. Run with --help to see available commands", subcommand)
	}

	// Find handler
	handler := pr.FindHandler(cap.UrnString())
	if handler == nil {
		return fmt.Errorf("no handler registered for cap '%s'", cap.UrnString())
	}

	// Build arguments from CLI (not implemented yet - simplified version)
	// For now, just pass empty payload
	payload := []byte("{}")

	// Create CLI-mode emitter and no-op peer invoker
	emitter := &cliStreamEmitter{}
	peer := &noPeerInvoker{}

	// Invoke handler
	result, err := handler(payload, emitter, peer)
	if err != nil {
		errorJSON, _ := json.Marshal(map[string]string{
			"error": err.Error(),
			"code":  "HANDLER_ERROR",
		})
		fmt.Fprintln(os.Stderr, string(errorJSON))
		return err
	}

	// Output final response if not empty
	if len(result) > 0 {
		fmt.Println(string(result))
	}

	return nil
}

// findCapByCommand finds a cap by its command name
func (pr *PluginRuntime) findCapByCommand(commandName string) *Cap {
	if pr.manifest == nil {
		return nil
	}
	for i := range pr.manifest.Caps {
		if pr.manifest.Caps[i].Command == commandName {
			return &pr.manifest.Caps[i]
		}
	}
	return nil
}

// printHelp prints help message showing all available subcommands
func (pr *PluginRuntime) printHelp() {
	if pr.manifest == nil {
		return
	}

	fmt.Fprintf(os.Stderr, "%s v%s\n", pr.manifest.Name, pr.manifest.Version)
	fmt.Fprintf(os.Stderr, "%s\n\n", pr.manifest.Description)
	fmt.Fprintf(os.Stderr, "USAGE:\n")
	fmt.Fprintf(os.Stderr, "    %s <COMMAND> [OPTIONS]\n\n", pr.manifest.Name)
	fmt.Fprintf(os.Stderr, "COMMANDS:\n")
	fmt.Fprintf(os.Stderr, "    manifest    Output the plugin manifest as JSON\n")

	for i := range pr.manifest.Caps {
		cap := &pr.manifest.Caps[i]
		desc := cap.Title
		if cap.CapDescription != nil {
			desc = *cap.CapDescription
		}
		fmt.Fprintf(os.Stderr, "    %-12s %s\n", cap.Command, desc)
	}

	fmt.Fprintf(os.Stderr, "\nRun '%s <COMMAND> --help' for more information on a command.\n", pr.manifest.Name)
}

// printCapHelp prints help for a specific cap
func (pr *PluginRuntime) printCapHelp(cap *Cap) {
	fmt.Fprintf(os.Stderr, "%s\n", cap.Title)
	if cap.CapDescription != nil {
		fmt.Fprintf(os.Stderr, "%s\n", *cap.CapDescription)
	}
	fmt.Fprintf(os.Stderr, "\nUSAGE:\n")
	fmt.Fprintf(os.Stderr, "    plugin %s [OPTIONS]\n\n", cap.Command)
}

// extractEffectivePayload extracts the effective payload from a REQ frame
func extractEffectivePayload(payload []byte, contentType string, capUrn string) ([]byte, error) {
	// Check if this is CBOR unified arguments
	if contentType != "application/cbor" {
		// Not CBOR unified arguments - return raw payload
		return payload, nil
	}

	// Parse the cap URN to get the expected input media URN
	capUrnParsed, err := NewCapUrnFromString(capUrn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cap URN '%s': %w", capUrn, err)
	}
	_ = capUrnParsed.InSpec()

	// Parse the CBOR payload as an array of argument maps
	// For now, simplified - just return raw payload
	// Full implementation would decode CBOR and match media URNs
	// This will be implemented when we add CBOR decoding logic
	return payload, nil
}

// threadSafeEmitter implements StreamEmitter with thread-safe writes
type threadSafeEmitter struct {
	writer    *cbor.FrameWriter
	requestID cbor.MessageId
	seq       uint64
	seqMu     sync.Mutex
}

func newThreadSafeEmitter(writer *cbor.FrameWriter, requestID cbor.MessageId) *threadSafeEmitter {
	return &threadSafeEmitter{
		writer:    writer,
		requestID: requestID,
	}
}

func (e *threadSafeEmitter) Emit(payload []byte) {
	e.seqMu.Lock()
	currentSeq := e.seq
	e.seq++
	e.seqMu.Unlock()

	frame := cbor.NewChunk(e.requestID, currentSeq, payload)
	if err := e.writer.WriteFrame(frame); err != nil {
		fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write chunk: %v\n", err)
	}
}

func (e *threadSafeEmitter) Log(level, message string) {
	frame := cbor.NewLog(e.requestID, level, message)
	if err := e.writer.WriteFrame(frame); err != nil {
		fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write log: %v\n", err)
	}
}

func (e *threadSafeEmitter) EmitStatus(operation, details string) {
	message := fmt.Sprintf("%s: %s", operation, details)
	frame := cbor.NewLog(e.requestID, "status", message)
	if err := e.writer.WriteFrame(frame); err != nil {
		fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write status: %v\n", err)
	}
}

// cliStreamEmitter implements StreamEmitter for CLI mode
type cliStreamEmitter struct{}

func (e *cliStreamEmitter) Emit(payload []byte) {
	os.Stdout.Write(payload)
	os.Stdout.Write([]byte("\n"))
}

func (e *cliStreamEmitter) Log(level, message string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", level, message)
}

func (e *cliStreamEmitter) EmitStatus(operation, details string) {
	statusJSON, _ := json.Marshal(map[string]string{
		"type":      "status",
		"operation": operation,
		"details":   details,
	})
	fmt.Fprintln(os.Stderr, string(statusJSON))
}

// pendingPeerRequest tracks a pending peer request
type pendingPeerRequest struct {
	sender chan InvokeResult
}

// peerInvokerImpl implements PeerInvoker
type peerInvokerImpl struct {
	writer          *cbor.FrameWriter
	pendingRequests *sync.Map
}

func newPeerInvokerImpl(writer *cbor.FrameWriter, pendingRequests *sync.Map) *peerInvokerImpl {
	return &peerInvokerImpl{
		writer:          writer,
		pendingRequests: pendingRequests,
	}
}

func (p *peerInvokerImpl) Invoke(capUrn string, arguments []CapArgumentValue) (<-chan InvokeResult, error) {
	// Generate a new message ID for this request
	requestID := cbor.NewMessageIdRandom()

	// Create a buffered channel for responses (buffer up to 64 chunks)
	sender := make(chan InvokeResult, 64)

	// Register the pending request before sending
	p.pendingRequests.Store(requestID, &pendingPeerRequest{sender: sender})

	// Serialize arguments as CBOR - binary values stay binary (no base64 needed)
	// For now, simplified - just send empty payload
	// Full implementation would encode CBOR unified arguments
	payload := []byte{}

	// Create and send the REQ frame with CBOR payload
	frame := cbor.NewReq(requestID, capUrn, payload, "application/cbor")

	if err := p.writer.WriteFrame(frame); err != nil {
		p.pendingRequests.Delete(requestID)
		return nil, fmt.Errorf("failed to send REQ frame: %w", err)
	}

	return sender, nil
}

// noPeerInvoker is a no-op PeerInvoker that always returns an error
type noPeerInvoker struct{}

func (n *noPeerInvoker) Invoke(capUrn string, arguments []CapArgumentValue) (<-chan InvokeResult, error) {
	return nil, errors.New("peer invocation not supported in this context")
}

// Limits returns the current protocol limits
func (pr *PluginRuntime) Limits() cbor.Limits {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.limits
}
