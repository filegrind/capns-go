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
	FrameTypeReq       FrameType = 1
	FrameTypeRes       FrameType = 2
	FrameTypeChunk     FrameType = 3
	FrameTypeEnd       FrameType = 4
	FrameTypeErr       FrameType = 5
	FrameTypeLog       FrameType = 6
	FrameTypeHeartbeat FrameType = 7
	FrameTypeHello     FrameType = 8
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
type Frame struct {
	FrameType   FrameType
	Id          MessageId
	Cap         string  // For REQ frames
	Seq         uint64  // For CHUNK frames
	Payload     []byte  // Frame payload
	ContentType string  // For RES/END frames
	Code        string  // For ERR frames
	Message     string  // For ERR/LOG frames
	Level       string  // For LOG frames
}

// NewReq creates a REQ frame
func NewReq(id MessageId, cap string, payload []byte, contentType string) *Frame {
	return &Frame{
		FrameType:   FrameTypeReq,
		Id:          id,
		Cap:         cap,
		Payload:     payload,
		ContentType: contentType,
	}
}

// NewRes creates a RES frame
func NewRes(id MessageId, payload []byte, contentType string) *Frame {
	return &Frame{
		FrameType:   FrameTypeRes,
		Id:          id,
		Payload:     payload,
		ContentType: contentType,
	}
}

// NewChunk creates a CHUNK frame
func NewChunk(id MessageId, seq uint64, payload []byte) *Frame {
	return &Frame{
		FrameType: FrameTypeChunk,
		Id:        id,
		Seq:       seq,
		Payload:   payload,
	}
}

// NewEnd creates an END frame
func NewEnd(id MessageId, payload []byte, contentType string) *Frame {
	return &Frame{
		FrameType:   FrameTypeEnd,
		Id:          id,
		Payload:     payload,
		ContentType: contentType,
	}
}

// NewErr creates an ERR frame
func NewErr(id MessageId, code string, message string) *Frame {
	return &Frame{
		FrameType: FrameTypeErr,
		Id:        id,
		Code:      code,
		Message:   message,
	}
}

// NewLog creates a LOG frame
func NewLog(id MessageId, level string, message string) *Frame {
	return &Frame{
		FrameType: FrameTypeLog,
		Id:        id,
		Level:     level,
		Message:   message,
	}
}

// NewHeartbeat creates a HEARTBEAT frame
func NewHeartbeat(id MessageId) *Frame {
	return &Frame{
		FrameType: FrameTypeHeartbeat,
		Id:        id,
	}
}

// NewHello creates a HELLO frame with manifest payload
func NewHello(manifestPayload []byte) *Frame {
	return &Frame{
		FrameType: FrameTypeHello,
		Id:        NewMessageIdDefault(),
		Payload:   manifestPayload,
	}
}
