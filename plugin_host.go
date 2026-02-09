package capns

import (
	"fmt"
	"io"
	"sync"

	"github.com/filegrind/cap-sdk-go/cbor"
)

// PluginHost manages communication with a plugin process
type PluginHost struct {
	reader          *cbor.FrameReader
	writer          *cbor.FrameWriter
	limits          cbor.Limits
	manifest        []byte
	pendingRequests map[string]*pendingRequest // key is MessageId.ToString()
	mu              sync.Mutex
	writerMu        sync.Mutex
	closed          bool
}

// streamState tracks a single stream within a request
type streamState struct {
	mediaUrn string
	active   bool // false after StreamEnd
}

// pendingRequest tracks a pending request with stream multiplexing
type pendingRequest struct {
	chunks    []*ResponseChunk
	done      chan error
	isChunked bool
	streams   map[string]*streamState // stream_id -> state
	ended     bool                    // true after END frame - any stream activity after is FATAL
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
	// RES frame REMOVED - old protocol no longer supported

	case cbor.FrameTypeStreamStart:
		// STREAM_START: Announce new stream
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
			if _, exists := req.streams[streamId]; exists {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: duplicate stream ID '%s'", streamId)
				return
			}

			// Track new stream
			req.streams[streamId] = &streamState{
				mediaUrn: mediaUrn,
				active:   true,
			}
		}

	case cbor.FrameTypeChunk:
		// CHUNK: Data chunk for a stream
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
			stream, exists := req.streams[streamId]
			if !exists {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: chunk for unknown stream ID '%s'", streamId)
				return
			}
			if !stream.active {
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
		// STREAM_END: End a specific stream
		if req, ok := ph.pendingRequests[idKey]; ok {
			// STRICT validation: must have stream_id
			if frame.StreamId == nil {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: StreamEnd missing stream_id")
				return
			}

			streamId := *frame.StreamId

			// FAIL HARD: Unknown stream
			stream, exists := req.streams[streamId]
			if !exists {
				delete(ph.pendingRequests, idKey)
				req.done <- fmt.Errorf("protocol violation: StreamEnd for unknown stream ID '%s'", streamId)
				return
			}

			// Mark stream as ended
			stream.active = false
		}

	case cbor.FrameTypeEnd:
		// END: Close entire request
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
func (ph *PluginHost) Call(capUrn string, payload []byte, contentType string) (*PluginResponse, error) {
	ph.mu.Lock()
	if ph.closed {
		ph.mu.Unlock()
		return nil, fmt.Errorf("host is closed")
	}

	// Generate new message ID
	requestID := cbor.NewMessageIdRandom()

	// Create pending request with stream tracking
	req := &pendingRequest{
		chunks:  make([]*ResponseChunk, 0),
		done:    make(chan error, 1),
		streams: make(map[string]*streamState),
		ended:   false,
	}
	idKey := requestID.ToString()
	ph.pendingRequests[idKey] = req
	maxChunk := ph.limits.MaxChunk
	ph.mu.Unlock()

	// Send request using stream multiplexing protocol:
	// 1. REQ (empty payload)
	// 2. STREAM_START + CHUNK(s) + STREAM_END for the payload
	// 3. END

	// Send REQ frame with cap_urn, empty payload
	reqFrame := cbor.NewReq(requestID, capUrn, []byte{}, "application/cbor")
	ph.writerMu.Lock()
	err := ph.writer.WriteFrame(reqFrame)
	ph.writerMu.Unlock()
	if err != nil {
		ph.mu.Lock()
		delete(ph.pendingRequests, idKey)
		ph.mu.Unlock()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Send payload as a single stream
	streamId := "arg-0" // Single argument stream
	mediaUrn := contentType
	if mediaUrn == "" {
		mediaUrn = "media:bytes"
	}

	// STREAM_START
	startFrame := cbor.NewStreamStart(requestID, streamId, mediaUrn)
	ph.writerMu.Lock()
	err = ph.writer.WriteFrame(startFrame)
	ph.writerMu.Unlock()
	if err != nil {
		ph.mu.Lock()
		delete(ph.pendingRequests, idKey)
		ph.mu.Unlock()
		return nil, fmt.Errorf("failed to send stream start: %w", err)
	}

	// CHUNK(s)
	offset := 0
	seq := uint64(0)
	for offset < len(payload) {
		remaining := len(payload) - offset
		chunkSize := remaining
		if chunkSize > maxChunk {
			chunkSize = maxChunk
		}
		chunkData := payload[offset : offset+chunkSize]

		chunkFrame := cbor.NewChunk(requestID, streamId, seq, chunkData)
		ph.writerMu.Lock()
		err := ph.writer.WriteFrame(chunkFrame)
		ph.writerMu.Unlock()
		if err != nil {
			ph.mu.Lock()
			delete(ph.pendingRequests, idKey)
			ph.mu.Unlock()
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
		ph.mu.Lock()
		delete(ph.pendingRequests, idKey)
		ph.mu.Unlock()
		return nil, fmt.Errorf("failed to send stream end: %w", err)
	}

	// END
	endFrame := cbor.NewEnd(requestID, nil)
	ph.writerMu.Lock()
	err = ph.writer.WriteFrame(endFrame)
	ph.writerMu.Unlock()
	if err != nil {
		ph.mu.Lock()
		delete(ph.pendingRequests, idKey)
		ph.mu.Unlock()
		return nil, fmt.Errorf("failed to send end: %w", err)
	}

	// Wait for response
	err = <-req.done
	if err != nil {
		return nil, err
	}

	// Build PluginResponse based on whether it was chunked
	if req.isChunked || len(req.chunks) > 1 {
		// Streaming response
		return &PluginResponse{
			Type:      PluginResponseTypeStreaming,
			Streaming: req.chunks,
		}, nil
	} else if len(req.chunks) == 1 {
		// Single response
		return &PluginResponse{
			Type:   PluginResponseTypeSingle,
			Single: req.chunks[0].Payload,
		}, nil
	} else {
		// Empty response
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
