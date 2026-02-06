package cbor

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Protocol version. Always 1 for this implementation.
const ProtocolVersion uint8 = 1

// Default maximum frame size (3.5 MB) - safe margin below 3.75MB limit
// Larger payloads automatically use CHUNK frames
const DefaultMaxFrame int = 3_670_016

// Default maximum chunk size (256 KB)
const DefaultMaxChunk int = 262_144

// Hard limit on frame size (16 MB) - prevents DoS
const MaxFrameHardLimit int = 16_777_216

// FrameType represents the type of CBOR frame
type FrameType uint8

const (
	FrameTypeHello     FrameType = 0 // MUST be 0 - matches Rust
	FrameTypeReq       FrameType = 1
	FrameTypeRes       FrameType = 2
	FrameTypeChunk     FrameType = 3
	FrameTypeEnd       FrameType = 4
	FrameTypeLog       FrameType = 5 // MUST be 5 - matches Rust
	FrameTypeErr       FrameType = 6 // MUST be 6 - matches Rust
	FrameTypeHeartbeat FrameType = 7
)

// String returns the frame type name
func (ft FrameType) String() string {
	switch ft {
	case FrameTypeReq:
		return "REQ"
	case FrameTypeRes:
		return "RES"
	case FrameTypeChunk:
		return "CHUNK"
	case FrameTypeEnd:
		return "END"
	case FrameTypeErr:
		return "ERR"
	case FrameTypeLog:
		return "LOG"
	case FrameTypeHeartbeat:
		return "HEARTBEAT"
	case FrameTypeHello:
		return "HELLO"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", ft)
	}
}

// MessageId represents a unique message identifier (either UUID or uint64)
type MessageId struct {
	uuidBytes []byte  // 16 bytes for UUID variant
	uintValue *uint64 // For uint variant
}

// NewMessageIdFromUuid creates a MessageId from UUID bytes
func NewMessageIdFromUuid(uuidBytes []byte) (MessageId, error) {
	if len(uuidBytes) != 16 {
		return MessageId{}, errors.New("UUID must be exactly 16 bytes")
	}
	return MessageId{uuidBytes: uuidBytes}, nil
}

// NewMessageIdFromUint creates a MessageId from a uint64
func NewMessageIdFromUint(value uint64) MessageId {
	return MessageId{uintValue: &value}
}

// NewMessageIdRandom creates a random UUID-based MessageId
func NewMessageIdRandom() MessageId {
	id := uuid.New()
	bytes, _ := id.MarshalBinary()
	return MessageId{uuidBytes: bytes}
}

// NewMessageIdDefault creates a default MessageId (uint 0)
func NewMessageIdDefault() MessageId {
	zero := uint64(0)
	return MessageId{uintValue: &zero}
}

// IsUuid returns true if this is a UUID-based ID
func (m MessageId) IsUuid() bool {
	return m.uuidBytes != nil
}

// ToUuidString returns UUID string representation (empty if uint variant)
func (m MessageId) ToUuidString() string {
	if m.uuidBytes != nil {
		id, err := uuid.FromBytes(m.uuidBytes)
		if err == nil {
			return id.String()
		}
	}
	return ""
}

// ToString returns string representation for both UUID and uint variants
func (m MessageId) ToString() string {
	if m.uuidBytes != nil {
		return m.ToUuidString()
	}
	if m.uintValue != nil {
		return fmt.Sprintf("%d", *m.uintValue)
	}
	return "0"
}

// AsBytes returns bytes for comparison
func (m MessageId) AsBytes() []byte {
	if m.uuidBytes != nil {
		return m.uuidBytes
	}
	if m.uintValue != nil {
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, *m.uintValue)
		return buf
	}
	return make([]byte, 8)
}

// Equals checks if two MessageIds are equal
func (m MessageId) Equals(other MessageId) bool {
	// Both UUID
	if m.uuidBytes != nil && other.uuidBytes != nil {
		return string(m.uuidBytes) == string(other.uuidBytes)
	}
	// Both uint
	if m.uintValue != nil && other.uintValue != nil {
		return *m.uintValue == *other.uintValue
	}
	// Different types
	return false
}

// Frame represents a CBOR protocol frame
// This structure MUST match the Rust Frame structure exactly
type Frame struct {
	Version     uint8                  // Protocol version (always 1)
	FrameType   FrameType              // Frame type discriminator
	Id          MessageId              // Message ID for correlation
	Seq         uint64                 // Sequence number within a stream
	ContentType *string                // Content type of payload (MIME-like)
	Meta        map[string]interface{} // Metadata map (for ERR/LOG data, HELLO limits, etc.)
	Payload     []byte                 // Binary payload
	Len         *uint64                // Total length for chunked transfers (first chunk only)
	Offset      *uint64                // Byte offset in chunked stream
	Eof         *bool                  // End of stream marker
	Cap         *string                // Cap URN (for REQ frames)
}

// New creates a new frame with required fields (matches Rust Frame::new)
func newFrame(frameType FrameType, id MessageId) *Frame {
	return &Frame{
		Version:   ProtocolVersion,
		FrameType: frameType,
		Id:        id,
		Seq:       0,
	}
}

// NewReq creates a REQ frame (matches Rust Frame::req)
func NewReq(id MessageId, capUrn string, payload []byte, contentType string) *Frame {
	frame := newFrame(FrameTypeReq, id)
	frame.Cap = &capUrn
	frame.Payload = payload
	frame.ContentType = &contentType
	return frame
}

// NewRes creates a RES frame (matches Rust Frame::res)
func NewRes(id MessageId, payload []byte, contentType string) *Frame {
	frame := newFrame(FrameTypeRes, id)
	frame.Payload = payload
	frame.ContentType = &contentType
	return frame
}

// NewChunk creates a CHUNK frame (matches Rust Frame::chunk)
func NewChunk(id MessageId, seq uint64, payload []byte) *Frame {
	frame := newFrame(FrameTypeChunk, id)
	frame.Seq = seq
	frame.Payload = payload
	return frame
}

// NewEnd creates an END frame (matches Rust Frame::end)
func NewEnd(id MessageId, payload []byte) *Frame {
	frame := newFrame(FrameTypeEnd, id)
	if payload != nil {
		frame.Payload = payload
	}
	eof := true
	frame.Eof = &eof
	return frame
}

// NewErr creates an ERR frame (matches Rust Frame::err)
// code and message are stored in the Meta map
func NewErr(id MessageId, code string, message string) *Frame {
	frame := newFrame(FrameTypeErr, id)
	frame.Meta = map[string]interface{}{
		"code":    code,
		"message": message,
	}
	return frame
}

// NewLog creates a LOG frame (matches Rust Frame::log)
// level and message are stored in the Meta map
func NewLog(id MessageId, level string, message string) *Frame {
	frame := newFrame(FrameTypeLog, id)
	frame.Meta = map[string]interface{}{
		"level":   level,
		"message": message,
	}
	return frame
}

// NewHeartbeat creates a HEARTBEAT frame (matches Rust Frame::heartbeat)
func NewHeartbeat(id MessageId) *Frame {
	return newFrame(FrameTypeHeartbeat, id)
}

// NewHello creates a HELLO frame for handshake (host side - no manifest)
// Matches Rust Frame::hello
func NewHello(maxFrame, maxChunk int) *Frame {
	frame := newFrame(FrameTypeHello, MessageId{uintValue: new(uint64)})
	frame.Meta = map[string]interface{}{
		"max_frame": maxFrame,
		"max_chunk": maxChunk,
		"version":   ProtocolVersion,
	}
	return frame
}

// NewHelloWithManifest creates a HELLO frame with manifest (plugin side)
// Matches Rust Frame::hello_with_manifest
func NewHelloWithManifest(maxFrame, maxChunk int, manifest []byte) *Frame {
	frame := newFrame(FrameTypeHello, MessageId{uintValue: new(uint64)})
	frame.Meta = map[string]interface{}{
		"max_frame": maxFrame,
		"max_chunk": maxChunk,
		"version":   ProtocolVersion,
		"manifest":  manifest,
	}
	return frame
}

// Helper methods to extract values from Meta map (matches Rust Frame::error_code, error_message, log_level, log_message)

// ErrorCode gets error code from ERR frame meta
func (f *Frame) ErrorCode() string {
	if f.FrameType != FrameTypeErr || f.Meta == nil {
		return ""
	}
	if code, ok := f.Meta["code"].(string); ok {
		return code
	}
	return ""
}

// ErrorMessage gets error message from ERR frame meta
func (f *Frame) ErrorMessage() string {
	if f.FrameType != FrameTypeErr || f.Meta == nil {
		return ""
	}
	if msg, ok := f.Meta["message"].(string); ok {
		return msg
	}
	return ""
}

// LogLevel gets log level from LOG frame meta
func (f *Frame) LogLevel() string {
	if f.FrameType != FrameTypeLog || f.Meta == nil {
		return ""
	}
	if level, ok := f.Meta["level"].(string); ok {
		return level
	}
	return ""
}

// LogMessage gets log message from LOG frame meta
func (f *Frame) LogMessage() string {
	if f.FrameType != FrameTypeLog || f.Meta == nil {
		return ""
	}
	if msg, ok := f.Meta["message"].(string); ok {
		return msg
	}
	return ""
}

// IsEof checks if this is the final frame in a stream (matches Rust Frame::is_eof)
func (f *Frame) IsEof() bool {
	return f.Eof != nil && *f.Eof
}
