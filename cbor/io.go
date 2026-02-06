package cbor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	cbor2 "github.com/fxamacker/cbor/v2"
)

// FrameReader reads length-prefixed CBOR frames from a stream
type FrameReader struct {
	reader io.Reader
	limits Limits
}

// NewFrameReader creates a new FrameReader
func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{
		reader: r,
		limits: DefaultLimits(),
	}
}

// SetLimits updates the reader's limits
func (fr *FrameReader) SetLimits(limits Limits) {
	fr.limits = limits
}

// ReadFrame reads a single frame from the stream
func (fr *FrameReader) ReadFrame() (*Frame, error) {
	// Read 4-byte length prefix (big-endian)
	var lengthBuf [4]byte
	if _, err := io.ReadFull(fr.reader, lengthBuf[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lengthBuf[:])

	// Enforce max_frame limit
	if int(length) > fr.limits.MaxFrame {
		return nil, fmt.Errorf("frame size %d exceeds max_frame limit %d", length, fr.limits.MaxFrame)
	}

	// Hard limit check
	if int(length) > MaxFrameHardLimit {
		return nil, fmt.Errorf("frame size %d exceeds hard limit %d", length, MaxFrameHardLimit)
	}

	// Read CBOR payload
	frameBuf := make([]byte, length)
	if _, err := io.ReadFull(fr.reader, frameBuf); err != nil {
		return nil, err
	}

	// Decode frame
	return DecodeFrame(frameBuf)
}

// FrameWriter writes length-prefixed CBOR frames to a stream
type FrameWriter struct {
	writer io.Writer
	limits Limits
}

// NewFrameWriter creates a new FrameWriter
func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{
		writer: w,
		limits: DefaultLimits(),
	}
}

// SetLimits updates the writer's limits
func (fw *FrameWriter) SetLimits(limits Limits) {
	fw.limits = limits
}

// WriteFrame writes a single frame to the stream
func (fw *FrameWriter) WriteFrame(frame *Frame) error {
	// Encode frame to CBOR
	frameBuf, err := EncodeFrame(frame)
	if err != nil {
		return err
	}

	// Enforce max_frame limit
	if len(frameBuf) > fw.limits.MaxFrame {
		return fmt.Errorf("encoded frame size %d exceeds max_frame limit %d", len(frameBuf), fw.limits.MaxFrame)
	}

	// Hard limit check
	if len(frameBuf) > MaxFrameHardLimit {
		return fmt.Errorf("encoded frame size %d exceeds hard limit %d", len(frameBuf), MaxFrameHardLimit)
	}

	// Write 4-byte length prefix (big-endian)
	var lengthBuf [4]byte
	binary.BigEndian.PutUint32(lengthBuf[:], uint32(len(frameBuf)))
	if _, err := fw.writer.Write(lengthBuf[:]); err != nil {
		return err
	}

	// Write CBOR payload
	if _, err := fw.writer.Write(frameBuf); err != nil {
		return err
	}

	return nil
}

// WriteResponseWithChunking writes a response with automatic chunking for large payloads
func (fw *FrameWriter) WriteResponseWithChunking(requestId MessageId, payload []byte) error {
	if len(payload) <= fw.limits.MaxChunk {
		// Small payload: single END frame
		frame := NewEnd(requestId, payload)
		return fw.WriteFrame(frame)
	}

	// Large payload: CHUNK frames + final END
	offset := 0
	seq := uint64(0)

	for offset < len(payload) {
		remaining := len(payload) - offset
		chunkSize := min(remaining, fw.limits.MaxChunk)
		chunkData := payload[offset : offset+chunkSize]
		offset += chunkSize

		if offset < len(payload) {
			// Not the last chunk - send CHUNK frame
			frame := NewChunk(requestId, seq, chunkData)
			if err := fw.WriteFrame(frame); err != nil {
				return err
			}
			seq++
		} else {
			// Last chunk - send END frame with remaining data
			frame := NewEnd(requestId, chunkData)
			return fw.WriteFrame(frame)
		}
	}

	return nil
}

// HandshakeAccept performs handshake from plugin side
func HandshakeAccept(reader *FrameReader, writer *FrameWriter, manifestData []byte) (Limits, error) {
	// 1. Read HELLO from host
	helloFrame, err := reader.ReadFrame()
	if err != nil {
		return Limits{}, fmt.Errorf("failed to read HELLO: %w", err)
	}

	if helloFrame.FrameType != FrameTypeHello {
		return Limits{}, errors.New("expected HELLO frame")
	}

	// 2. Decode host limits from Meta map
	var hostLimits Limits
	if helloFrame.Meta != nil {
		if maxFrame, ok := helloFrame.Meta["max_frame"].(int); ok {
			hostLimits.MaxFrame = maxFrame
		}
		if maxChunk, ok := helloFrame.Meta["max_chunk"].(int); ok {
			hostLimits.MaxChunk = maxChunk
		}
	}
	if hostLimits.MaxFrame == 0 || hostLimits.MaxChunk == 0 {
		hostLimits = DefaultLimits()
	}

	// 3. Send HELLO back with manifest
	responseFrame := NewHelloWithManifest(DefaultMaxFrame, DefaultMaxChunk, manifestData)
	if err := writer.WriteFrame(responseFrame); err != nil {
		return Limits{}, fmt.Errorf("failed to write HELLO response: %w", err)
	}

	// 4. Negotiate limits (min of both sides)
	negotiated := NegotiateLimits(DefaultLimits(), hostLimits)

	return negotiated, nil
}

// HandshakeInitiate performs handshake from host side
func HandshakeInitiate(reader *FrameReader, writer *FrameWriter) ([]byte, Limits, error) {
	// 1. Send HELLO with our limits
	helloFrame := NewHello(DefaultMaxFrame, DefaultMaxChunk)
	if err := writer.WriteFrame(helloFrame); err != nil {
		return nil, Limits{}, fmt.Errorf("failed to write HELLO: %w", err)
	}

	// 2. Read HELLO response with manifest
	responseFrame, err := reader.ReadFrame()
	if err != nil {
		return nil, Limits{}, fmt.Errorf("failed to read HELLO response: %w", err)
	}

	if responseFrame.FrameType != FrameTypeHello {
		return nil, Limits{}, errors.New("expected HELLO response")
	}

	// 3. Extract manifest from Meta map
	var manifestData []byte
	if responseFrame.Meta != nil {
		if manifest, ok := responseFrame.Meta["manifest"].([]byte); ok {
			manifestData = manifest
		}
	}

	// 4. Extract plugin limits from Meta map
	var pluginLimits Limits
	if responseFrame.Meta != nil {
		if maxFrame, ok := responseFrame.Meta["max_frame"].(int); ok {
			pluginLimits.MaxFrame = maxFrame
		}
		if maxChunk, ok := responseFrame.Meta["max_chunk"].(int); ok {
			pluginLimits.MaxChunk = maxChunk
		}
	}
	if pluginLimits.MaxFrame == 0 || pluginLimits.MaxChunk == 0 {
		pluginLimits = DefaultLimits()
	}

	// 5. Negotiate limits
	negotiated := NegotiateLimits(DefaultLimits(), pluginLimits)

	return manifestData, negotiated, nil
}

// EncodeCBOR encodes Limits to CBOR
func EncodeCBOR(limits Limits) ([]byte, error) {
	m := map[string]int{
		"max_frame": limits.MaxFrame,
		"max_chunk": limits.MaxChunk,
	}
	return cbor2.Marshal(m)
}

// DecodeCBOR decodes CBOR to Limits
func DecodeCBOR(data []byte, limits *Limits) error {
	var m map[string]interface{}
	if err := cbor2.Unmarshal(data, &m); err != nil {
		return err
	}

	if maxFrameVal, ok := m["max_frame"]; ok {
		if maxFrame, ok := maxFrameVal.(uint64); ok {
			limits.MaxFrame = int(maxFrame)
		}
	}
	if maxChunkVal, ok := m["max_chunk"]; ok {
		if maxChunk, ok := maxChunkVal.(uint64); ok {
			limits.MaxChunk = int(maxChunk)
		}
	}

	return nil
}
