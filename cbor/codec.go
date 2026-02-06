package cbor

import (
	"errors"

	"github.com/fxamacker/cbor/v2"
)

// EncodeFrame encodes a Frame to CBOR bytes
func EncodeFrame(frame *Frame) ([]byte, error) {
	// Build CBOR map matching Rust/Python/Swift layout
	m := make(map[string]interface{})

	// Always include frame_type and id
	m["frame_type"] = uint8(frame.FrameType)

	// Encode ID
	if frame.Id.IsUuid() {
		m["id"] = frame.Id.uuidBytes
	} else {
		m["id"] = *frame.Id.uintValue
	}

	// Frame-specific fields
	switch frame.FrameType {
	case FrameTypeReq:
		m["cap"] = frame.Cap
		if frame.Payload != nil {
			m["payload"] = frame.Payload
		}
		if frame.ContentType != "" {
			m["content_type"] = frame.ContentType
		}

	case FrameTypeRes, FrameTypeEnd:
		if frame.Payload != nil {
			m["payload"] = frame.Payload
		}
		if frame.ContentType != "" {
			m["content_type"] = frame.ContentType
		}

	case FrameTypeChunk:
		m["seq"] = frame.Seq
		if frame.Payload != nil {
			m["payload"] = frame.Payload
		}

	case FrameTypeErr:
		m["code"] = frame.Code
		m["message"] = frame.Message

	case FrameTypeLog:
		m["level"] = frame.Level
		m["message"] = frame.Message

	case FrameTypeHeartbeat:
		// No additional fields

	case FrameTypeHello:
		if frame.Payload != nil {
			m["payload"] = frame.Payload
		}
	}

	return cbor.Marshal(m)
}

// DecodeFrame decodes CBOR bytes to a Frame
func DecodeFrame(data []byte) (*Frame, error) {
	var m map[string]interface{}
	if err := cbor.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	frame := &Frame{}

	// Extract frame_type
	ftVal, ok := m["frame_type"]
	if !ok {
		return nil, errors.New("missing frame_type")
	}
	ft, ok := ftVal.(uint64)
	if !ok {
		return nil, errors.New("frame_type must be uint")
	}
	frame.FrameType = FrameType(ft)

	// Extract id (can be bytes for UUID or uint for uint64)
	idVal, ok := m["id"]
	if !ok {
		return nil, errors.New("missing id")
	}

	switch v := idVal.(type) {
	case []byte:
		// UUID variant
		if len(v) != 16 {
			return nil, errors.New("UUID id must be 16 bytes")
		}
		frame.Id = MessageId{uuidBytes: v}
	case uint64:
		// uint variant
		frame.Id = NewMessageIdFromUint(v)
	default:
		return nil, errors.New("id must be bytes or uint")
	}

	// Extract frame-specific fields
	switch frame.FrameType {
	case FrameTypeReq:
		if cap, ok := m["cap"].(string); ok {
			frame.Cap = cap
		} else {
			return nil, errors.New("REQ frame requires cap string")
		}
		if payload, ok := m["payload"].([]byte); ok {
			frame.Payload = payload
		}
		if contentType, ok := m["content_type"].(string); ok {
			frame.ContentType = contentType
		}

	case FrameTypeRes, FrameTypeEnd:
		if payload, ok := m["payload"].([]byte); ok {
			frame.Payload = payload
		}
		if contentType, ok := m["content_type"].(string); ok {
			frame.ContentType = contentType
		}

	case FrameTypeChunk:
		if seq, ok := m["seq"].(uint64); ok {
			frame.Seq = seq
		} else {
			return nil, errors.New("CHUNK frame requires seq uint")
		}
		if payload, ok := m["payload"].([]byte); ok {
			frame.Payload = payload
		}

	case FrameTypeErr:
		if code, ok := m["code"].(string); ok {
			frame.Code = code
		} else {
			return nil, errors.New("ERR frame requires code string")
		}
		if message, ok := m["message"].(string); ok {
			frame.Message = message
		} else {
			return nil, errors.New("ERR frame requires message string")
		}

	case FrameTypeLog:
		if level, ok := m["level"].(string); ok {
			frame.Level = level
		} else {
			return nil, errors.New("LOG frame requires level string")
		}
		if message, ok := m["message"].(string); ok {
			frame.Message = message
		} else {
			return nil, errors.New("LOG frame requires message string")
		}

	case FrameTypeHeartbeat:
		// No additional fields to extract

	case FrameTypeHello:
		if payload, ok := m["payload"].([]byte); ok {
			frame.Payload = payload
		}
	}

	return frame, nil
}
