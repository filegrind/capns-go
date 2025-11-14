package capdef

import (
	"strings"
)

// CapabilityKeyBuilder provides a fluent builder interface for constructing and manipulating capability identifiers.
// This replaces manual creation and manipulation of capability IDs with a type-safe API.
type CapabilityKeyBuilder struct {
	segments []string
}

// NewCapabilityKeyBuilder creates a new empty builder
func NewCapabilityKeyBuilder() *CapabilityKeyBuilder {
	return &CapabilityKeyBuilder{
		segments: make([]string, 0),
	}
}

// NewCapabilityKeyBuilderFromCapabilityKey creates a builder starting with a base capability ID
func NewCapabilityKeyBuilderFromCapabilityKey(capabilityKey *CapabilityKey) *CapabilityKeyBuilder {
	segments := make([]string, len(capabilityKey.segments))
	copy(segments, capabilityKey.segments)
	return &CapabilityKeyBuilder{
		segments: segments,
	}
}

// NewCapabilityKeyBuilderFromString creates a builder from a capability string
func NewCapabilityKeyBuilderFromString(s string) (*CapabilityKeyBuilder, error) {
	capabilityKey, err := NewCapabilityKeyFromString(s)
	if err != nil {
		return nil, err
	}
	return NewCapabilityKeyBuilderFromCapabilityKey(capabilityKey), nil
}

// Sub adds a segment to the capability ID
func (b *CapabilityKeyBuilder) Sub(segment string) *CapabilityKeyBuilder {
	b.segments = append(b.segments, segment)
	return b
}

// Subs adds multiple segments to the capability ID
func (b *CapabilityKeyBuilder) Subs(segments ...string) *CapabilityKeyBuilder {
	b.segments = append(b.segments, segments...)
	return b
}

// SubsFromSlice adds multiple segments from a slice to the capability ID
func (b *CapabilityKeyBuilder) SubsFromSlice(segments []string) *CapabilityKeyBuilder {
	b.segments = append(b.segments, segments...)
	return b
}

// ReplaceSegment replaces a segment at the given index
func (b *CapabilityKeyBuilder) ReplaceSegment(index int, segment string) *CapabilityKeyBuilder {
	if index >= 0 && index < len(b.segments) {
		b.segments[index] = segment
	}
	return b
}

// MakeMoreGeneral removes the last segment (make more general)
func (b *CapabilityKeyBuilder) MakeMoreGeneral() *CapabilityKeyBuilder {
	if len(b.segments) > 0 {
		b.segments = b.segments[:len(b.segments)-1]
	}
	return b
}

// MakeGeneralToLevel removes segments from the given index onwards (make more general to that level)
func (b *CapabilityKeyBuilder) MakeGeneralToLevel(level int) *CapabilityKeyBuilder {
	if level >= 0 && level < len(b.segments) {
		b.segments = b.segments[:level]
	}
	return b
}

// AddWildcard adds a wildcard segment
func (b *CapabilityKeyBuilder) AddWildcard() *CapabilityKeyBuilder {
	return b.Sub("*")
}

// MakeWildcard replaces the last segment with a wildcard
func (b *CapabilityKeyBuilder) MakeWildcard() *CapabilityKeyBuilder {
	if len(b.segments) > 0 {
		b.segments[len(b.segments)-1] = "*"
	}
	return b
}

// MakeWildcardFromLevel replaces all segments from the given index with a wildcard
func (b *CapabilityKeyBuilder) MakeWildcardFromLevel(level int) *CapabilityKeyBuilder {
	if level >= 0 {
		if level < len(b.segments) {
			b.segments = b.segments[:level+1]
			b.segments[level] = "*"
		} else if level == len(b.segments) {
			b.segments = append(b.segments, "*")
		}
	}
	return b
}

// Segments returns the current segments as a copy
func (b *CapabilityKeyBuilder) Segments() []string {
	segments := make([]string, len(b.segments))
	copy(segments, b.segments)
	return segments
}

// Len returns the number of segments
func (b *CapabilityKeyBuilder) Len() int {
	return len(b.segments)
}

// IsEmpty checks if the builder is empty
func (b *CapabilityKeyBuilder) IsEmpty() bool {
	return len(b.segments) == 0
}

// Clear removes all segments
func (b *CapabilityKeyBuilder) Clear() *CapabilityKeyBuilder {
	b.segments = b.segments[:0]
	return b
}

// Clone creates a copy of the builder
func (b *CapabilityKeyBuilder) Clone() *CapabilityKeyBuilder {
	segments := make([]string, len(b.segments))
	copy(segments, b.segments)
	return &CapabilityKeyBuilder{
		segments: segments,
	}
}

// Build creates the final CapabilityKey
func (b *CapabilityKeyBuilder) Build() (*CapabilityKey, error) {
	return NewCapabilityKeyFromSegments(b.segments)
}

// BuildString creates the final CapabilityKey as a string
func (b *CapabilityKeyBuilder) BuildString() (string, error) {
	capabilityKey, err := b.Build()
	if err != nil {
		return "", err
	}
	return capabilityKey.ToString(), nil
}

// String returns the current capability ID as a string (for debugging)
func (b *CapabilityKeyBuilder) String() string {
	return strings.Join(b.segments, ":")
}

// CapabilityKeyBuilderInterface defines the interface for creating builders from various types
type CapabilityKeyBuilderInterface interface {
	IntoBuilder() (*CapabilityKeyBuilder, error)
}

// StringIntoBuilder converts a string into a capability ID builder
func StringIntoBuilder(s string) (*CapabilityKeyBuilder, error) {
	return NewCapabilityKeyBuilderFromString(s)
}

// CapabilityKeyIntoBuilder converts a CapabilityKey into a builder
func CapabilityKeyIntoBuilder(capId *CapabilityKey) (*CapabilityKeyBuilder, error) {
	return NewCapabilityKeyBuilderFromCapabilityKey(capId), nil
}