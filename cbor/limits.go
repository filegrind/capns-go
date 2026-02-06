package cbor

// Limits represents protocol negotiation limits
type Limits struct {
	MaxFrame int `cbor:"max_frame"`
	MaxChunk int `cbor:"max_chunk"`
}

// DefaultLimits returns the default protocol limits
func DefaultLimits() Limits {
	return Limits{
		MaxFrame: DefaultMaxFrame,
		MaxChunk: DefaultMaxChunk,
	}
}

// NegotiateLimits returns the minimum of two limit sets
func NegotiateLimits(a, b Limits) Limits {
	return Limits{
		MaxFrame: min(a.MaxFrame, b.MaxFrame),
		MaxChunk: min(a.MaxChunk, b.MaxChunk),
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
