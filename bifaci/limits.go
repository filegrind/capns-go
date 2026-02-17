package bifaci

// DefaultMaxReorderBuffer is the default reorder buffer size (64 slots)
const DefaultMaxReorderBuffer int = 64

// Limits represents protocol negotiation limits
type Limits struct {
	MaxFrame         int `cbor:"max_frame"`
	MaxChunk         int `cbor:"max_chunk"`
	MaxReorderBuffer int `cbor:"max_reorder_buffer"`
}

// DefaultLimits returns the default protocol limits
func DefaultLimits() Limits {
	return Limits{
		MaxFrame:         DefaultMaxFrame,
		MaxChunk:         DefaultMaxChunk,
		MaxReorderBuffer: DefaultMaxReorderBuffer,
	}
}

// NegotiateLimits returns the minimum of two limit sets
func NegotiateLimits(a, b Limits) Limits {
	return Limits{
		MaxFrame:         min(a.MaxFrame, b.MaxFrame),
		MaxChunk:         min(a.MaxChunk, b.MaxChunk),
		MaxReorderBuffer: min(a.MaxReorderBuffer, b.MaxReorderBuffer),
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
