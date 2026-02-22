package bifaci

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TEST171: Test all FrameType discriminants roundtrip through uint8 conversion preserving identity
func Test171_frame_type_roundtrip(t *testing.T) {
	types := []FrameType{
		FrameTypeReq,
		// Res REMOVED - old protocol no longer supported
		FrameTypeChunk,
		FrameTypeEnd,
		FrameTypeErr,
		FrameTypeLog,
		FrameTypeHeartbeat,
		FrameTypeHello,
		FrameTypeStreamStart,
		FrameTypeStreamEnd,
	}

	for _, ft := range types {
		asUint := uint8(ft)
		backToType := FrameType(asUint)
		if backToType != ft {
			t.Errorf("FrameType %v roundtrip failed: got %v", ft, backToType)
		}
	}
}

// TEST172: Test FrameType::from_u8 returns invalid for values outside the valid discriminant range
func Test172_frame_type_valid_range(t *testing.T) {
	validTypes := map[uint8]bool{
		0:  true,  // HELLO
		1:  true,  // REQ
		2:  false, // RES REMOVED - old protocol no longer supported
		3:  true,  // CHUNK
		4:  true,  // END
		5:  true,  // LOG
		6:  true,  // ERR
		7:  true,  // HEARTBEAT
		8:  true,  // STREAM_START
		9:  true,  // STREAM_END
		10: true,  // RELAY_NOTIFY
		11: true,  // RELAY_STATE
	}

	for i := uint8(0); i <= 11; i++ {
		if expected, exists := validTypes[i]; exists && expected {
			ft := FrameType(i)
			if ft.String() == fmt.Sprintf("UNKNOWN(%d)", i) {
				t.Errorf("Expected %d to be a valid FrameType", i)
			}
		}
	}
	// 12 is one past RelayState — must be invalid
	ft12 := FrameType(12)
	if ft12.String() != "UNKNOWN(12)" {
		t.Errorf("Expected 12 to be invalid, got %s", ft12.String())
	}
}

// TEST173: Test FrameType discriminant values match the wire protocol specification exactly
func Test173_frame_type_wire_protocol_values(t *testing.T) {
	if uint8(FrameTypeHello) != 0 {
		t.Errorf("HELLO must be 0, got %d", FrameTypeHello)
	}
	if uint8(FrameTypeReq) != 1 {
		t.Errorf("REQ must be 1, got %d", FrameTypeReq)
	}
	// Res = 2 REMOVED - old protocol no longer supported
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
	if uint8(FrameTypeStreamStart) != 8 {
		t.Errorf("STREAM_START must be 8, got %d", FrameTypeStreamStart)
	}
	if uint8(FrameTypeStreamEnd) != 9 {
		t.Errorf("STREAM_END must be 9, got %d", FrameTypeStreamEnd)
	}
}

// TEST174: Test MessageId::new_uuid generates valid UUID that roundtrips through string conversion
func Test174_message_id_new_uuid_roundtrip(t *testing.T) {
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
func Test175_message_id_uuid_distinct(t *testing.T) {
	id1 := NewMessageIdRandom()
	id2 := NewMessageIdRandom()

	if id1.Equals(id2) {
		t.Error("Two random UUIDs should not be equal")
	}
}

// TEST176: Test MessageId::Uint does not produce a UUID string, to_uuid_string returns empty
func Test176_message_id_uint_no_uuid_string(t *testing.T) {
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
func Test177_message_id_from_uuid_invalid_bytes(t *testing.T) {
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
func Test178_message_id_as_bytes(t *testing.T) {
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
func Test179_message_id_default(t *testing.T) {
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
func Test180_frame_hello_without_manifest(t *testing.T) {
	frame := NewHello(DefaultMaxFrame, DefaultMaxChunk, DefaultMaxReorderBuffer)
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
func Test181_frame_hello_with_manifest(t *testing.T) {
	manifest := []byte(`{"name":"test"}`)
	frame := NewHelloWithManifest(DefaultMaxFrame, DefaultMaxChunk, DefaultMaxReorderBuffer, manifest)
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
func Test182_frame_req(t *testing.T) {
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

// TEST183: REMOVED - RES frame no longer supported in protocol v2

// TEST184: Test Frame::chunk stores seq and payload for streaming (updated for stream_id)
func Test184_frame_chunk(t *testing.T) {
	id := NewMessageIdRandom()
	streamId := "stream-123"
	seq := uint64(5)
	payload := []byte("chunk data")
	chunkIndex := uint64(0)
	checksum := ComputeChecksum(payload)

	frame := NewChunk(id, streamId, seq, payload, chunkIndex, checksum)

	if frame.FrameType != FrameTypeChunk {
		t.Errorf("Expected CHUNK frame type, got %v", frame.FrameType)
	}
	if frame.StreamId == nil || *frame.StreamId != streamId {
		t.Errorf("Expected streamId %s, got %v", streamId, frame.StreamId)
	}
	if frame.Seq != seq {
		t.Errorf("Expected seq %d, got %d", seq, frame.Seq)
	}
	if string(frame.Payload) != string(payload) {
		t.Error("Payload mismatch")
	}
}

// TEST185: Test Frame::err stores error code and message
func Test185_frame_err(t *testing.T) {
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
func Test186_frame_log(t *testing.T) {
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
func Test187_frame_end_with_payload(t *testing.T) {
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
func Test188_frame_end_without_payload(t *testing.T) {
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
func Test189_frame_chunk_with_offset(t *testing.T) {
	// This test documents expected behavior for offset/len fields
	// Currently not implemented in Go version
	t.Skip("Offset/len fields not yet implemented in Go version")
}

// TEST190: Test Frame::heartbeat creates minimal frame with no payload or metadata
func Test190_frame_heartbeat(t *testing.T) {
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
func Test191_error_accessors_on_non_err_frame(t *testing.T) {
	req := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if req.ErrorCode() != "" {
		t.Error("REQ must have no error_code")
	}
	if req.ErrorMessage() != "" {
		t.Error("REQ must have no error_message")
	}

	hello := NewHello(1000, 500, DefaultMaxReorderBuffer)
	if hello.ErrorCode() != "" {
		t.Error("HELLO must have no error_code")
	}
}

// TEST192: Test log_level and log_message return empty for non-Log frame types
func Test192_log_accessors_on_non_log_frame(t *testing.T) {
	req := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if req.LogLevel() != "" {
		t.Error("REQ must have no log_level")
	}
	if req.LogMessage() != "" {
		t.Error("REQ must have no log_message")
	}
}

// TEST193: Test hello_max_frame and hello_max_chunk return appropriate values
func Test193_hello_accessors_on_non_hello_frame(t *testing.T) {
	err := NewErr(NewMessageIdRandom(), "E", "m")
	// ERR frames have no Meta with hello limits
	if err.Meta != nil {
		if _, hasMaxFrame := err.Meta["max_frame"]; hasMaxFrame {
			t.Error("ERR frame should not have max_frame in meta")
		}
	}
}

// TEST194: Test newFrame sets version and defaults correctly, optional fields are nil
func Test194_frame_new_defaults(t *testing.T) {
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
		t.Error("cap.Cap should be nil")
	}
}

// TEST195: Test default frame type (Go doesn't have Frame::default, skip or use REQ as default)
func Test195_frame_default_type(t *testing.T) {
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
func Test196_is_eof_when_none(t *testing.T) {
	frame := newFrame(FrameTypeChunk, MessageId{uintValue: new(uint64)})
	if frame.IsEof() {
		t.Error("eof=nil must mean not EOF")
	}
}

// TEST197: Test IsEof returns false when eof field is explicitly false
func Test197_is_eof_when_false(t *testing.T) {
	frame := newFrame(FrameTypeChunk, MessageId{uintValue: new(uint64)})
	falseVal := false
	frame.Eof = &falseVal
	if frame.IsEof() {
		t.Error("eof=false must mean not EOF")
	}
}

// TEST198: Test Limits::default provides the documented default values
func Test198_limits_default(t *testing.T) {
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

// TEST199: Test PROTOCOL_VERSION is 2
func Test199_protocol_version_constant(t *testing.T) {
	if ProtocolVersion != 2 {
		t.Errorf("PROTOCOL_VERSION must be 2, got %d", ProtocolVersion)
	}
}

// TEST200: Test integer key constants match the protocol specification
func Test200_key_constants(t *testing.T) {
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
func Test201_hello_manifest_binary_data(t *testing.T) {
	binaryManifest := []byte{0x00, 0x01, 0xFF, 0xFE, 0x80}
	frame := NewHelloWithManifest(1000, 500, DefaultMaxReorderBuffer, binaryManifest)

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
func Test202_message_id_equality_and_hash(t *testing.T) {
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
func Test203_message_id_cross_variant_inequality(t *testing.T) {
	uuidId := MessageId{uuidBytes: make([]byte, 16)} // all zeros
	zero := uint64(0)
	uintId := MessageId{uintValue: &zero}

	if uuidId.Equals(uintId) {
		t.Error("Different variants must not be equal")
	}
}

// TEST204: Test Frame::req with empty payload stores empty slice not nil
func Test204_req_frame_empty_payload(t *testing.T) {
	frame := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if frame.Payload == nil {
		t.Error("Empty payload should be empty slice, not nil")
	}
	if len(frame.Payload) != 0 {
		t.Error("Empty payload should have length 0")
	}
}

// TEST365: Frame::stream_start stores reqId, streamId, mediaUrn
func Test365_stream_start_frame(t *testing.T) {
	reqId := NewMessageIdRandom()
	streamId := "stream-abc-123"
	mediaUrn := "media:"

	frame := NewStreamStart(reqId, streamId, mediaUrn)

	if frame.FrameType != FrameTypeStreamStart {
		t.Errorf("Expected STREAM_START frame type, got %v", frame.FrameType)
	}
	if frame.StreamId == nil || *frame.StreamId != streamId {
		t.Errorf("Expected streamId %s, got %v", streamId, frame.StreamId)
	}
	if frame.MediaUrn == nil || *frame.MediaUrn != mediaUrn {
		t.Errorf("Expected mediaUrn %s, got %v", mediaUrn, frame.MediaUrn)
	}
	if !frame.Id.Equals(reqId) {
		t.Error("Request ID mismatch")
	}
}

// TEST366: Frame::stream_end stores reqId, streamId
func Test366_stream_end_frame(t *testing.T) {
	reqId := NewMessageIdRandom()
	streamId := "stream-xyz-456"

	frame := NewStreamEnd(reqId, streamId, 0)

	if frame.FrameType != FrameTypeStreamEnd {
		t.Errorf("Expected STREAM_END frame type, got %v", frame.FrameType)
	}
	if frame.StreamId == nil || *frame.StreamId != streamId {
		t.Errorf("Expected streamId %s, got %v", streamId, frame.StreamId)
	}
	if frame.MediaUrn != nil {
		t.Errorf("STREAM_END should not have mediaUrn, got %v", frame.MediaUrn)
	}
	if !frame.Id.Equals(reqId) {
		t.Error("Request ID mismatch")
	}
}

// TEST367: Frame::stream_start with empty streamId still constructs
func Test367_stream_start_with_empty_stream_id(t *testing.T) {
	reqId := NewMessageIdRandom()
	streamId := ""
	mediaUrn := "media:json"

	frame := NewStreamStart(reqId, streamId, mediaUrn)

	if frame.FrameType != FrameTypeStreamStart {
		t.Errorf("Expected STREAM_START frame type, got %v", frame.FrameType)
	}
	if frame.StreamId == nil {
		t.Error("StreamId should not be nil, even if empty")
	}
	if frame.MediaUrn == nil || *frame.MediaUrn != mediaUrn {
		t.Errorf("Expected mediaUrn %s, got %v", mediaUrn, frame.MediaUrn)
	}
}

// TEST368: Frame::stream_start with empty mediaUrn still constructs
func Test368_stream_start_with_empty_media_urn(t *testing.T) {
	reqId := NewMessageIdRandom()
	streamId := "stream-test"
	mediaUrn := ""

	frame := NewStreamStart(reqId, streamId, mediaUrn)

	if frame.FrameType != FrameTypeStreamStart {
		t.Errorf("Expected STREAM_START frame type, got %v", frame.FrameType)
	}
	if frame.StreamId == nil || *frame.StreamId != streamId {
		t.Errorf("Expected streamId %s, got %v", streamId, frame.StreamId)
	}
	if frame.MediaUrn == nil {
		t.Error("MediaUrn should not be nil, even if empty")
	}
}

// TEST399: RelayNotify discriminant roundtrips through uint8 conversion (value 10)
func Test399_relay_notify_discriminant_roundtrip(t *testing.T) {
	ft := FrameTypeRelayNotify
	asUint := uint8(ft)
	if asUint != 10 {
		t.Errorf("RELAY_NOTIFY must be 10, got %d", asUint)
	}
	backToType := FrameType(asUint)
	if backToType != FrameTypeRelayNotify {
		t.Errorf("FrameType(10) must be RELAY_NOTIFY, got %v", backToType)
	}
}

// TEST400: RelayState discriminant roundtrips through uint8 conversion (value 11)
func Test400_relay_state_discriminant_roundtrip(t *testing.T) {
	ft := FrameTypeRelayState
	asUint := uint8(ft)
	if asUint != 11 {
		t.Errorf("RELAY_STATE must be 11, got %d", asUint)
	}
	backToType := FrameType(asUint)
	if backToType != FrameTypeRelayState {
		t.Errorf("FrameType(11) must be RELAY_STATE, got %v", backToType)
	}
}

// TEST401: relay_notify factory stores manifest and limits, accessors extract them correctly
func Test401_relay_notify_factory_and_accessors(t *testing.T) {
	manifest := []byte(`{"caps":["cap:op=test"]}`)
	maxFrame := 2_000_000
	maxChunk := 128_000

	frame := NewRelayNotify(manifest, maxFrame, maxChunk, DefaultMaxReorderBuffer)

	if frame.FrameType != FrameTypeRelayNotify {
		t.Errorf("Expected RELAY_NOTIFY, got %v", frame.FrameType)
	}

	// Test manifest accessor
	extractedManifest := frame.RelayNotifyManifest()
	if extractedManifest == nil {
		t.Fatal("RelayNotifyManifest() returned nil")
	}
	if string(extractedManifest) != string(manifest) {
		t.Errorf("Manifest mismatch: got %s", string(extractedManifest))
	}

	// Test limits accessor
	extractedLimits := frame.RelayNotifyLimits()
	if extractedLimits == nil {
		t.Fatal("RelayNotifyLimits() returned nil")
	}
	if extractedLimits.MaxFrame != maxFrame {
		t.Errorf("MaxFrame mismatch: expected %d, got %d", maxFrame, extractedLimits.MaxFrame)
	}
	if extractedLimits.MaxChunk != maxChunk {
		t.Errorf("MaxChunk mismatch: expected %d, got %d", maxChunk, extractedLimits.MaxChunk)
	}

	// Test accessors on wrong frame type return nil
	req := NewReq(NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
	if req.RelayNotifyManifest() != nil {
		t.Error("RelayNotifyManifest on REQ must return nil")
	}
	if req.RelayNotifyLimits() != nil {
		t.Error("RelayNotifyLimits on REQ must return nil")
	}
}

// TEST402: relay_state factory stores resource payload in Payload field
func Test402_relay_state_factory_and_payload(t *testing.T) {
	resources := []byte(`{"gpu_memory":8192}`)

	frame := NewRelayState(resources)

	if frame.FrameType != FrameTypeRelayState {
		t.Errorf("Expected RELAY_STATE, got %v", frame.FrameType)
	}
	if string(frame.Payload) != string(resources) {
		t.Errorf("Payload mismatch: got %s", string(frame.Payload))
	}
}

// TEST403: FrameType from value 12 is invalid (one past RelayState)
func Test403_frame_type_one_past_relay_state(t *testing.T) {
	ft := FrameType(12)
	if ft.String() != fmt.Sprintf("UNKNOWN(%d)", 12) {
		t.Errorf("FrameType(12) must be unknown, got %s", ft.String())
	}
}

// TEST667: VerifyChunkChecksum detects corrupted payload
func Test667_verify_chunk_checksum_detects_corruption(t *testing.T) {
	id := NewMessageIdRandom()
	streamId := "stream-test"
	payload := []byte("original payload data")
	checksum := ComputeChecksum(payload)

	// Create valid chunk frame
	frame := NewChunk(id, streamId, 0, payload, 0, checksum)

	// Valid frame should pass verification
	if err := VerifyChunkChecksum(frame); err != nil {
		t.Errorf("Valid frame should pass verification: %v", err)
	}

	// Corrupt the payload (simulate transmission error)
	frame.Payload = []byte("corrupted payload!!")

	// Corrupted frame should fail verification
	err := VerifyChunkChecksum(frame)
	if err == nil {
		t.Error("Corrupted frame should fail verification")
	}
	if err != nil && !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("Error should mention checksum mismatch, got: %v", err)
	}

	// Missing checksum should fail
	frame.Checksum = nil
	err = VerifyChunkChecksum(frame)
	if err == nil {
		t.Error("Frame without checksum should fail verification")
	}
	if err != nil && !strings.Contains(err.Error(), "missing") {
		t.Errorf("Error should mention missing checksum, got: %v", err)
	}
}
