package capns

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/filegrind/capns-go/cbor"
)

// CapHandler is a function that handles a peer invoke request.
// It receives the concatenated payload bytes and returns response bytes.
type CapHandler func(payload []byte) ([]byte, error)

// peerIncomingStream tracks a single stream within an incoming peer request
type peerIncomingStream struct {
	mediaUrn string
	chunks   [][]byte
	complete bool
}

// peerIncomingRequest tracks an incoming peer request from a plugin (peer invoke)
type peerIncomingRequest struct {
	capUrn      string
	contentType string
	streams     []struct {
		streamID string
		stream   *peerIncomingStream
	}
	ended bool
}

// PluginHost manages communication with a plugin process
type PluginHost struct {
	reader          *cbor.FrameReader
	writer          *cbor.FrameWriter
	limits          cbor.Limits
	manifest        []byte
	pendingRequests map[string]*pendingRequest // key is MessageId.ToString()
	capHandlers     map[string]CapHandler      // op → handler for peer invoke
	peerIncoming    map[string]*peerIncomingRequest
	mu              sync.Mutex
	writerMu        sync.Mutex
	closed          bool
}

// streamState tracks a single stream within a request
type streamState struct {
	mediaUrn string
	active   bool // false after StreamEnd
}

// hostStreamEntry maintains ordered stream tracking (insertion order preserved)
type hostStreamEntry struct {
	streamID string
	state    *streamState
}

// pendingRequest tracks a pending request with stream multiplexing
type pendingRequest struct {
	chunks    []*ResponseChunk
	done      chan error
	isChunked bool
	streams   []hostStreamEntry // Ordered — maintains insertion order per Protocol v2
	ended     bool              // true after END frame - any stream activity after is FATAL
}

// ResponseChunk represents a response chunk from a plugin (matches Rust ResponseChunk)
type ResponseChunk struct {
	// The binary payload
	Payload []byte
	// Sequence number
	Seq uint64
	// Offset in the stream (for chunked transfers)
	Offset *uint64
	// Total length (set on first chunk of chunked transfer)
	Len *uint64
	// Whether this is the final chunk
	IsEof bool
}

// PluginResponseType indicates whether a response is single or streaming
type PluginResponseType int

const (
	// PluginResponseTypeSingle represents a single complete response
	PluginResponseTypeSingle PluginResponseType = iota
	// PluginResponseTypeStreaming represents a streaming response with chunks
	PluginResponseTypeStreaming
)

// PluginResponse represents a complete response from a plugin (matches Rust PluginResponse enum)
// Can be either Single or Streaming
type PluginResponse struct {
	Type PluginResponseType
	// Single response data (used when Type == PluginResponseTypeSingle)
	Single []byte
	// Streaming chunks (used when Type == PluginResponseTypeStreaming)
	Streaming []*ResponseChunk
}

// FinalPayload gets the final payload (single response or last chunk of streaming)
// Matches Rust PluginResponse::final_payload()
func (pr *PluginResponse) FinalPayload() []byte {
	switch pr.Type {
	case PluginResponseTypeSingle:
		return pr.Single
	case PluginResponseTypeStreaming:
		if len(pr.Streaming) > 0 {
			return pr.Streaming[len(pr.Streaming)-1].Payload
		}
		return nil
	default:
		return nil
	}
}

// Concatenated concatenates all payloads into a single buffer
// Matches Rust PluginResponse::concatenated()
func (pr *PluginResponse) Concatenated() []byte {
	switch pr.Type {
	case PluginResponseTypeSingle:
		// Clone the data
		result := make([]byte, len(pr.Single))
		copy(result, pr.Single)
		return result
	case PluginResponseTypeStreaming:
		// Pre-calculate total length
		totalLen := 0
		for _, chunk := range pr.Streaming {
			totalLen += len(chunk.Payload)
		}
		// Pre-allocate result buffer
		result := make([]byte, 0, totalLen)
		for _, chunk := range pr.Streaming {
			result = append(result, chunk.Payload...)
		}
		return result
	default:
		return nil
	}
}

// HostError represents errors from the plugin host (matches Rust AsyncHostError)
type HostError struct {
	Type    HostErrorType
	Message string
	Code    string // For PluginError type
}

// HostErrorType represents the type of host error
type HostErrorType int

const (
	HostErrorTypeCbor HostErrorType = iota
	HostErrorTypeIo
	HostErrorTypePluginError
	HostErrorTypeUnexpectedFrameType
	HostErrorTypeProcessExited
	HostErrorTypeHandshake
	HostErrorTypeClosed
	HostErrorTypeSendError
	HostErrorTypeRecvError
)

func (e *HostError) Error() string {
	switch e.Type {
	case HostErrorTypeCbor:
		return fmt.Sprintf("CBOR error: %s", e.Message)
	case HostErrorTypeIo:
		return fmt.Sprintf("I/O error: %s", e.Message)
	case HostErrorTypePluginError:
		return fmt.Sprintf("Plugin returned error: [%s] %s", e.Code, e.Message)
	case HostErrorTypeUnexpectedFrameType:
		return fmt.Sprintf("Unexpected frame type: %s", e.Message)
	case HostErrorTypeProcessExited:
		return "Plugin process exited unexpectedly"
	case HostErrorTypeHandshake:
		return fmt.Sprintf("Handshake failed: %s", e.Message)
	case HostErrorTypeClosed:
		return "Host is closed"
	case HostErrorTypeSendError:
		return "Send error: channel closed"
	case HostErrorTypeRecvError:
		return "Receive error: channel closed"
	default:
		return fmt.Sprintf("Unknown error: %s", e.Message)
	}
}

// NewPluginHost creates a new plugin host and performs handshake
func NewPluginHost(stdin io.Writer, stdout io.Reader) (*PluginHost, error) {
	reader := cbor.NewFrameReader(stdout)
	writer := cbor.NewFrameWriter(stdin)

	// Perform handshake
	manifest, limits, err := cbor.HandshakeInitiate(reader, writer)
	if err != nil {
		return nil, fmt.Errorf("handshake failed: %w", err)
	}

	reader.SetLimits(limits)
	writer.SetLimits(limits)

	host := &PluginHost{
		reader:          reader,
		writer:          writer,
		limits:          limits,
		manifest:        manifest,
		pendingRequests: make(map[string]*pendingRequest),
		capHandlers:     make(map[string]CapHandler),
		peerIncoming:    make(map[string]*peerIncomingRequest),
		closed:          false,
	}

	// Start background reader
	go host.readerLoop()

	return host, nil
}

// readerLoop reads frames from the plugin in the background
func (ph *PluginHost) readerLoop() {
	for {
		frame, err := ph.reader.ReadFrame()
		if err != nil {
			if err == io.EOF {
				ph.mu.Lock()
				ph.closed = true
				// Complete all pending requests with error
				for _, req := range ph.pendingRequests {
					req.done <- fmt.Errorf("plugin exited")
				}
				ph.mu.Unlock()
				return
			}
			fmt.Printf("[PluginHost] reader error: %v\n", err)
			continue
		}

		ph.handleFrame(frame)
	}
}

// handleFrame processes an incoming frame using stream multiplexing protocol
func (ph *PluginHost) handleFrame(frame *cbor.Frame) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	idKey := frame.Id.ToString()

	switch frame.FrameType {
	case cbor.FrameTypeReq:
		// Incoming REQ from plugin (peer invoke) — only if not a pending host request
		if _, isHostReq := ph.pendingRequests[idKey]; !isHostReq {
			ph.handlePeerInvoke(frame)
			return
		}

	case cbor.FrameTypeStreamStart:
		// Check if this belongs to a peer incoming request first
		if _, isPeer := ph.peerIncoming[idKey]; isPeer {
			ph.handlePeerFrame(frame)
			return
		}
		// STREAM_START: Announce new stream (host-initiated request response)
		if req, ok := ph.pendingRequests[idKey]; ok {
			// STRICT validation: must have stream_id and media_urn
			if frame.StreamId == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: StreamStart missing stream_id")
				return
			}
			if frame.MediaUrn == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: StreamStart missing media_urn")
				return
			}

			streamId := *frame.StreamId
			mediaUrn := *frame.MediaUrn

			// FAIL HARD: Request already ended
			if req.ended {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: stream activity after request END")
				return
			}

			// FAIL HARD: Duplicate stream ID
			for _, entry := range req.streams {
				if entry.streamID == streamId {
					delete(ph.pendingRequests, idKey)
					req.done <- fmt.Errorf("protocol violation: duplicate stream ID '%s'", streamId)
					return
				}
			}

			// Track new stream (ordered append)
			req.streams = append(req.streams, hostStreamEntry{
				streamID: streamId,
				state: &streamState{
					mediaUrn: mediaUrn,
					active:   true,
				},
			})
		}

	case cbor.FrameTypeChunk:
		// Check if this belongs to a peer incoming request first
		if _, isPeer := ph.peerIncoming[idKey]; isPeer {
			ph.handlePeerFrame(frame)
			return
		}
		// CHUNK: Data chunk for a stream (host-initiated request response)
		if req, ok := ph.pendingRequests[idKey]; ok {
			req.isChunked = true

			// STRICT validation: must have stream_id
			if frame.StreamId == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: Chunk missing stream_id")
				return
			}

			streamId := *frame.StreamId

			// FAIL HARD: Request already ended
			if req.ended {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: chunk after request END")
				return
			}

			// FAIL HARD: Unknown or inactive stream
			var chunkStream *streamState
			for _, entry := range req.streams {
				if entry.streamID == streamId {
					chunkStream = entry.state
					break
				}
			}
			if chunkStream == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: chunk for unknown stream ID '%s'", streamId)
				return
			}
			if !chunkStream.active {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: chunk for ended stream ID '%s'", streamId)
				return
			}

			// Valid chunk for active stream
			chunk := &ResponseChunk{
				Payload: frame.Payload,
				Seq:     frame.Seq,
				Offset:  frame.Offset,
				Len:     frame.Len,
				IsEof:   frame.IsEof(),
			}
			req.chunks = append(req.chunks, chunk)
		}

	case cbor.FrameTypeStreamEnd:
		// Check if this belongs to a peer incoming request first
		if _, isPeer := ph.peerIncoming[idKey]; isPeer {
			ph.handlePeerFrame(frame)
			return
		}
		// STREAM_END: End a specific stream (host-initiated request response)
		if req, ok := ph.pendingRequests[idKey]; ok {
			// STRICT validation: must have stream_id
			if frame.StreamId == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: StreamEnd missing stream_id")
				return
			}

			streamId := *frame.StreamId

			// FAIL HARD: Unknown stream
			var endStream *streamState
			for _, entry := range req.streams {
				if entry.streamID == streamId {
					endStream = entry.state
					break
				}
			}
			if endStream == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: StreamEnd for unknown stream ID '%s'", streamId)
				return
			}

			// Mark stream as ended
			endStream.active = false
		}

	case cbor.FrameTypeEnd:
		// Check if this belongs to a peer incoming request first
		if _, isPeer := ph.peerIncoming[idKey]; isPeer {
			ph.handlePeerFrame(frame)
			return
		}
		// END: Close entire request (host-initiated request response)
		if req, ok := ph.pendingRequests[idKey]; ok {
			// Mark request as ended - any stream activity after is FATAL
			req.ended = true

			if frame.Payload != nil {
				chunk := &ResponseChunk{
					Payload: frame.Payload,
					Seq:     frame.Seq,
					Offset:  frame.Offset,
					Len:     frame.Len,
					IsEof:   true,
				}
				req.chunks = append(req.chunks, chunk)
			}

			delete(ph.pendingRequests, idKey)
			req.done <- nil
		}

	case cbor.FrameTypeErr:
		// Error response
		if req, ok := ph.pendingRequests[idKey]; ok {
			req.ended = true
			delete(ph.pendingRequests, idKey)
			req.done <- fmt.Errorf("[%s] %s", frame.ErrorCode(), frame.ErrorMessage())
		}

	case cbor.FrameTypeLog:
		// Log message from plugin
		fmt.Printf("[Plugin:%s] %s\n", frame.LogLevel(), frame.LogMessage())

	case cbor.FrameTypeHeartbeat:
		// Heartbeat - send response
		response := cbor.NewHeartbeat(frame.Id)
		ph.writerMu.Lock()
		ph.writer.WriteFrame(response)
		ph.writerMu.Unlock()

	default:
		fmt.Printf("[PluginHost] unexpected frame type: %v\n", frame.FrameType)
	}
}

// Call invokes a capability on the plugin and waits for the response
// Call invokes a capability on the plugin and waits for the response using stream multiplexing
// CallWithArguments sends a cap request with typed arguments and waits for the complete response.
//
// Each argument becomes an independent stream (STREAM_START + CHUNK(s) + STREAM_END).
// This matches the Rust AsyncPluginHost.request_with_arguments() wire format exactly.
func (ph *PluginHost) CallWithArguments(capUrn string, arguments []CapArgumentValue) (*PluginResponse, error) {
	ph.mu.Lock()
	if ph.closed {
		ph.mu.Unlock()
		return nil, fmt.Errorf("host is closed")
	}

	requestID := cbor.NewMessageIdRandom()

	req := &pendingRequest{
		chunks:  make([]*ResponseChunk, 0),
		done:    make(chan error, 1),
		streams: nil,
		ended:   false,
	}
	idKey := requestID.ToString()
	ph.pendingRequests[idKey] = req
	maxChunk := ph.limits.MaxChunk
	ph.mu.Unlock()

	cleanup := func() {
		ph.mu.Lock()
		delete(ph.pendingRequests, idKey)
		ph.mu.Unlock()
	}

	// REQ with empty payload — arguments come as streams
	reqFrame := cbor.NewReq(requestID, capUrn, []byte{}, "application/cbor")
	ph.writerMu.Lock()
	err := ph.writer.WriteFrame(reqFrame)
	ph.writerMu.Unlock()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Each argument becomes an independent stream
	for i, arg := range arguments {
		streamId := fmt.Sprintf("arg-%d", i)

		// STREAM_START with the argument's media_urn
		startFrame := cbor.NewStreamStart(requestID, streamId, arg.MediaUrn)
		ph.writerMu.Lock()
		err = ph.writer.WriteFrame(startFrame)
		ph.writerMu.Unlock()
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to send stream start: %w", err)
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

			chunkFrame := cbor.NewChunk(requestID, streamId, seq, chunkData)
			ph.writerMu.Lock()
			err := ph.writer.WriteFrame(chunkFrame)
			ph.writerMu.Unlock()
			if err != nil {
				cleanup()
				return nil, fmt.Errorf("failed to send chunk: %w", err)
			}

			offset += chunkSize
			seq++
		}

		// STREAM_END
		streamEndFrame := cbor.NewStreamEnd(requestID, streamId)
		ph.writerMu.Lock()
		err = ph.writer.WriteFrame(streamEndFrame)
		ph.writerMu.Unlock()
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("failed to send stream end: %w", err)
		}
	}

	// END closes the entire request
	endFrame := cbor.NewEnd(requestID, nil)
	ph.writerMu.Lock()
	err = ph.writer.WriteFrame(endFrame)
	ph.writerMu.Unlock()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to send end: %w", err)
	}

	// Wait for response
	err = <-req.done
	if err != nil {
		return nil, err
	}

	// Build PluginResponse
	if req.isChunked || len(req.chunks) > 1 {
		return &PluginResponse{
			Type:      PluginResponseTypeStreaming,
			Streaming: req.chunks,
		}, nil
	} else if len(req.chunks) == 1 {
		return &PluginResponse{
			Type:   PluginResponseTypeSingle,
			Single: req.chunks[0].Payload,
		}, nil
	} else {
		return &PluginResponse{
			Type:   PluginResponseTypeSingle,
			Single: []byte{},
		}, nil
	}
}
func (ph *PluginHost) Manifest() []byte {
	return ph.manifest
}

// Limits returns the negotiated protocol limits
func (ph *PluginHost) Limits() cbor.Limits {
	return ph.limits
}

// Close closes the plugin host
func (ph *PluginHost) Close() error {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.closed = true
	return nil
}

// RegisterCapability registers a host-side capability handler for peer invoke.
// The capUrn should contain an op= tag that will be matched against incoming requests.
func (ph *PluginHost) RegisterCapability(capUrn string, handler CapHandler) {
	// Extract the op tag from the cap URN for matching
	op := extractOpTag(capUrn)
	if op == "" {
		return
	}
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.capHandlers[op] = handler
}

// extractOpTag extracts the "op" tag value from a cap URN string.
// E.g. "cap:in=*;op=echo;out=*" → "echo"
func extractOpTag(capUrn string) string {
	// Parse as CapUrn if possible, otherwise use simple extraction
	parsed, err := NewCapUrnFromString(capUrn)
	if err == nil {
		if op, ok := parsed.GetTag("op"); ok {
			return op
		}
		return ""
	}
	// Fallback: simple string extraction for wildcard URNs like "cap:in=*;op=echo;out=*"
	for _, part := range strings.Split(capUrn, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "op=") {
			return part[3:]
		}
	}
	return ""
}

// handlePeerInvoke handles an incoming REQ from a plugin (peer invoke)
func (ph *PluginHost) handlePeerInvoke(frame *cbor.Frame) {
	idKey := frame.Id.ToString()

	// Start tracking the peer request
	capUrn := ""
	if frame.Cap != nil {
		capUrn = *frame.Cap
	}
	contentType := ""
	if frame.ContentType != nil {
		contentType = *frame.ContentType
	}

	ph.peerIncoming[idKey] = &peerIncomingRequest{
		capUrn:      capUrn,
		contentType: contentType,
		ended:       false,
	}
}

// handlePeerFrame processes a frame belonging to an incoming peer request
func (ph *PluginHost) handlePeerFrame(frame *cbor.Frame) {
	idKey := frame.Id.ToString()
	peer, ok := ph.peerIncoming[idKey]
	if !ok {
		return
	}

	switch frame.FrameType {
	case cbor.FrameTypeStreamStart:
		streamId := ""
		if frame.StreamId != nil {
			streamId = *frame.StreamId
		}
		mediaUrn := ""
		if frame.MediaUrn != nil {
			mediaUrn = *frame.MediaUrn
		}
		peer.streams = append(peer.streams, struct {
			streamID string
			stream   *peerIncomingStream
		}{
			streamID: streamId,
			stream: &peerIncomingStream{
				mediaUrn: mediaUrn,
				chunks:   nil,
				complete: false,
			},
		})

	case cbor.FrameTypeChunk:
		streamId := ""
		if frame.StreamId != nil {
			streamId = *frame.StreamId
		}
		for _, entry := range peer.streams {
			if entry.streamID == streamId {
				if frame.Payload != nil {
					entry.stream.chunks = append(entry.stream.chunks, frame.Payload)
				}
				break
			}
		}

	case cbor.FrameTypeStreamEnd:
		streamId := ""
		if frame.StreamId != nil {
			streamId = *frame.StreamId
		}
		for _, entry := range peer.streams {
			if entry.streamID == streamId {
				entry.stream.complete = true
				break
			}
		}

	case cbor.FrameTypeEnd:
		// Concatenate all stream chunks
		var payload []byte
		for _, entry := range peer.streams {
			for _, chunk := range entry.stream.chunks {
				payload = append(payload, chunk...)
			}
		}

		delete(ph.peerIncoming, idKey)

		// Dispatch handler in goroutine (must release lock first)
		reqId := frame.Id
		capUrn := peer.capUrn
		contentType := peer.contentType
		go ph.dispatchPeerHandler(reqId, capUrn, contentType, payload)
	}
}

// dispatchPeerHandler finds and executes the matching handler, sends response back
func (ph *PluginHost) dispatchPeerHandler(reqId cbor.MessageId, capUrn string, contentType string, payload []byte) {
	// Extract op from the request's cap URN
	op := extractOpTag(capUrn)

	ph.mu.Lock()
	handler, ok := ph.capHandlers[op]
	maxChunk := ph.limits.MaxChunk
	ph.mu.Unlock()

	if !ok {
		// No handler — send error
		errFrame := cbor.NewErr(reqId, "NO_HANDLER", fmt.Sprintf("No handler for op=%s", op))
		ph.writerMu.Lock()
		ph.writer.WriteFrame(errFrame)
		ph.writerMu.Unlock()
		return
	}

	result, err := handler(payload)
	if err != nil {
		errFrame := cbor.NewErr(reqId, "HANDLER_ERROR", err.Error())
		ph.writerMu.Lock()
		ph.writer.WriteFrame(errFrame)
		ph.writerMu.Unlock()
		return
	}

	// Send response: STREAM_START + CHUNK(s) + STREAM_END + END
	peerStreamId := fmt.Sprintf("peer-resp-%s", reqId.ToString()[:8])
	peerMediaUrn := contentType
	if peerMediaUrn == "" {
		peerMediaUrn = "media:bytes"
	}

	ph.writerMu.Lock()
	defer ph.writerMu.Unlock()

	// STREAM_START
	ph.writer.WriteFrame(cbor.NewStreamStart(reqId, peerStreamId, peerMediaUrn))

	// CHUNK(s) with proper chunking
	if len(result) > 0 {
		offset := 0
		seq := uint64(0)
		for offset < len(result) {
			chunkSize := len(result) - offset
			if chunkSize > maxChunk {
				chunkSize = maxChunk
			}
			chunkData := result[offset : offset+chunkSize]
			ph.writer.WriteFrame(cbor.NewChunk(reqId, peerStreamId, seq, chunkData))
			offset += chunkSize
			seq++
		}
	}

	// STREAM_END
	ph.writer.WriteFrame(cbor.NewStreamEnd(reqId, peerStreamId))

	// END
	ph.writer.WriteFrame(cbor.NewEnd(reqId, nil))
}
