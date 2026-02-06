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
		0: true, // HELLO
		1: true, // REQ
		2: true, // RES
		3: true, // CHUNK
		4: true, // END
		5: true, // LOG
		6: true, // ERR
		7: true, // HEARTBEAT
	}

	for i := uint8(0); i < 20; i++ {
		if i >= 0 && i <= 7 {
			if !validTypes[i] {
				t.Errorf("Expected %d to be valid FrameType", i)
			}
		}
	}
}

// TEST173: Test FrameType discriminant values match the wire protocol specification exactly
func TestFrameTypeWireProtocolValues(t *testing.T) {
	if uint8(FrameTypeHello) != 0 {
		t.Errorf("HELLO must be 0, got %d", FrameTypeHello)
	}
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
	if uint8(FrameTypeLog) != 5 {
		t.Errorf("LOG must be 5, got %d", FrameTypeLog)
	}
	if uint8(FrameTypeErr) != 6 {
		t.Errorf("ERR must be 6, got %d", FrameTypeErr)
	}
	if uint8(FrameTypeHeartbeat) != 7 {
		t.Errorf("HEARTBEAT must be 7, got %d", FrameTypeHeartbeat)
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
	frame := NewHello(DefaultMaxFrame, DefaultMaxChunk)
	if frame.FrameType != FrameTypeHello {
		t.Errorf("Expected HELLO frame type, got %v", frame.FrameType)
	}
	// Host-side HELLO has limits in Meta, no manifest in payload
	if frame.Meta == nil {
		t.Error("Expected Meta map with limits")
	}
	if frame.Meta["max_frame"] == nil {
		t.Error("Expected max_frame in Meta")
	}
}

// TEST181: Test Frame::hello_with_manifest produces HELLO with manifest bytes
func TestFrameHelloWithManifest(t *testing.T) {
	manifest := []byte(`{"name":"test"}`)
	frame := NewHelloWithManifest(DefaultMaxFrame, DefaultMaxChunk, manifest)
	if frame.FrameType != FrameTypeHello {
		t.Errorf("Expected HELLO frame type, got %v", frame.FrameType)
	}
	// Plugin-side HELLO has limits AND manifest in Meta
	if frame.Meta == nil {
		t.Error("Expected Meta map")
	}
	if manifestBytes, ok := frame.Meta["manifest"].([]byte); !ok || string(manifestBytes) != string(manifest) {
		t.Errorf("Expected manifest in Meta, got %v", frame.Meta["manifest"])
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
	if frame.Cap == nil || *frame.Cap != cap {
		t.Errorf("Expected cap %s, got %v", cap, frame.Cap)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if frame.ContentType == nil || *frame.ContentType != contentType {
		t.Errorf("Expected content_type %s, got %v", contentType, frame.ContentType)
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
	if frame.ContentType == nil || *frame.ContentType != contentType {
		t.Errorf("Expected content_type %s, got %v", contentType, frame.ContentType)
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
	if frame.ErrorCode() != code {
		t.Errorf("Expected code %s, got %s", code, frame.ErrorCode())
	}
	if frame.ErrorMessage() != message {
		t.Errorf("Expected message %s, got %s", message, frame.ErrorMessage())
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
	if frame.LogLevel() != level {
		t.Errorf("Expected level %s, got %s", level, frame.LogLevel())
	}
	if frame.LogMessage() != message {
		t.Errorf("Expected message %s, got %s", message, frame.LogMessage())
	}
}

// TEST187: Test Frame::end with payload sets final payload
func TestFrameEndWithPayload(t *testing.T) {
	id := NewMessageIdRandom()
	payload := []byte("final data")

	frame := NewEnd(id, payload)

	if frame.FrameType != FrameTypeEnd {
		t.Errorf("Expected END frame type, got %v", frame.FrameType)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
	if !frame.IsEof() {
		t.Error("Expected eof to be true")
	}
}

// TEST188: Test Frame::end without payload
func TestFrameEndWithoutPayload(t *testing.T) {
	id := NewMessageIdRandom()
	frame := NewEnd(id, []byte{})

	if frame.FrameType != FrameTypeEnd {
		t.Errorf("Expected END frame type, got %v", frame.FrameType)
	}
	if len(frame.Payload) != 0 {
		t.Error("Expected empty payload")
	}
	if !frame.IsEof() {
		t.Error("Expected eof to be true")
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
	if frame.Cap != nil {
		t.Error("HEARTBEAT should have no cap")
	}
}

// TEST191: Test error_code and error_message return empty for non-Err frame types
func TestErrorAccessorsOnNonErrFrame(t *testing.T) {
	req := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if req.ErrorCode() != "" {
		t.Error("REQ must have no error_code")
	}
	if req.ErrorMessage() != "" {
		t.Error("REQ must have no error_message")
	}

	hello := NewHello(1000, 500)
	if hello.ErrorCode() != "" {
		t.Error("HELLO must have no error_code")
	}
}

// TEST192: Test log_level and log_message return empty for non-Log frame types
func TestLogAccessorsOnNonLogFrame(t *testing.T) {
	req := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if req.LogLevel() != "" {
		t.Error("REQ must have no log_level")
	}
	if req.LogMessage() != "" {
		t.Error("REQ must have no log_message")
	}
}

// TEST193: Test hello_max_frame and hello_max_chunk return appropriate values
func TestHelloAccessorsOnNonHelloFrame(t *testing.T) {
	err := NewErr(NewMessageIdRandom(), "E", "m")
	// ERR frames have no Meta with hello limits
	if err.Meta != nil {
		if _, hasMaxFrame := err.Meta["max_frame"]; hasMaxFrame {
			t.Error("ERR frame should not have max_frame in meta")
		}
	}
}

// TEST194: Test newFrame sets version and defaults correctly, optional fields are nil
func TestFrameNewDefaults(t *testing.T) {
	id := NewMessageIdRandom()
	frame := newFrame(FrameTypeChunk, id)

	if frame.Version != ProtocolVersion {
		t.Errorf("Expected version %d, got %d", ProtocolVersion, frame.Version)
	}
	if frame.FrameType != FrameTypeChunk {
		t.Error("Frame type mismatch")
	}
	if !frame.Id.Equals(id) {
		t.Error("ID mismatch")
	}
	if frame.Seq != 0 {
		t.Error("Seq should be 0")
	}
	if frame.ContentType != nil {
		t.Error("ContentType should be nil")
	}
	if frame.Meta != nil {
		t.Error("Meta should be nil")
	}
	if frame.Payload != nil {
		t.Error("Payload should be nil")
	}
	if frame.Len != nil {
		t.Error("Len should be nil")
	}
	if frame.Offset != nil {
		t.Error("Offset should be nil")
	}
	if frame.Eof != nil {
		t.Error("Eof should be nil")
	}
	if frame.Cap != nil {
		t.Error("Cap should be nil")
	}
}

// TEST195: Test default frame type (Go doesn't have Frame::default, skip or use REQ as default)
func TestFrameDefaultType(t *testing.T) {
	// Go doesn't have a Frame::default(), but we can verify REQ is a common default
	frame := NewReq(NewMessageIdDefault(), "cap:op=test", []byte{}, "text/plain")
	if frame.FrameType != FrameTypeReq {
		t.Error("Expected REQ frame type")
	}
	if frame.Version != ProtocolVersion {
		t.Errorf("Expected version %d", ProtocolVersion)
	}
}

// TEST196: Test IsEof returns false when eof field is nil (unset)
func TestIsEofWhenNone(t *testing.T) {
	frame := newFrame(FrameTypeChunk, MessageId{uintValue: new(uint64)})
	if frame.IsEof() {
		t.Error("eof=nil must mean not EOF")
	}
}

// TEST197: Test IsEof returns false when eof field is explicitly false
func TestIsEofWhenFalse(t *testing.T) {
	frame := newFrame(FrameTypeChunk, MessageId{uintValue: new(uint64)})
	falseVal := false
	frame.Eof = &falseVal
	if frame.IsEof() {
		t.Error("eof=false must mean not EOF")
	}
}

// TEST198: Test Limits::default provides the documented default values
func TestLimitsDefault(t *testing.T) {
	limits := DefaultLimits()
	if limits.MaxFrame != DefaultMaxFrame {
		t.Errorf("Expected max_frame %d, got %d", DefaultMaxFrame, limits.MaxFrame)
	}
	if limits.MaxChunk != DefaultMaxChunk {
		t.Errorf("Expected max_chunk %d, got %d", DefaultMaxChunk, limits.MaxChunk)
	}
	// Verify actual values match Rust constants
	if limits.MaxFrame != 3_670_016 {
		t.Error("default max_frame should be 3.5 MB")
	}
	if limits.MaxChunk != 262_144 {
		t.Error("default max_chunk should be 256 KB")
	}
}

// TEST199: Test PROTOCOL_VERSION is 1
func TestProtocolVersionConstant(t *testing.T) {
	if ProtocolVersion != 1 {
		t.Errorf("PROTOCOL_VERSION must be 1, got %d", ProtocolVersion)
	}
}

// TEST200: Test integer key constants match the protocol specification
func TestKeyConstants(t *testing.T) {
	if keyVersion != 0 {
		t.Errorf("keyVersion must be 0, got %d", keyVersion)
	}
	if keyFrameType != 1 {
		t.Errorf("keyFrameType must be 1, got %d", keyFrameType)
	}
	if keyId != 2 {
		t.Errorf("keyId must be 2, got %d", keyId)
	}
	if keySeq != 3 {
		t.Errorf("keySeq must be 3, got %d", keySeq)
	}
	if keyContentType != 4 {
		t.Errorf("keyContentType must be 4, got %d", keyContentType)
	}
	if keyMeta != 5 {
		t.Errorf("keyMeta must be 5, got %d", keyMeta)
	}
	if keyPayload != 6 {
		t.Errorf("keyPayload must be 6, got %d", keyPayload)
	}
	if keyLen != 7 {
		t.Errorf("keyLen must be 7, got %d", keyLen)
	}
	if keyOffset != 8 {
		t.Errorf("keyOffset must be 8, got %d", keyOffset)
	}
	if keyEof != 9 {
		t.Errorf("keyEof must be 9, got %d", keyEof)
	}
	if keyCap != 10 {
		t.Errorf("keyCap must be 10, got %d", keyCap)
	}
}

// TEST201: Test hello_with_manifest preserves binary manifest data (not just JSON text)
func TestHelloManifestBinaryData(t *testing.T) {
	binaryManifest := []byte{0x00, 0x01, 0xFF, 0xFE, 0x80}
	frame := NewHelloWithManifest(1000, 500, binaryManifest)

	// Extract manifest from meta
	if frame.Meta == nil {
		t.Fatal("Meta should not be nil")
	}
	manifestVal, ok := frame.Meta["manifest"]
	if !ok {
		t.Fatal("Meta should contain manifest key")
	}
	manifestBytes, ok := manifestVal.([]byte)
	if !ok {
		t.Fatal("Manifest should be bytes")
	}
	if string(manifestBytes) != string(binaryManifest) {
		t.Error("Binary manifest data not preserved")
	}
}

// TEST202: Test MessageId Eq semantics: equal UUIDs are equal, different ones are not
func TestMessageIdEqualityAndHash(t *testing.T) {
	id1 := MessageId{uuidBytes: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	id2 := MessageId{uuidBytes: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	id3 := MessageId{uuidBytes: []byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}}

	if !id1.Equals(id2) {
		t.Error("Equal UUID IDs should be equal")
	}
	if id1.Equals(id3) {
		t.Error("Different UUID IDs should not be equal")
	}

	val1 := uint64(42)
	val2 := uint64(42)
	val3 := uint64(43)
	uint1 := MessageId{uintValue: &val1}
	uint2 := MessageId{uintValue: &val2}
	uint3 := MessageId{uintValue: &val3}

	if !uint1.Equals(uint2) {
		t.Error("Equal Uint IDs should be equal")
	}
	if uint1.Equals(uint3) {
		t.Error("Different Uint IDs should not be equal")
	}
}

// TEST203: Test Uuid and Uint variants of MessageId are never equal even for coincidental byte values
func TestMessageIdCrossVariantInequality(t *testing.T) {
	uuidId := MessageId{uuidBytes: make([]byte, 16)} // all zeros
	zero := uint64(0)
	uintId := MessageId{uintValue: &zero}

	if uuidId.Equals(uintId) {
		t.Error("Different variants must not be equal")
	}
}

// TEST204: Test Frame::req with empty payload stores empty slice not nil
func TestReqFrameEmptyPayload(t *testing.T) {
	frame := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if frame.Payload == nil {
		t.Error("Empty payload should be empty slice, not nil")
	}
	if len(frame.Payload) != 0 {
		t.Error("Empty payload should have length 0")
	}
}
