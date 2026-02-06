package cbor

import (
	"bytes"
	"io"
	"testing"
)

// TEST205: Test REQ frame encode/decode roundtrip preserves all fields
func TestReqFrameRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	cap := `cap:in="media:void";op=test;out="media:void"`
	payload := []byte("test payload")
	contentType := "application/json"

	original := NewReq(id, cap, payload, contentType)
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.FrameType != original.FrameType {
		t.Error("FrameType mismatch")
	}
	if decoded.Cap != original.Cap {
		t.Errorf("Cap mismatch: expected %s, got %s", original.Cap, decoded.Cap)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Error("Payload mismatch")
	}
	if decoded.ContentType != original.ContentType {
		t.Errorf("ContentType mismatch: expected %s, got %s", original.ContentType, decoded.ContentType)
	}
}

// TEST206: Test HELLO frame encode/decode roundtrip preserves metadata
func TestHelloFrameRoundtrip(t *testing.T) {
	limits := DefaultLimits()
	limitsData, _ := EncodeCBOR(limits)
	original := NewHello(limitsData)

	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.FrameType != FrameTypeHello {
		t.Error("FrameType mismatch")
	}
	if len(decoded.Payload) == 0 {
		t.Error("Expected limits payload")
	}
}

// TEST207: Test ERR frame encode/decode roundtrip preserves error code and message
func TestErrFrameRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	code := "HANDLER_ERROR"
	message := "Something failed"

	original := NewErr(id, code, message)
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Code != code {
		t.Errorf("Code mismatch: expected %s, got %s", code, decoded.Code)
	}
	if decoded.Message != message {
		t.Errorf("Message mismatch: expected %s, got %s", message, decoded.Message)
	}
}

// TEST208: Test LOG frame encode/decode roundtrip preserves level and message
func TestLogFrameRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	level := "info"
	message := "Log entry"

	original := NewLog(id, level, message)
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Level != level {
		t.Errorf("Level mismatch: expected %s, got %s", level, decoded.Level)
	}
	if decoded.Message != message {
		t.Errorf("Message mismatch: expected %s, got %s", message, decoded.Message)
	}
}

// TEST209: Test RES frame encode/decode roundtrip preserves payload and content_type
func TestResFrameRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	payload := []byte("response data")
	contentType := "application/json"

	original := NewRes(id, payload, contentType)
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if string(decoded.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if decoded.ContentType != contentType {
		t.Errorf("ContentType mismatch")
	}
}

// TEST210: Test END frame encode/decode roundtrip preserves payload
func TestEndFrameRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	payload := []byte("final data")
	contentType := "application/json"

	original := NewEnd(id, payload, contentType)
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.FrameType != FrameTypeEnd {
		t.Error("FrameType mismatch")
	}
	if string(decoded.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
}

// TEST211: Test HELLO with manifest encode/decode roundtrip preserves manifest bytes
func TestHelloWithManifestRoundtrip(t *testing.T) {
	manifest := []byte(`{"name":"test","version":"1.0.0"}`)
	original := NewHello(manifest)

	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if string(decoded.Payload) != string(manifest) {
		t.Errorf("Manifest mismatch: expected %s, got %s", string(manifest), string(decoded.Payload))
	}
}

// TEST212: Test chunk encode/decode roundtrip (offset/len not yet implemented)
func TestChunkWithOffsetRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	seq := uint64(3)
	payload := []byte("chunk data")

	original := NewChunk(id, seq, payload)
	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Seq != seq {
		t.Errorf("Seq mismatch: expected %d, got %d", seq, decoded.Seq)
	}
	if string(decoded.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
}

// TEST213: Test heartbeat frame encode/decode roundtrip preserves ID with no extra fields
func TestHeartbeatRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	original := NewHeartbeat(id)

	encoded, err := EncodeFrame(original)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.FrameType != FrameTypeHeartbeat {
		t.Error("FrameType mismatch")
	}
	if len(decoded.Payload) != 0 {
		t.Error("HEARTBEAT should have empty payload")
	}
}

// TEST214: Test write_frame/read_frame IO roundtrip through length-prefixed wire format
func TestFrameIOroundtrip(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)
	reader := NewFrameReader(&buf)

	id := NewMessageIdRandom()
	original := NewReq(id, `cap:in="media:void";op=test;out="media:void"`, []byte("test"), "application/json")

	// Write frame
	if err := writer.WriteFrame(original); err != nil {
		t.Fatalf("WriteFrame failed: %v", err)
	}

	// Read frame
	decoded, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if decoded.Cap != original.Cap {
		t.Error("Cap mismatch after I/O roundtrip")
	}
}

// TEST215: Test reading multiple sequential frames from a single buffer
func TestReadMultipleFrames(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)

	// Write three frames
	id1 := NewMessageIdFromUint(1)
	id2 := NewMessageIdFromUint(2)
	id3 := NewMessageIdFromUint(3)

	writer.WriteFrame(NewHeartbeat(id1))
	writer.WriteFrame(NewHeartbeat(id2))
	writer.WriteFrame(NewHeartbeat(id3))

	// Read them back
	reader := NewFrameReader(&buf)
	frame1, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("Read frame 1 failed: %v", err)
	}
	frame2, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("Read frame 2 failed: %v", err)
	}
	frame3, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("Read frame 3 failed: %v", err)
	}

	if frame1.FrameType != FrameTypeHeartbeat || frame2.FrameType != FrameTypeHeartbeat || frame3.FrameType != FrameTypeHeartbeat {
		t.Error("Frame types mismatch")
	}
}

// TEST216: Test write_frame rejects frames exceeding max_frame limit
func TestWriteFrameRejectsOversized(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)

	// Set a small limit
	writer.SetLimits(Limits{MaxFrame: 100, MaxChunk: 50})

	// Create a frame with large payload that will exceed limit when encoded
	id := NewMessageIdRandom()
	largePayload := make([]byte, 200)
	frame := NewReq(id, `cap:in="media:void";op=test;out="media:void"`, largePayload, "")

	err := writer.WriteFrame(frame)
	if err == nil {
		t.Error("Expected error for oversized frame, got nil")
	}
}

// TEST217: Test read_frame rejects incoming frames exceeding the negotiated max_frame limit
func TestReadFrameRejectsOversized(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)

	// Write with default limits
	id := NewMessageIdRandom()
	largePayload := make([]byte, 1000)
	frame := NewReq(id, `cap:in="media:void";op=test;out="media:void"`, largePayload, "")
	writer.WriteFrame(frame)

	// Try to read with much smaller limit
	reader := NewFrameReader(&buf)
	reader.SetLimits(Limits{MaxFrame: 100, MaxChunk: 50})

	_, err := reader.ReadFrame()
	if err == nil {
		t.Error("Expected error for oversized frame, got nil")
	}
}

// TEST218: Test write_chunked splits data into chunks respecting max_chunk
func TestWriteChunked(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)
	writer.SetLimits(Limits{MaxFrame: DefaultMaxFrame, MaxChunk: 100})

	id := NewMessageIdRandom()
	data := make([]byte, 250) // Will be split into 3 chunks: 100 + 100 + 50

	err := writer.WriteResponseWithChunking(id, data)
	if err != nil {
		t.Fatalf("WriteResponseWithChunking failed: %v", err)
	}

	// Read back and verify we got multiple frames
	reader := NewFrameReader(&buf)
	var chunks [][]byte

	for {
		frame, err := reader.ReadFrame()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadFrame failed: %v", err)
		}
		chunks = append(chunks, frame.Payload)
		if frame.FrameType == FrameTypeEnd {
			break
		}
	}

	if len(chunks) < 2 {
		t.Errorf("Expected multiple chunks, got %d", len(chunks))
	}
}

// TEST219: Test write_chunked with empty data produces a single END frame
func TestWriteChunkedEmpty(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)

	id := NewMessageIdRandom()
	err := writer.WriteResponseWithChunking(id, []byte{})
	if err != nil {
		t.Fatalf("WriteResponseWithChunking failed: %v", err)
	}

	reader := NewFrameReader(&buf)
	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if frame.FrameType != FrameTypeEnd {
		t.Errorf("Expected END frame for empty data, got %v", frame.FrameType)
	}
}

// TEST220: Test write_chunked with data exactly equal to max_chunk produces one chunk
func TestWriteChunkedExactChunkSize(t *testing.T) {
	var buf bytes.Buffer
	writer := NewFrameWriter(&buf)
	writer.SetLimits(Limits{MaxFrame: DefaultMaxFrame, MaxChunk: 100})

	id := NewMessageIdRandom()
	data := make([]byte, 100) // Exactly max_chunk

	err := writer.WriteResponseWithChunking(id, data)
	if err != nil {
		t.Fatalf("WriteResponseWithChunking failed: %v", err)
	}

	reader := NewFrameReader(&buf)
	frame, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	// Should be a single END frame (not CHUNK + END)
	if frame.FrameType != FrameTypeEnd {
		t.Errorf("Expected END frame, got %v", frame.FrameType)
	}
}

// TEST221: Test read_frame returns Ok(None) on clean EOF (empty stream)
func TestReadFrameEOF(t *testing.T) {
	var buf bytes.Buffer // Empty buffer
	reader := NewFrameReader(&buf)

	_, err := reader.ReadFrame()
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}
}

// TEST222: Test read_frame handles truncated length prefix
func TestReadFrameTruncatedLengthPrefix(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0x00, 0x00}) // Only 2 bytes of 4-byte length prefix
	reader := NewFrameReader(buf)

	_, err := reader.ReadFrame()
	if err == nil {
		t.Error("Expected error for truncated length prefix")
	}
}

// TEST223: Test read_frame returns error on truncated frame body
func TestReadFrameTruncatedBody(t *testing.T) {
	var buf bytes.Buffer
	// Write a length prefix indicating 100 bytes
	lengthPrefix := []byte{0x00, 0x00, 0x00, 0x64} // 100 in big-endian
	buf.Write(lengthPrefix)
	// But only write 10 bytes of body
	buf.Write(make([]byte, 10))

	reader := NewFrameReader(&buf)
	_, err := reader.ReadFrame()
	if err == nil {
		t.Error("Expected error for truncated frame body")
	}
}

// TEST224: Test MessageId::Uint roundtrips through encode/decode
func TestMessageIdUintRoundtrip(t *testing.T) {
	id := NewMessageIdFromUint(42)
	frame := NewHeartbeat(id)

	encoded, err := EncodeFrame(frame)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeFrame(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Id.ToString() != "42" {
		t.Errorf("Expected ID '42', got '%s'", decoded.Id.ToString())
	}
}
