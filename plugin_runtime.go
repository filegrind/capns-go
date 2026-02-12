package capns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	cborlib "github.com/fxamacker/cbor/v2"

	"github.com/filegrind/capns-go/cbor"
	taggedurn "github.com/filegrind/tagged-urn-go"
)

// StreamEmitter allows handlers to emit CBOR values and logs.
// Handlers emit CBOR values via EmitCbor() or logs via EmitLog().
// The value is CBOR-encoded once and sent as raw CBOR bytes in CHUNK frames.
// No double-encoding: one CBOR layer from handler to consumer.
type StreamEmitter interface {
	// EmitCbor emits a CBOR value as output.
	// The value is CBOR-encoded once and sent as raw CBOR bytes in CHUNK frames.
	EmitCbor(value interface{}) error
	// EmitLog emits a log message at the given level.
	// Sends a LOG frame (side-channel, does not affect response stream).
	EmitLog(level, message string)
}

// PeerInvoker allows handlers to invoke caps on the peer (host).
// Spawns a goroutine that receives response frames and forwards them to a channel.
// Returns a channel that yields bare CBOR Frame objects (STREAM_START, CHUNK,
// STREAM_END, END, ERR) as they arrive from the host. The consumer processes
// frames directly - no decoding, no wrapper types.
type PeerInvoker interface {
	Invoke(capUrn string, arguments []CapArgumentValue) (<-chan cbor.Frame, error)
}

// StreamChunk removed - handlers now receive bare CBOR Frame objects directly

// HandlerFunc is the function signature for cap handlers.
// Receives bare CBOR Frame objects for both input arguments and peer responses.
// Handler has full streaming control - decides when to consume frames and when to produce output.
type HandlerFunc func(frames <-chan cbor.Frame, emitter StreamEmitter, peer PeerInvoker) error

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
	rawWriter := cbor.NewFrameWriter(os.Stdout)

	// Perform handshake - send our manifest in the HELLO response
	// Handshake is single-threaded so raw writer is safe here
	negotiatedLimits, err := cbor.HandshakeAccept(reader, rawWriter, pr.manifestData)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	reader.SetLimits(negotiatedLimits)
	rawWriter.SetLimits(negotiatedLimits)

	// Wrap writer for thread-safe concurrent access from handler goroutines
	writer := newSyncFrameWriter(rawWriter)

	pr.mu.Lock()
	pr.limits = negotiatedLimits
	pr.mu.Unlock()

	// Track pending peer requests (plugin invoking host caps)
	// Key is MessageId.ToString() because MessageId contains []byte which is not comparable
	pendingPeerRequests := &sync.Map{} // map[string]*pendingPeerRequest

	// Track incoming requests that are being chunked
	// Protocol v2: Stream tracking for incoming request streams
	type pendingStream struct {
		mediaUrn string
		chunks   [][]byte
		complete bool
	}

	type streamEntry struct {
		streamID string
		stream   *pendingStream
	}

	type pendingIncomingRequest struct {
		capUrn  string
		handler HandlerFunc
		streams []streamEntry // Ordered list of streams
		ended   bool          // True after END frame - any stream activity after is FATAL
	}
	pendingIncoming := make(map[string]*pendingIncomingRequest)
	pendingIncomingMu := &sync.Mutex{}

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

			capUrn := *frame.Cap
			rawPayload := frame.Payload

			// Protocol v2: REQ must have empty payload - arguments come as streams
			if len(rawPayload) > 0 {
				errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "REQ frame must have empty payload - use STREAM_START for arguments")
				if err := writer.WriteFrame(errFrame); err != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write PROTOCOL_ERROR: %v\n", err)
				}
				continue
			}

			// Find handler
			handler := pr.FindHandler(capUrn)
			if handler == nil {
				errFrame := cbor.NewErr(frame.Id, "NO_HANDLER", fmt.Sprintf("No handler registered for cap: %s", capUrn))
				if writeErr := writer.WriteFrame(errFrame); writeErr != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", writeErr)
				}
				continue
			}

			// Start tracking this request - streams will be added via STREAM_START
			pendingIncomingMu.Lock()
			pendingIncoming[frame.Id.ToString()] = &pendingIncomingRequest{
				capUrn:  capUrn,
				handler: handler,
				streams: []streamEntry{}, // Streams added via STREAM_START
				ended:   false,
			}
			pendingIncomingMu.Unlock()
			fmt.Fprintf(os.Stderr, "[PluginRuntime] REQ: req_id=%s cap=%s - waiting for streams\n", frame.Id.ToString(), capUrn)
			continue // Wait for STREAM_START/CHUNK/STREAM_END/END frames

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

		case cbor.FrameTypeChunk:
			// Protocol v2: CHUNK must have stream_id
			if frame.StreamId == nil {
				errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "CHUNK frame missing stream_id")
				if err := writer.WriteFrame(errFrame); err != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
				}
				continue
			}

			streamID := *frame.StreamId

			// Check if this is a chunk for an incoming request
			pendingIncomingMu.Lock()
			if pendingReq, exists := pendingIncoming[frame.Id.ToString()]; exists {
				// FAIL HARD: Request already ended
				if pendingReq.ended {
					delete(pendingIncoming, frame.Id.ToString())
					pendingIncomingMu.Unlock()
					errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "CHUNK after request END")
					if err := writer.WriteFrame(errFrame); err != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
					}
					continue
				}

				// FAIL HARD: Unknown or inactive stream
				var foundStream *pendingStream
				for i := range pendingReq.streams {
					if pendingReq.streams[i].streamID == streamID {
						foundStream = pendingReq.streams[i].stream
						break
					}
				}

				if foundStream == nil {
					delete(pendingIncoming, frame.Id.ToString())
					pendingIncomingMu.Unlock()
					errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", fmt.Sprintf("CHUNK for unknown stream_id: %s", streamID))
					if err := writer.WriteFrame(errFrame); err != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
					}
					continue
				}

				if foundStream.complete {
					delete(pendingIncoming, frame.Id.ToString())
					pendingIncomingMu.Unlock()
					errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", fmt.Sprintf("CHUNK for ended stream: %s", streamID))
					if err := writer.WriteFrame(errFrame); err != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
					}
					continue
				}

				// ✅ Valid chunk for active stream
				if frame.Payload != nil {
					foundStream.chunks = append(foundStream.chunks, frame.Payload)
				}
				pendingIncomingMu.Unlock()
				continue // Wait for more chunks or STREAM_END
			}
			pendingIncomingMu.Unlock()

			// Not an incoming request chunk - must be a peer response chunk
			// Forward bare Frame object to handler - no wrapping, no decoding
			idKey := frame.Id.ToString()
			if pending, ok := pendingPeerRequests.Load(idKey); ok {
				pendingReq := pending.(*pendingPeerRequest)
				pendingReq.sender <- *frame
			}

		case cbor.FrameTypeEnd:
			// Protocol v2: END frame marks the end of all streams for this request
			pendingIncomingMu.Lock()
			pendingReq, exists := pendingIncoming[frame.Id.ToString()]
			if exists {
				pendingReq.ended = true
				delete(pendingIncoming, frame.Id.ToString())
			}
			pendingIncomingMu.Unlock()

			if exists {
				// Build frame channel with all incoming frames in order
				// Protocol v2: Send STREAM_START → CHUNK(s) → STREAM_END for each stream, then END
				requestID := frame.Id
				handler := pendingReq.handler
				capUrn := pendingReq.capUrn

				// Create buffered channel for input frames
				framesChan := make(chan cbor.Frame, 64)

				activeHandlers.Add(1)
				go func() {
					defer activeHandlers.Done()
					defer close(framesChan)

					// Generate unique stream ID for response
					streamID := fmt.Sprintf("resp-%s", requestID.ToString()[:8])
					mediaUrn := "media:bytes" // Default output media URN

					// Create emitter with stream multiplexing
					emitter := newThreadSafeEmitter(writer, requestID, streamID, mediaUrn, negotiatedLimits.MaxChunk)
					peerInvoker := newPeerInvokerImpl(writer, pendingPeerRequests, negotiatedLimits.MaxChunk)

					fmt.Fprintf(os.Stderr, "[PluginRuntime] END: Invoking handler for cap=%s with %d streams\n", capUrn, len(pendingReq.streams))

					// Send all frames to channel: STREAM_START → CHUNK(s) → STREAM_END per stream, then END
					go func() {
						for _, entry := range pendingReq.streams {
							// STREAM_START
							startFrame := cbor.NewStreamStart(requestID, entry.streamID, entry.stream.mediaUrn)
							framesChan <- *startFrame

							// CHUNKs
							for seq, chunk := range entry.stream.chunks {
								chunkFrame := cbor.NewChunk(requestID, entry.streamID, uint64(seq), chunk)
								framesChan <- *chunkFrame
							}

							// STREAM_END
							endStreamFrame := cbor.NewStreamEnd(requestID, entry.streamID)
							framesChan <- *endStreamFrame
						}

						// END frame
						framesChan <- *frame
					}()

					// Invoke handler with frame channel
					err := handler(framesChan, emitter, peerInvoker)
					if err != nil {
						errFrame := cbor.NewErr(requestID, "HANDLER_ERROR", err.Error())
						if writeErr := writer.WriteFrame(errFrame); writeErr != nil {
							fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", writeErr)
						}
						return
					}

					// Finalize sends STREAM_END + END frames
					emitter.Finalize()
				}()

				continue
			}

			// Not an incoming request end - must be a peer response end
			// Closing the channel signals completion to the handler
			idKey := frame.Id.ToString()
			if pending, ok := pendingPeerRequests.LoadAndDelete(idKey); ok {
				pendingReq := pending.(*pendingPeerRequest)
				close(pendingReq.sender)
			}

		// RES frame REMOVED - old protocol no longer supported
		// Peer invoke responses now use stream multiplexing (handled by END case above)

		case cbor.FrameTypeErr:
			// Error frame from host - could be response to peer request
			// Forward bare ERR frame to handler - handler extracts error details
			idKey := frame.Id.ToString()
			if pending, ok := pendingPeerRequests.LoadAndDelete(idKey); ok {
				pendingReq := pending.(*pendingPeerRequest)
				pendingReq.sender <- *frame
				close(pendingReq.sender)
			}

		case cbor.FrameTypeLog:
			// Log frames from host - shouldn't normally receive these, ignore
			continue

		case cbor.FrameTypeStreamStart:
			// Protocol v2: A new stream is starting for a request
			if frame.StreamId == nil {
				errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "STREAM_START missing stream_id")
				if err := writer.WriteFrame(errFrame); err != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
				}
				continue
			}

			if frame.MediaUrn == nil {
				errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "STREAM_START missing media_urn")
				if err := writer.WriteFrame(errFrame); err != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
				}
				continue
			}

			streamID := *frame.StreamId
			mediaUrn := *frame.MediaUrn

			fmt.Fprintf(os.Stderr, "[PluginRuntime] STREAM_START: req_id=%s stream_id=%s media_urn=%s\n",
				frame.Id.ToString(), streamID, mediaUrn)

			// STRICT: Add stream with validation
			pendingIncomingMu.Lock()
			if pendingReq, exists := pendingIncoming[frame.Id.ToString()]; exists {
				// FAIL HARD: Request already ended
				if pendingReq.ended {
					delete(pendingIncoming, frame.Id.ToString())
					pendingIncomingMu.Unlock()
					errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "STREAM_START after request END")
					if err := writer.WriteFrame(errFrame); err != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
					}
					continue
				}

				// FAIL HARD: Duplicate stream_id
				for _, entry := range pendingReq.streams {
					if entry.streamID == streamID {
						delete(pendingIncoming, frame.Id.ToString())
						pendingIncomingMu.Unlock()
						errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", fmt.Sprintf("Duplicate stream_id: %s", streamID))
						if err := writer.WriteFrame(errFrame); err != nil {
							fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
						}
						continue
					}
				}

				// ✅ Add new stream
				pendingReq.streams = append(pendingReq.streams, streamEntry{
					streamID: streamID,
					stream: &pendingStream{
						mediaUrn: mediaUrn,
						chunks:   [][]byte{},
						complete: false,
					},
				})
				pendingIncomingMu.Unlock()
				fmt.Fprintf(os.Stderr, "[PluginRuntime] Incoming stream started: %s\n", streamID)
				continue
			}
			pendingIncomingMu.Unlock()

			// Not an incoming request — check if it's a peer response stream
			idKey := frame.Id.ToString()
			if pending, ok := pendingPeerRequests.Load(idKey); ok {
				pendingReq := pending.(*pendingPeerRequest)
				pendingReq.streams[streamID] = mediaUrn
				// Forward bare STREAM_START frame to handler
				pendingReq.sender <- *frame
			} else {
				fmt.Fprintf(os.Stderr, "[PluginRuntime] STREAM_START for unknown request_id: %s\n", frame.Id.ToString())
			}

		case cbor.FrameTypeStreamEnd:
			// Protocol v2: A stream has ended for a request
			if frame.StreamId == nil {
				errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", "STREAM_END missing stream_id")
				if err := writer.WriteFrame(errFrame); err != nil {
					fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
				}
				continue
			}

			streamID := *frame.StreamId
			fmt.Fprintf(os.Stderr, "[PluginRuntime] STREAM_END: stream_id=%s\n", streamID)

			// STRICT: Mark stream as complete with validation
			pendingIncomingMu.Lock()
			if pendingReq, exists := pendingIncoming[frame.Id.ToString()]; exists {
				// Find and mark stream as complete
				found := false
				for i := range pendingReq.streams {
					if pendingReq.streams[i].streamID == streamID {
						pendingReq.streams[i].stream.complete = true
						found = true
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Incoming stream marked complete: %s\n", streamID)
						break
					}
				}

				if !found {
					// FAIL HARD: STREAM_END for unknown stream
					delete(pendingIncoming, frame.Id.ToString())
					pendingIncomingMu.Unlock()
					errFrame := cbor.NewErr(frame.Id, "PROTOCOL_ERROR", fmt.Sprintf("STREAM_END for unknown stream_id: %s", streamID))
					if err := writer.WriteFrame(errFrame); err != nil {
						fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write error: %v\n", err)
					}
					continue
				}
				pendingIncomingMu.Unlock()
				continue
			}
			pendingIncomingMu.Unlock()

			// Not an incoming request stream — check if it's a peer response stream end
			idKey := frame.Id.ToString()
			if pending, ok := pendingPeerRequests.Load(idKey); ok {
				pendingReq := pending.(*pendingPeerRequest)
				// Forward bare STREAM_END frame to handler
				pendingReq.sender <- *frame
			} else {
				fmt.Fprintf(os.Stderr, "[PluginRuntime] STREAM_END for unknown request_id: %s\n", frame.Id.ToString())
			}

		case cbor.FrameTypeRelayNotify, cbor.FrameTypeRelayState:
			// Relay-level frames must never reach a plugin runtime.
			// If they do, it's a bug in the relay layer — fail hard.
			return fmt.Errorf("relay frame %v must not reach plugin runtime", frame.FrameType)
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

	// Build CBOR payload from CLI args
	rawPayload, err := pr.buildPayloadFromCLI(cap, args[2:])
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	// Create CLI-mode frame channel
	// CLI mode: single argument stream with STREAM_START → CHUNK → STREAM_END → END
	framesChan := make(chan cbor.Frame, 8)
	requestID := cbor.NewMessageIdDefault()
	streamID := "cli-arg"
	mediaUrn := "media:bytes"

	// Send frames in a goroutine
	go func() {
		defer close(framesChan)

		// STREAM_START
		startFrame := cbor.NewStreamStart(requestID, streamID, mediaUrn)
		framesChan <- *startFrame

		// CHUNK (single chunk for CLI)
		if len(rawPayload) > 0 {
			chunkFrame := cbor.NewChunk(requestID, streamID, 0, rawPayload)
			framesChan <- *chunkFrame
		}

		// STREAM_END
		endStreamFrame := cbor.NewStreamEnd(requestID, streamID)
		framesChan <- *endStreamFrame

		// END
		endFrame := cbor.NewEnd(requestID, nil)
		framesChan <- *endFrame
	}()

	// Create CLI-mode emitter and no-op peer invoker
	emitter := &cliStreamEmitter{}
	peer := &noPeerInvoker{}

	// Invoke handler with frame channel
	err = handler(framesChan, emitter, peer)
	if err != nil {
		errorJSON, _ := json.Marshal(map[string]string{
			"error": err.Error(),
			"code":  "HANDLER_ERROR",
		})
		fmt.Fprintln(os.Stderr, string(errorJSON))
		return err
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

// extractEffectivePayload extracts the effective payload from a REQ frame.
// When content_type is "application/cbor", decodes the CBOR arguments
// and finds the argument whose media_urn semantically matches the cap's input spec.
func extractEffectivePayload(payload []byte, contentType string, capUrn string) ([]byte, error) {
	// Not CBOR arguments - return raw payload
	if contentType != "application/cbor" {
		return payload, nil
	}

	// Empty payload with CBOR content type — no arguments
	if len(payload) == 0 {
		return payload, nil
	}

	// Parse the cap URN to get the expected input media URN
	capUrnParsed, err := NewCapUrnFromString(capUrn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cap URN '%s': %w", capUrn, err)
	}
	expectedInSpec := capUrnParsed.InSpec()

	// Parse expected input as a TaggedUrn for semantic matching
	var expectedUrn *taggedurn.TaggedUrn
	if expectedInSpec != "*" {
		expectedUrn, err = taggedurn.NewTaggedUrnFromString(expectedInSpec)
		if err != nil {
			return nil, fmt.Errorf("failed to parse expected in_spec '%s': %w", expectedInSpec, err)
		}
	}

	// Decode CBOR payload as array of argument maps
	var args []map[string]interface{}
	if err := cborlib.Unmarshal(payload, &args); err != nil {
		// Not a valid CBOR arguments array - fall back to raw payload
		return payload, nil
	}

	// Search for the argument matching the expected input media URN
	for _, arg := range args {
		mediaUrnStr, ok := arg["media_urn"].(string)
		if !ok {
			continue
		}
		value, hasValue := arg["value"]
		if !hasValue {
			continue
		}

		// If wildcard input, take the first argument
		if expectedUrn == nil {
			return toBytes(value), nil
		}

		// Semantic match: try both directions of conforms_to
		argUrn, parseErr := taggedurn.NewTaggedUrnFromString(mediaUrnStr)
		if parseErr != nil {
			continue
		}

		fwd, _ := argUrn.ConformsTo(expectedUrn)
		rev, _ := expectedUrn.ConformsTo(argUrn)
		if fwd || rev {
			return toBytes(value), nil
		}
	}

	// No matching argument found - if there's exactly one argument, use it
	if len(args) == 1 {
		if value, ok := args[0]["value"]; ok {
			return toBytes(value), nil
		}
	}

	return nil, fmt.Errorf("no argument matching in_spec '%s' found in CBOR arguments", expectedInSpec)
}

// toBytes converts a CBOR-decoded value to []byte
func toBytes(v interface{}) []byte {
	switch val := v.(type) {
	case []byte:
		return val
	case string:
		return []byte(val)
	default:
		// Try JSON encoding as fallback
		data, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return data
	}
}

// syncFrameWriter wraps FrameWriter with a mutex for concurrent access.
// FrameWriter.WriteFrame does two Write() calls (length prefix + CBOR data)
// which interleave when called from multiple goroutines.
type syncFrameWriter struct {
	mu     sync.Mutex
	writer *cbor.FrameWriter
}

func newSyncFrameWriter(w *cbor.FrameWriter) *syncFrameWriter {
	return &syncFrameWriter{writer: w}
}

func (s *syncFrameWriter) WriteFrame(frame *cbor.Frame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.WriteFrame(frame)
}

func (s *syncFrameWriter) SetLimits(limits cbor.Limits) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer.SetLimits(limits)
}

// threadSafeEmitter implements StreamEmitter with thread-safe writes using stream multiplexing
type threadSafeEmitter struct {
	writer        *syncFrameWriter
	requestID     cbor.MessageId
	streamID      string // Response stream ID
	mediaUrn      string // Response media URN
	streamStarted bool   // Track if STREAM_START was sent
	seq           uint64
	seqMu         sync.Mutex
	maxChunk      int
}

func newThreadSafeEmitter(writer *syncFrameWriter, requestID cbor.MessageId, streamID string, mediaUrn string, maxChunk int) *threadSafeEmitter {
	return &threadSafeEmitter{
		writer:        writer,
		requestID:     requestID,
		streamID:      streamID,
		mediaUrn:      mediaUrn,
		streamStarted: false,
		maxChunk:      maxChunk,
	}
}

func (e *threadSafeEmitter) EmitCbor(value interface{}) error {
	e.seqMu.Lock()
	defer e.seqMu.Unlock()

	// CBOR-encode the value once - handler produces CBOR values, not raw bytes
	cborPayload, err := cborlib.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to CBOR-encode value: %w", err)
	}

	// STREAM MULTIPLEXING: Send STREAM_START before first chunk
	if !e.streamStarted {
		e.streamStarted = true
		startFrame := cbor.NewStreamStart(e.requestID, e.streamID, e.mediaUrn)
		if err := e.writer.WriteFrame(startFrame); err != nil {
			return fmt.Errorf("failed to write STREAM_START: %w", err)
		}
	}

	// AUTO-CHUNKING: Split large CBOR payloads into negotiated max_chunk sized pieces
	offset := 0
	for offset < len(cborPayload) {
		chunkSize := len(cborPayload) - offset
		if chunkSize > e.maxChunk {
			chunkSize = e.maxChunk
		}
		chunkData := cborPayload[offset : offset+chunkSize]

		currentSeq := e.seq
		e.seq++

		frame := cbor.NewChunk(e.requestID, e.streamID, currentSeq, chunkData)
		if err := e.writer.WriteFrame(frame); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
		offset += chunkSize
	}
	return nil
}

// Finalize sends STREAM_END + END frames to complete the response
func (e *threadSafeEmitter) Finalize() {
	e.seqMu.Lock()
	defer e.seqMu.Unlock()

	// If no chunks were sent, still send STREAM_START to keep protocol consistent
	if !e.streamStarted {
		e.streamStarted = true
		startFrame := cbor.NewStreamStart(e.requestID, e.streamID, e.mediaUrn)
		if err := e.writer.WriteFrame(startFrame); err != nil {
			fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write STREAM_START: %v\n", err)
			return
		}
	}

	// STREAM_END: Close this stream
	streamEndFrame := cbor.NewStreamEnd(e.requestID, e.streamID)
	if err := e.writer.WriteFrame(streamEndFrame); err != nil {
		fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write STREAM_END: %v\n", err)
		return
	}

	// END: Close the entire request
	endFrame := cbor.NewEnd(e.requestID, nil)
	if err := e.writer.WriteFrame(endFrame); err != nil {
		fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write END: %v\n", err)
	}
}

func (e *threadSafeEmitter) EmitLog(level, message string) {
	frame := cbor.NewLog(e.requestID, level, message)
	if err := e.writer.WriteFrame(frame); err != nil {
		fmt.Fprintf(os.Stderr, "[PluginRuntime] Failed to write log: %v\n", err)
	}
}

// cliStreamEmitter implements StreamEmitter for CLI mode
type cliStreamEmitter struct{}

func (e *cliStreamEmitter) EmitCbor(value interface{}) error {
	// CBOR-encode the value once
	cborPayload, err := cborlib.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to CBOR-encode value: %w", err)
	}
	os.Stdout.Write(cborPayload)
	os.Stdout.Write([]byte("\n"))
	return nil
}

func (e *cliStreamEmitter) EmitLog(level, message string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", level, message)
}

// pendingPeerRequest tracks a pending peer request.
// The reader loop forwards response frames to the channel.
type pendingPeerRequest struct {
	sender  chan cbor.Frame    // Channel to send response frames to handler
	streams map[string]string  // stream_id → media_urn mapping
	ended   bool               // true after END frame (close channel)
}

// peerInvokerImpl implements PeerInvoker
type peerInvokerImpl struct {
	writer          *syncFrameWriter
	pendingRequests *sync.Map
	maxChunk        int
}

func newPeerInvokerImpl(writer *syncFrameWriter, pendingRequests *sync.Map, maxChunk int) *peerInvokerImpl {
	return &peerInvokerImpl{
		writer:          writer,
		pendingRequests: pendingRequests,
		maxChunk:        maxChunk,
	}
}

func (p *peerInvokerImpl) Invoke(capUrn string, arguments []CapArgumentValue) (<-chan cbor.Frame, error) {
	// Generate a new message ID for this request
	requestID := cbor.NewMessageIdRandom()

	// Create a buffered channel for response frames
	sender := make(chan cbor.Frame, 64)

	// Register the pending request before sending
	p.pendingRequests.Store(requestID.ToString(), &pendingPeerRequest{
		sender:  sender,
		streams: make(map[string]string),
		ended:   false,
	})

	maxChunk := p.maxChunk

	// Protocol v2: REQ(empty) + STREAM_START + CHUNK(s) + STREAM_END + END per argument

	// 1. REQ with empty payload
	reqFrame := cbor.NewReq(requestID, capUrn, nil, "application/cbor")
	if err := p.writer.WriteFrame(reqFrame); err != nil {
		p.pendingRequests.Delete(requestID.ToString())
		return nil, fmt.Errorf("failed to send REQ frame: %w", err)
	}

	// 2. Each argument as an independent stream
	for _, arg := range arguments {
		streamID := fmt.Sprintf("peer-%s", cbor.NewMessageIdRandom().ToString()[:8])

		// STREAM_START
		startFrame := cbor.NewStreamStart(requestID, streamID, arg.MediaUrn)
		if err := p.writer.WriteFrame(startFrame); err != nil {
			p.pendingRequests.Delete(requestID.ToString())
			return nil, fmt.Errorf("failed to send STREAM_START: %w", err)
		}

		// CHUNK(s)
		offset := 0
		seq := uint64(0)
		for offset < len(arg.Value) {
			chunkSize := len(arg.Value) - offset
			if chunkSize > maxChunk {
				chunkSize = maxChunk
			}
			chunkData := arg.Value[offset : offset+chunkSize]
			chunkFrame := cbor.NewChunk(requestID, streamID, seq, chunkData)
			if err := p.writer.WriteFrame(chunkFrame); err != nil {
				p.pendingRequests.Delete(requestID.ToString())
				return nil, fmt.Errorf("failed to send CHUNK: %w", err)
			}
			offset += chunkSize
			seq++
		}

		// STREAM_END
		endFrame := cbor.NewStreamEnd(requestID, streamID)
		if err := p.writer.WriteFrame(endFrame); err != nil {
			p.pendingRequests.Delete(requestID.ToString())
			return nil, fmt.Errorf("failed to send STREAM_END: %w", err)
		}
	}

	// 3. END
	endFrame := cbor.NewEnd(requestID, nil)
	if err := p.writer.WriteFrame(endFrame); err != nil {
		p.pendingRequests.Delete(requestID.ToString())
		return nil, fmt.Errorf("failed to send END: %w", err)
	}

	return sender, nil
}

// noPeerInvoker is a no-op PeerInvoker that always returns an error
type noPeerInvoker struct{}

func (n *noPeerInvoker) Invoke(capUrn string, arguments []CapArgumentValue) (<-chan cbor.Frame, error) {
	return nil, errors.New("peer invocation not supported in this context")
}

// Limits returns the current protocol limits
func (pr *PluginRuntime) Limits() cbor.Limits {
	pr.mu.RLock()
	defer pr.mu.RUnlock()
	return pr.limits
}

// buildPayloadFromStreamingReader builds CBOR payload from streaming reader (testable version).
//
// This simulates the CBOR chunked request flow for CLI piped stdin:
// - Pure binary chunks from reader
// - Accumulated in chunks (respecting max_chunk size)
// - Built into CBOR arguments array (same format as CBOR mode)
//
// This makes all 4 modes use the SAME payload format:
// - CLI file path → read file → payload
// - CLI piped binary → chunk reader → payload
// - CBOR chunked → payload
// - CBOR file path → auto-convert → payload
func (pr *PluginRuntime) buildPayloadFromStreamingReader(cap *Cap, reader io.Reader, maxChunk int) ([]byte, error) {
	// Accumulate chunks
	var chunks [][]byte
	totalBytes := 0

	for {
		buffer := make([]byte, maxChunk)
		n, err := reader.Read(buffer)
		if n > 0 {
			buffer = buffer[:n]
			chunks = append(chunks, buffer)
			totalBytes += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	// Concatenate chunks
	completePayload := make([]byte, 0, totalBytes)
	for _, chunk := range chunks {
		completePayload = append(completePayload, chunk...)
	}

	// Build CBOR arguments array (same format as CBOR mode)
	capUrn, err := NewCapUrnFromString(cap.UrnString())
	if err != nil {
		return nil, fmt.Errorf("invalid cap URN: %w", err)
	}
	expectedMediaUrn := capUrn.InSpec()

	arg := CapArgumentValue{
		MediaUrn: expectedMediaUrn,
		Value:    completePayload,
	}

	// Encode as CBOR array
	cborArgs := []interface{}{
		map[string]interface{}{
			"media_urn": arg.MediaUrn,
			"value":     arg.Value,
		},
	}

	payload, err := cborlib.Marshal(cborArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to encode CBOR payload: %w", err)
	}

	return payload, nil
}

// buildPayloadFromCLI builds CBOR payload from CLI arguments based on cap's arg definitions.
// Returns CBOR-encoded array of CapArgumentValue objects.
func (pr *PluginRuntime) buildPayloadFromCLI(cap *Cap, cliArgs []string) ([]byte, error) {
	// Read stdin if available (non-blocking check)
	stdinData, err := pr.readStdinIfAvailable()
	if err != nil {
		return nil, fmt.Errorf("failed to read stdin: %w", err)
	}

	// If no args defined, check for stdin data
	if len(cap.Args) == 0 {
		if stdinData != nil {
			return stdinData, nil
		}
		// No args and no stdin - return empty payload
		return []byte{}, nil
	}

	// Build CBOR arguments array (same format as CBOR mode)
	var arguments []CapArgumentValue

	for i := range cap.Args {
		argDef := &cap.Args[i]

		// Extract argument value (handles file-path conversion)
		value, err := pr.extractArgValue(argDef, cliArgs, stdinData)
		if err != nil {
			return nil, err
		}

		if value != nil {
			// Determine media URN for this argument value
			// If file-path with stdin source, use stdin source's media URN (the target type)
			mediaUrn := argDef.MediaUrn

			// Check if this is a file-path arg with stdin source
			argMediaUrn, parseErr := NewMediaUrnFromString(argDef.MediaUrn)
			if parseErr == nil {
				filePathPattern, _ := NewMediaUrnFromString(MediaFilePath)
				filePathArrayPattern, _ := NewMediaUrnFromString(MediaFilePathArray)

				isFilePath := false
				if filePathPattern != nil {
					if filePathPattern.Accepts(argMediaUrn) {
						isFilePath = true
					}
				}
				if !isFilePath && filePathArrayPattern != nil {
					if filePathArrayPattern.Accepts(argMediaUrn) {
						isFilePath = true
					}
				}

				// If file-path type, check for stdin source and use its media URN
				if isFilePath {
					for i := range argDef.Sources {
						source := &argDef.Sources[i]
						if source.Stdin != nil {
							mediaUrn = *source.Stdin
							break
						}
					}
				}
			}

			arguments = append(arguments, CapArgumentValue{
				MediaUrn: mediaUrn,
				Value:    value,
			})
		} else if argDef.Required {
			return nil, fmt.Errorf("required argument missing: %s", argDef.MediaUrn)
		}
	}

	if len(arguments) > 0 {
		// Encode as CBOR array
		cborArgs := make([]interface{}, len(arguments))
		for i, arg := range arguments {
			cborArgs[i] = map[string]interface{}{
				"media_urn": arg.MediaUrn,
				"value":     arg.Value,
			}
		}

		payload, err := cborlib.Marshal(cborArgs)
		if err != nil {
			return nil, fmt.Errorf("failed to encode CBOR payload: %w", err)
		}
		return payload, nil
	}

	// No arguments and no stdin
	return []byte{}, nil
}

// extractArgValue extracts a single argument value from CLI args or stdin.
// Handles automatic file-path to bytes conversion when appropriate.
func (pr *PluginRuntime) extractArgValue(argDef *CapArg, cliArgs []string, stdinData []byte) ([]byte, error) {
	// Check if this arg requires file-path to bytes conversion using proper URN matching
	argMediaUrn, err := NewMediaUrnFromString(argDef.MediaUrn)
	if err != nil {
		return nil, fmt.Errorf("invalid media URN '%s': %w", argDef.MediaUrn, err)
	}

	filePathPattern, err := NewMediaUrnFromString(MediaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MediaFilePath constant: %w", err)
	}
	filePathArrayPattern, err := NewMediaUrnFromString(MediaFilePathArray)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MediaFilePathArray constant: %w", err)
	}

	// Check array first (more specific), then single file-path
	isArray := false
	isFilePath := false

	if filePathArrayPattern.Accepts(argMediaUrn) {
		isArray = true
		isFilePath = true
	} else if filePathPattern.Accepts(argMediaUrn) {
		isFilePath = true
	}

	// Get stdin source media URN if it exists (tells us target type)
	hasStdinSource := false
	for i := range argDef.Sources {
		if argDef.Sources[i].Stdin != nil {
			hasStdinSource = true
			break
		}
	}

	// Try each source in order
	for i := range argDef.Sources {
		source := &argDef.Sources[i]

		if source.CliFlag != nil {
			if value, found := pr.getCliFlagValue(cliArgs, *source.CliFlag); found {
				// If file-path type with stdin source, read file(s)
				if isFilePath && hasStdinSource {
					return pr.readFilePathToBytes(value, isArray)
				}
				return []byte(value), nil
			}
		} else if source.Position != nil {
			// Positional args: filter out flags and their values
			positional := pr.getPositionalArgs(cliArgs)
			if *source.Position < len(positional) {
				value := positional[*source.Position]
				// If file-path type with stdin source, read file(s)
				if isFilePath && hasStdinSource {
					return pr.readFilePathToBytes(value, isArray)
				}
				return []byte(value), nil
			}
		} else if source.Stdin != nil {
			if stdinData != nil && len(stdinData) > 0 {
				return stdinData, nil
			}
		}
	}

	// Try default value
	if argDef.DefaultValue != nil {
		bytes, err := json.Marshal(argDef.DefaultValue)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize default value: %w", err)
		}
		return bytes, nil
	}

	return nil, nil
}

// getCliFlagValue gets the value for a CLI flag (e.g., --model "value")
func (pr *PluginRuntime) getCliFlagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", false
		}
		// Handle --flag=value format
		if len(args[i]) > len(flag) && args[i][:len(flag)] == flag && args[i][len(flag)] == '=' {
			return args[i][len(flag)+1:], true
		}
	}
	return "", false
}

// getPositionalArgs gets positional arguments (non-flag arguments)
func (pr *PluginRuntime) getPositionalArgs(args []string) []string {
	var positional []string
	skipNext := false

	for _, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			// This is a flag - skip its value too if not --flag=value format
			if !contains(arg, '=') {
				skipNext = true
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return positional
}

// contains checks if a string contains a character
func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

// readStdinIfAvailable reads stdin if data is available (non-blocking check)
func (pr *PluginRuntime) readStdinIfAvailable() ([]byte, error) {
	// Check if stdin is a terminal (interactive)
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	// Don't read from stdin if it's a terminal (interactive)
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, nil
	}

	// Read all data from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	return data, nil
}

// readFilePathToBytes reads file(s) for file-path arguments and returns bytes.
//
// This method implements automatic file-path to bytes conversion when:
// - arg.media_urn is "media:file-path" or "media:file-path-array"
// - arg has a stdin source (indicating bytes are the canonical type)
//
// # Arguments
// * pathValue - File path string (single path or JSON array of path patterns)
// * isArray - True if media:file-path-array (read multiple files with glob expansion)
//
// # Returns
// - For single file: []byte containing raw file bytes
// - For array: CBOR-encoded array of file bytes (each element is one file's contents)
//
// # Errors
// Returns error if file cannot be read with clear error message.
func (pr *PluginRuntime) readFilePathToBytes(pathValue string, isArray bool) ([]byte, error) {
	if isArray {
		// Parse JSON array of path patterns
		var pathPatterns []string
		if err := json.Unmarshal([]byte(pathValue), &pathPatterns); err != nil {
			return nil, fmt.Errorf(
				"failed to parse file-path-array: expected JSON array of path patterns, got '%s': %w",
				pathValue, err,
			)
		}

		// Expand globs and collect all file paths
		var allFiles []string
		for _, pattern := range pathPatterns {
			// Check if this is a literal path (no glob metacharacters) or a glob pattern
			isGlob := containsAny(pattern, "*?[")

			if !isGlob {
				// Literal path - verify it exists and is a file
				info, err := os.Stat(pattern)
				if err != nil {
					if os.IsNotExist(err) {
						return nil, fmt.Errorf(
							"failed to read file '%s' from file-path-array: No such file or directory",
							pattern,
						)
					}
					return nil, fmt.Errorf(
						"failed to read file '%s' from file-path-array: %w",
						pattern, err,
					)
				}
				if info.Mode().IsRegular() {
					allFiles = append(allFiles, pattern)
				}
				// Skip directories silently for consistency with glob behavior
			} else {
				// Glob pattern - expand it
				matches, err := filepath.Glob(pattern)
				if err != nil {
					return nil, fmt.Errorf("invalid glob pattern '%s': %w", pattern, err)
				}

				for _, path := range matches {
					info, err := os.Stat(path)
					if err != nil {
						continue
					}
					// Only include files (skip directories)
					if info.Mode().IsRegular() {
						allFiles = append(allFiles, path)
					}
				}
			}
		}

		// Read each file sequentially
		var filesData []interface{}
		for _, path := range allFiles {
			bytes, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf(
					"failed to read file '%s' from file-path-array: %w",
					path, err,
				)
			}
			filesData = append(filesData, bytes)
		}

		// Encode as CBOR array
		cborBytes, err := cborlib.Marshal(filesData)
		if err != nil {
			return nil, fmt.Errorf("failed to encode CBOR array: %w", err)
		}

		return cborBytes, nil
	} else {
		// Single file path - read and return raw bytes
		bytes, err := os.ReadFile(pathValue)
		if err != nil {
			return nil, fmt.Errorf("failed to read file '%s': %w", pathValue, err)
		}
		return bytes, nil
	}
}

// containsAny checks if string contains any of the given characters
func containsAny(s string, chars string) bool {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return true
			}
		}
	}
	return false
}
