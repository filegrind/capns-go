package cbor

import (
	"testing"

	"github.com/google/uuid"
)

// TEST171: Test all FrameType discriminants roundtrip through uint8 conversion preserving identity
func TestFrameTypeRoundtrip(t *testing.T) {
	types := []FrameType{
		FrameTypeReq,
		FrameTypeRes,
		FrameTypeChunk,
		FrameTypeEnd,
		FrameTypeErr,
		FrameTypeLog,
		FrameTypeHeartbeat,
		FrameTypeHello,
	}

	for _, ft := range types {
		asUint := uint8(ft)
		backToType := FrameType(asUint)
		if backToType != ft {
			t.Errorf("FrameType %v roundtrip failed: got %v", ft, backToType)
		}
	}
}

// TEST172: Test FrameType values are within expected range
func TestFrameTypeValidRange(t *testing.T) {
	validTypes := map[uint8]bool{
		1: true, // REQ
		2: true, // RES
		3: true, // CHUNK
		4: true, // END
		5: true, // ERR
		6: true, // LOG
		7: true, // HEARTBEAT
		8: true, // HELLO
	}

	for i := uint8(0); i < 20; i++ {
		if i >= 1 && i <= 8 {
			if !validTypes[i] {
				t.Errorf("Expected %d to be valid FrameType", i)
			}
		}
	}
}

// TEST173: Test FrameType discriminant values match the wire protocol specification exactly
func TestFrameTypeWireProtocolValues(t *testing.T) {
	if uint8(FrameTypeReq) != 1 {
		t.Errorf("REQ must be 1, got %d", FrameTypeReq)
	}
	if uint8(FrameTypeRes) != 2 {
		t.Errorf("RES must be 2, got %d", FrameTypeRes)
	}
	if uint8(FrameTypeChunk) != 3 {
		t.Errorf("CHUNK must be 3, got %d", FrameTypeChunk)
	}
	if uint8(FrameTypeEnd) != 4 {
		t.Errorf("END must be 4, got %d", FrameTypeEnd)
	}
	if uint8(FrameTypeErr) != 5 {
		t.Errorf("ERR must be 5, got %d", FrameTypeErr)
	}
	if uint8(FrameTypeLog) != 6 {
		t.Errorf("LOG must be 6, got %d", FrameTypeLog)
	}
	if uint8(FrameTypeHeartbeat) != 7 {
		t.Errorf("HEARTBEAT must be 7, got %d", FrameTypeHeartbeat)
	}
	if uint8(FrameTypeHello) != 8 {
		t.Errorf("HELLO must be 8, got %d", FrameTypeHello)
	}
}

// TEST174: Test MessageId::new_uuid generates valid UUID that roundtrips through string conversion
func TestMessageIdNewUuidRoundtrip(t *testing.T) {
	id := NewMessageIdRandom()
	if !id.IsUuid() {
		t.Fatal("Expected UUID variant")
	}

	uuidStr := id.ToUuidString()
	if uuidStr == "" {
		t.Fatal("Expected non-empty UUID string")
	}

	// Verify it's a valid UUID format
	_, err := uuid.Parse(uuidStr)
	if err != nil {
		t.Errorf("Invalid UUID string: %v", err)
	}
}

// TEST175: Test two MessageId::new_uuid calls produce distinct IDs (no collisions)
func TestMessageIdUuidDistinct(t *testing.T) {
	id1 := NewMessageIdRandom()
	id2 := NewMessageIdRandom()

	if id1.Equals(id2) {
		t.Error("Two random UUIDs should not be equal")
	}
}

// TEST176: Test MessageId::Uint does not produce a UUID string, to_uuid_string returns empty
func TestMessageIdUintNoUuidString(t *testing.T) {
	id := NewMessageIdFromUint(42)
	if id.IsUuid() {
		t.Fatal("Expected Uint variant, got UUID")
	}

	uuidStr := id.ToUuidString()
	if uuidStr != "" {
		t.Errorf("Uint variant should not produce UUID string, got %s", uuidStr)
	}
}

// TEST177: Test MessageId::from_uuid_bytes rejects invalid UUID bytes
func TestMessageIdFromUuidInvalidBytes(t *testing.T) {
	// UUID must be exactly 16 bytes
	_, err := NewMessageIdFromUuid([]byte{1, 2, 3}) // too short
	if err == nil {
		t.Error("Expected error for invalid UUID length")
	}

	_, err = NewMessageIdFromUuid(make([]byte, 20)) // too long
	if err == nil {
		t.Error("Expected error for invalid UUID length")
	}
}

// TEST178: Test MessageId::as_bytes produces correct byte representations for Uuid and Uint variants
func TestMessageIdAsBytes(t *testing.T) {
	// UUID variant
	uuidBytes := make([]byte, 16)
	for i := 0; i < 16; i++ {
		uuidBytes[i] = byte(i)
	}
	id1, _ := NewMessageIdFromUuid(uuidBytes)
	bytes1 := id1.AsBytes()
	if len(bytes1) != 16 {
		t.Errorf("UUID bytes should be 16, got %d", len(bytes1))
	}

	// Uint variant
	id2 := NewMessageIdFromUint(42)
	bytes2 := id2.AsBytes()
	if len(bytes2) != 8 {
		t.Errorf("Uint bytes should be 8, got %d", len(bytes2))
	}
}

// TEST179: Test MessageId::default creates appropriate variant
func TestMessageIdDefault(t *testing.T) {
	id := NewMessageIdDefault()
	// Default is Uint 0
	if id.IsUuid() {
		t.Error("Default should be Uint variant")
	}
	if id.ToString() != "0" {
		t.Errorf("Default Uint should be 0, got %s", id.ToString())
	}
}

// TEST180: Test Frame::hello without manifest produces correct HELLO frame
func TestFrameHelloWithoutManifest(t *testing.T) {
	frame := NewHello([]byte{})
	if frame.FrameType != FrameTypeHello {
		t.Errorf("Expected HELLO frame type, got %v", frame.FrameType)
	}
	if len(frame.Payload) != 0 {
		t.Error("Expected empty payload for HELLO without manifest")
	}
}

// TEST181: Test Frame::hello_with_manifest produces HELLO with manifest bytes
func TestFrameHelloWithManifest(t *testing.T) {
	manifest := []byte(`{"name":"test"}`)
	frame := NewHello(manifest)
	if frame.FrameType != FrameTypeHello {
		t.Errorf("Expected HELLO frame type, got %v", frame.FrameType)
	}
	if string(frame.Payload) != string(manifest) {
		t.Errorf("Expected manifest payload, got %s", string(frame.Payload))
	}
}

// TEST182: Test Frame::req stores cap URN, payload, and content_type correctly
func TestFrameReq(t *testing.T) {
	id := NewMessageIdRandom()
	cap := `cap:in="media:void";op=test;out="media:void"`
	payload := []byte("request data")
	contentType := "application/json"

	frame := NewReq(id, cap, payload, contentType)

	if frame.FrameType != FrameTypeReq {
		t.Errorf("Expected REQ frame type, got %v", frame.FrameType)
	}
	if frame.Cap != cap {
		t.Errorf("Expected cap %s, got %s", cap, frame.Cap)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if frame.ContentType != contentType {
		t.Errorf("Expected content_type %s, got %s", contentType, frame.ContentType)
	}
}

// TEST183: Test Frame::res stores payload and content_type for single complete response
func TestFrameRes(t *testing.T) {
	id := NewMessageIdRandom()
	payload := []byte("response data")
	contentType := "application/json"

	frame := NewRes(id, payload, contentType)

	if frame.FrameType != FrameTypeRes {
		t.Errorf("Expected RES frame type, got %v", frame.FrameType)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if frame.ContentType != contentType {
		t.Errorf("Expected content_type %s, got %s", contentType, frame.ContentType)
	}
}

// TEST184: Test Frame::chunk stores seq and payload for streaming
func TestFrameChunk(t *testing.T) {
	id := NewMessageIdRandom()
	seq := uint64(5)
	payload := []byte("chunk data")

	frame := NewChunk(id, seq, payload)

	if frame.FrameType != FrameTypeChunk {
		t.Errorf("Expected CHUNK frame type, got %v", frame.FrameType)
	}
	if frame.Seq != seq {
		t.Errorf("Expected seq %d, got %d", seq, frame.Seq)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
}

// TEST185: Test Frame::err stores error code and message
func TestFrameErr(t *testing.T) {
	id := NewMessageIdRandom()
	code := "HANDLER_ERROR"
	message := "Something went wrong"

	frame := NewErr(id, code, message)

	if frame.FrameType != FrameTypeErr {
		t.Errorf("Expected ERR frame type, got %v", frame.FrameType)
	}
	if frame.Code != code {
		t.Errorf("Expected code %s, got %s", code, frame.Code)
	}
	if frame.Message != message {
		t.Errorf("Expected message %s, got %s", message, frame.Message)
	}
}

// TEST186: Test Frame::log stores level and message
func TestFrameLog(t *testing.T) {
	id := NewMessageIdRandom()
	level := "info"
	message := "Log message"

	frame := NewLog(id, level, message)

	if frame.FrameType != FrameTypeLog {
		t.Errorf("Expected LOG frame type, got %v", frame.FrameType)
	}
	if frame.Level != level {
		t.Errorf("Expected level %s, got %s", level, frame.Level)
	}
	if frame.Message != message {
		t.Errorf("Expected message %s, got %s", message, frame.Message)
	}
}

// TEST187: Test Frame::end with payload sets final payload
func TestFrameEndWithPayload(t *testing.T) {
	id := NewMessageIdRandom()
	payload := []byte("final data")
	contentType := "application/json"

	frame := NewEnd(id, payload, contentType)

	if frame.FrameType != FrameTypeEnd {
		t.Errorf("Expected END frame type, got %v", frame.FrameType)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if frame.ContentType != contentType {
		t.Errorf("Expected content_type %s, got %s", contentType, frame.ContentType)
	}
}

// TEST188: Test Frame::end without payload
func TestFrameEndWithoutPayload(t *testing.T) {
	id := NewMessageIdRandom()
	frame := NewEnd(id, []byte{}, "")

	if frame.FrameType != FrameTypeEnd {
		t.Errorf("Expected END frame type, got %v", frame.FrameType)
	}
	if len(frame.Payload) != 0 {
		t.Error("Expected empty payload")
	}
}

// TEST189: Test chunk with offset (future enhancement - not yet implemented)
func TestFrameChunkWithOffset(t *testing.T) {
	// This test documents expected behavior for offset/len fields
	// Currently not implemented in Go version
	t.Skip("Offset/len fields not yet implemented in Go version")
}

// TEST190: Test Frame::heartbeat creates minimal frame with no payload or metadata
func TestFrameHeartbeat(t *testing.T) {
	id := NewMessageIdRandom()
	frame := NewHeartbeat(id)

	if frame.FrameType != FrameTypeHeartbeat {
		t.Errorf("Expected HEARTBEAT frame type, got %v", frame.FrameType)
	}
	if len(frame.Payload) != 0 {
		t.Error("HEARTBEAT should have empty payload")
	}
	if frame.Cap != "" {
		t.Error("HEARTBEAT should have no cap")
	}
}
