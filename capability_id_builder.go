package capdef

import (
	"strings"
)

// CapabilityIdBuilder provides a fluent builder interface for constructing and manipulating capability identifiers.
// This replaces manual creation and manipulation of capability IDs with a type-safe API.
type CapabilityIdBuilder struct {
	segments []string
}

// NewCapabilityIdBuilder creates a new empty builder
func NewCapabilityIdBuilder() *CapabilityIdBuilder {
	return &CapabilityIdBuilder{
		segments: make([]string, 0),
	}
}

// NewCapabilityIdBuilderFromCapabilityId creates a builder starting with a base capability ID
func NewCapabilityIdBuilderFromCapabilityId(capabilityId *CapabilityId) *CapabilityIdBuilder {
	segments := make([]string, len(capabilityId.segments))
	copy(segments, capabilityId.segments)
	return &CapabilityIdBuilder{
		segments: segments,
	}
}

// NewCapabilityIdBuilderFromString creates a builder from a capability string
func NewCapabilityIdBuilderFromString(s string) (*CapabilityIdBuilder, error) {
	capabilityId, err := NewCapabilityIdFromString(s)
	if err != nil {
		return nil, err
	}
	return NewCapabilityIdBuilderFromCapabilityId(capabilityId), nil
}

// Sub adds a segment to the capability ID
func (b *CapabilityIdBuilder) Sub(segment string) *CapabilityIdBuilder {
	b.segments = append(b.segments, segment)
	return b
}

// Subs adds multiple segments to the capability ID
func (b *CapabilityIdBuilder) Subs(segments ...string) *CapabilityIdBuilder {
	b.segments = append(b.segments, segments...)
	return b
}

// SubsFromSlice adds multiple segments from a slice to the capability ID
func (b *CapabilityIdBuilder) SubsFromSlice(segments []string) *CapabilityIdBuilder {
	b.segments = append(b.segments, segments...)
	return b
}

// ReplaceSegment replaces a segment at the given index
func (b *CapabilityIdBuilder) ReplaceSegment(index int, segment string) *CapabilityIdBuilder {
	if index >= 0 && index < len(b.segments) {
		b.segments[index] = segment
	}
	return b
}

// MakeMoreGeneral removes the last segment (make more general)
func (b *CapabilityIdBuilder) MakeMoreGeneral() *CapabilityIdBuilder {
	if len(b.segments) > 0 {
		b.segments = b.segments[:len(b.segments)-1]
	}
	return b
}

// MakeGeneralToLevel removes segments from the given index onwards (make more general to that level)
func (b *CapabilityIdBuilder) MakeGeneralToLevel(level int) *CapabilityIdBuilder {
	if level >= 0 && level < len(b.segments) {
		b.segments = b.segments[:level]
	}
	return b
}

// AddWildcard adds a wildcard segment
func (b *CapabilityIdBuilder) AddWildcard() *CapabilityIdBuilder {
	return b.Sub("*")
}

// MakeWildcard replaces the last segment with a wildcard
func (b *CapabilityIdBuilder) MakeWildcard() *CapabilityIdBuilder {
	if len(b.segments) > 0 {
		b.segments[len(b.segments)-1] = "*"
	}
	return b
}

// MakeWildcardFromLevel replaces all segments from the given index with a wildcard
func (b *CapabilityIdBuilder) MakeWildcardFromLevel(level int) *CapabilityIdBuilder {
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
func (b *CapabilityIdBuilder) Segments() []string {
	segments := make([]string, len(b.segments))
	copy(segments, b.segments)
	return segments
}

// Len returns the number of segments
func (b *CapabilityIdBuilder) Len() int {
	return len(b.segments)
}

// IsEmpty checks if the builder is empty
func (b *CapabilityIdBuilder) IsEmpty() bool {
	return len(b.segments) == 0
}

// Clear removes all segments
func (b *CapabilityIdBuilder) Clear() *CapabilityIdBuilder {
	b.segments = b.segments[:0]
	return b
}

// Clone creates a copy of the builder
func (b *CapabilityIdBuilder) Clone() *CapabilityIdBuilder {
	segments := make([]string, len(b.segments))
	copy(segments, b.segments)
	return &CapabilityIdBuilder{
		segments: segments,
	}
}

// Build creates the final CapabilityId
func (b *CapabilityIdBuilder) Build() (*CapabilityId, error) {
	return NewCapabilityIdFromSegments(b.segments)
}

// BuildString creates the final CapabilityId as a string
func (b *CapabilityIdBuilder) BuildString() (string, error) {
	capabilityId, err := b.Build()
	if err != nil {
		return "", err
	}
	return capabilityId.ToString(), nil
}

// String returns the current capability ID as a string (for debugging)
func (b *CapabilityIdBuilder) String() string {
	return strings.Join(b.segments, ":")
}

// CapabilityIdBuilderInterface defines the interface for creating builders from various types
type CapabilityIdBuilderInterface interface {
	IntoBuilder() (*CapabilityIdBuilder, error)
}

// StringIntoBuilder converts a string into a capability ID builder
func StringIntoBuilder(s string) (*CapabilityIdBuilder, error) {
	return NewCapabilityIdBuilderFromString(s)
}

// CapabilityIdIntoBuilder converts a CapabilityId into a builder
func CapabilityIdIntoBuilder(capId *CapabilityId) (*CapabilityIdBuilder, error) {
	return NewCapabilityIdBuilderFromCapabilityId(capId), nil
}