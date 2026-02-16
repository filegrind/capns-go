package bifaci

import (
	"fmt"

	cborlib "github.com/fxamacker/cbor/v2"

	"github.com/filegrind/capns-go/cap"
	taggedurn "github.com/filegrind/tagged-urn-go"
)

// CollectArgsByMediaUrn collects all argument streams that match the given media URN pattern.
// Returns a slice of CBOR-decoded values (one per stream).
//
// This is a convenience helper for handlers that expect arguments of a specific type.
// For full streaming control, handlers should process frames directly.
func CollectArgsByMediaUrn(frames <-chan Frame, mediaUrnPattern string) ([]interface{}, error) {
	pattern, err := taggedurn.NewTaggedUrnFromString(mediaUrnPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid media URN pattern '%s': %w", mediaUrnPattern, err)
	}

	var results []interface{}
	var currentStreamID string
	var currentChunks [][]byte
	var currentMediaUrn string

	for frame := range frames {
		switch frame.FrameType {
		case FrameTypeStreamStart:
			if frame.StreamId == nil || frame.MediaUrn == nil {
				continue
			}
			currentStreamID = *frame.StreamId
			currentMediaUrn = *frame.MediaUrn
			currentChunks = [][]byte{}

		case FrameTypeChunk:
			if frame.StreamId == nil || *frame.StreamId != currentStreamID {
				continue
			}
			if frame.Payload != nil {
				currentChunks = append(currentChunks, frame.Payload)
			}

		case FrameTypeStreamEnd:
			if frame.StreamId == nil || *frame.StreamId != currentStreamID {
				continue
			}

			// Check if this stream matches the pattern
			streamUrn, parseErr := taggedurn.NewTaggedUrnFromString(currentMediaUrn)
			if parseErr != nil {
				continue
			}

			accepts, _ := pattern.Accepts(streamUrn)
			if !accepts {
				// Try reverse direction
				conforms, _ := streamUrn.ConformsTo(pattern)
				if !conforms {
					currentStreamID = ""
					continue
				}
			}

			// Concatenate chunks and decode
			var fullData []byte
			for _, chunk := range currentChunks {
				fullData = append(fullData, chunk...)
			}

			// Decode CBOR value
			var value interface{}
			if len(fullData) > 0 {
				if err := cborlib.Unmarshal(fullData, &value); err != nil {
					return nil, fmt.Errorf("failed to decode CBOR value from stream %s: %w", currentStreamID, err)
				}
			}

			results = append(results, value)
			currentStreamID = ""

		case FrameTypeEnd:
			// End of all streams - return what we collected
			return results, nil

		case FrameTypeErr:
			code := frame.ErrorCode()
			message := frame.ErrorMessage()
			return nil, fmt.Errorf("[%s] %s", code, message)
		}
	}

	return results, nil
}

// CollectFirstArg collects the first argument stream regardless of media URN.
// Returns raw bytes (concatenated chunks from first stream).
//
// This is useful for handlers that accept a single argument of any type.
func CollectFirstArg(frames <-chan Frame) ([]byte, error) {
	var firstStreamID string
	var chunks [][]byte
	foundFirst := false

	for frame := range frames {
		switch frame.FrameType {
		case FrameTypeStreamStart:
			if !foundFirst && frame.StreamId != nil {
				firstStreamID = *frame.StreamId
				foundFirst = true
				chunks = [][]byte{}
			}

		case FrameTypeChunk:
			if foundFirst && frame.StreamId != nil && *frame.StreamId == firstStreamID {
				if frame.Payload != nil {
					chunks = append(chunks, frame.Payload)
				}
			}

		case FrameTypeStreamEnd:
			if foundFirst && frame.StreamId != nil && *frame.StreamId == firstStreamID {
				// Concatenate all chunks
				var fullData []byte
				for _, chunk := range chunks {
					fullData = append(fullData, chunk...)
				}
				// Drain remaining frames until END
				for frame := range frames {
					if frame.FrameType == FrameTypeEnd {
						return fullData, nil
					} else if frame.FrameType == FrameTypeErr {
						code := frame.ErrorCode()
						message := frame.ErrorMessage()
						return nil, fmt.Errorf("[%s] %s", code, message)
					}
				}
				return fullData, nil
			}

		case FrameTypeEnd:
			// End reached before first stream ended
			var fullData []byte
			for _, chunk := range chunks {
				fullData = append(fullData, chunk...)
			}
			return fullData, nil

		case FrameTypeErr:
			code := frame.ErrorCode()
			message := frame.ErrorMessage()
			return nil, fmt.Errorf("[%s] %s", code, message)
		}
	}

	return nil, fmt.Errorf("unexpected end of frame stream")
}

// CollectPeerResponse collects all frames from a peer invocation response.
// Returns a map of stream_id → concatenated bytes.
//
// This is useful for handlers that invoke peer caps and need the complete response.
func CollectPeerResponse(frames <-chan Frame) (map[string][]byte, error) {
	streams := make(map[string][]byte)
	activeStreams := make(map[string][][]byte) // stream_id → chunks

	for frame := range frames {
		switch frame.FrameType {
		case FrameTypeStreamStart:
			if frame.StreamId != nil {
				activeStreams[*frame.StreamId] = [][]byte{}
			}

		case FrameTypeChunk:
			if frame.StreamId != nil {
				if chunks, exists := activeStreams[*frame.StreamId]; exists {
					if frame.Payload != nil {
						activeStreams[*frame.StreamId] = append(chunks, frame.Payload)
					}
				}
			}

		case FrameTypeStreamEnd:
			if frame.StreamId != nil {
				if chunks, exists := activeStreams[*frame.StreamId]; exists {
					// Concatenate chunks
					var fullData []byte
					for _, chunk := range chunks {
						fullData = append(fullData, chunk...)
					}
					streams[*frame.StreamId] = fullData
					delete(activeStreams, *frame.StreamId)
				}
			}

		case FrameTypeEnd:
			// Response complete
			return streams, nil

		case FrameTypeErr:
			code := frame.ErrorCode()
			message := frame.ErrorMessage()
			return nil, fmt.Errorf("[%s] %s", code, message)
		}
	}

	return streams, nil
}

// CollectAllArgs collects all argument streams in order.
// Returns a slice of {media_urn, data} pairs.
//
// This is useful for handlers that need to inspect all arguments.
func CollectAllArgs(frames <-chan Frame) ([]cap.CapArgumentValue, error) {
	var results []cap.CapArgumentValue
	var currentStreamID string
	var currentChunks [][]byte
	var currentMediaUrn string

	for frame := range frames {
		switch frame.FrameType {
		case FrameTypeStreamStart:
			if frame.StreamId == nil || frame.MediaUrn == nil {
				continue
			}
			currentStreamID = *frame.StreamId
			currentMediaUrn = *frame.MediaUrn
			currentChunks = [][]byte{}

		case FrameTypeChunk:
			if frame.StreamId == nil || *frame.StreamId != currentStreamID {
				continue
			}
			if frame.Payload != nil {
				currentChunks = append(currentChunks, frame.Payload)
			}

		case FrameTypeStreamEnd:
			if frame.StreamId == nil || *frame.StreamId != currentStreamID {
				continue
			}

			// Concatenate chunks
			var fullData []byte
			for _, chunk := range currentChunks {
				fullData = append(fullData, chunk...)
			}

			results = append(results, cap.CapArgumentValue{
				MediaUrn: currentMediaUrn,
				Value:    fullData,
			})
			currentStreamID = ""

		case FrameTypeEnd:
			// End of all streams
			return results, nil

		case FrameTypeErr:
			code := frame.ErrorCode()
			message := frame.ErrorMessage()
			return nil, fmt.Errorf("[%s] %s", code, message)
		}
	}

	return results, nil
}
