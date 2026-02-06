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

// pendingRequest tracks a pending request
type pendingRequest struct {
	chunks  [][]byte
	done    chan error
	isChunked bool
}

// PluginResponse represents a response from a plugin
type PluginResponse struct {
	Data []byte
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

// handleFrame processes an incoming frame
func (ph *PluginHost) handleFrame(frame *cbor.Frame) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	idKey := frame.Id.ToString()

	switch frame.FrameType {
	case cbor.FrameTypeRes:
		// Single response
		if req, ok := ph.pendingRequests[idKey]; ok {
			req.chunks = append(req.chunks, frame.Payload)
			delete(ph.pendingRequests, idKey)
			req.done <- nil
		}

	case cbor.FrameTypeChunk:
		// Streaming chunk
		if req, ok := ph.pendingRequests[idKey]; ok {
			req.isChunked = true
			req.chunks = append(req.chunks, frame.Payload)
		}

	case cbor.FrameTypeEnd:
		// Final chunk or end of stream
		if req, ok := ph.pendingRequests[idKey]; ok {
			req.chunks = append(req.chunks, frame.Payload)
			delete(ph.pendingRequests, idKey)
			req.done <- nil
		}

	case cbor.FrameTypeErr:
		// Error response
		if req, ok := ph.pendingRequests[idKey]; ok {
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
func (ph *PluginHost) Call(capUrn string, payload []byte, contentType string) (*PluginResponse, error) {
	ph.mu.Lock()
	if ph.closed {
		ph.mu.Unlock()
		return nil, fmt.Errorf("host is closed")
	}

	// Generate new message ID
	requestID := cbor.NewMessageIdRandom()

	// Create pending request
	req := &pendingRequest{
		chunks: make([][]byte, 0),
		done:   make(chan error, 1),
	}
	idKey := requestID.ToString()
	ph.pendingRequests[idKey] = req
	ph.mu.Unlock()

	// Send request frame
	frame := cbor.NewReq(requestID, capUrn, payload, contentType)
	ph.writerMu.Lock()
	err := ph.writer.WriteFrame(frame)
	ph.writerMu.Unlock()
	if err != nil {
		ph.mu.Lock()
		delete(ph.pendingRequests, idKey)
		ph.mu.Unlock()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	err = <-req.done
	if err != nil {
		return nil, err
	}

	// Concatenate all chunks
	totalLen := 0
	for _, chunk := range req.chunks {
		totalLen += len(chunk)
	}
	result := make([]byte, 0, totalLen)
	for _, chunk := range req.chunks {
		result = append(result, chunk...)
	}

	return &PluginResponse{Data: result}, nil
}

// Manifest returns the plugin manifest received during handshake
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
