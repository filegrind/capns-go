// Package capdef provides the fundamental capability identifier system used across
// all LBVR plugins and providers. It defines the formal structure for capability
// identifiers with hierarchical naming, wildcard support, and specificity comparison.
package capdef

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CapabilityKey represents a formal capability identifier with hierarchical naming and wildcard support
//
// Examples:
// - file_handling:thumbnail_generation:pdf
// - file_handling:thumbnail_generation:*
// - file_handling:*
// - data_processing:transform:json
type CapabilityKey struct {
	segments []string
}

// CapabilityKeyError represents errors that can occur during capability identifier operations
type CapabilityKeyError struct {
	Code    int
	Message string
}

func (e *CapabilityKeyError) Error() string {
	return e.Message
}

// Error codes for capability identifier operations
const (
	ErrorInvalidFormat     = 1
	ErrorEmptySegment     = 2
	ErrorInvalidCharacter = 3
)

var validSegmentPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\*]+$`)



// NewCapabilityKeyFromString creates a capability identifier from a string
func NewCapabilityKeyFromString(s string) (*CapabilityKey, error) {
	if s == "" {
		return nil, &CapabilityKeyError{
			Code:    ErrorInvalidFormat,
			Message: "capability identifier cannot be empty",
		}
	}

	segments := strings.Split(s, ":")
	return NewCapabilityKeyFromSegments(segments)
}

// NewCapabilityKeyFromSegments creates a capability identifier from segments
func NewCapabilityKeyFromSegments(segments []string) (*CapabilityKey, error) {
	if len(segments) == 0 {
		return nil, &CapabilityKeyError{
			Code:    ErrorInvalidFormat,
			Message: "capability identifier must have at least one segment",
		}
	}

	// Validate segments
	for _, segment := range segments {
		if segment == "" {
			return nil, &CapabilityKeyError{
				Code:    ErrorEmptySegment,
				Message: "capability identifier segments cannot be empty",
			}
		}

		if !validSegmentPattern.MatchString(segment) {
			return nil, &CapabilityKeyError{
				Code:    ErrorInvalidCharacter,
				Message: fmt.Sprintf("invalid character in segment: %s", segment),
			}
		}
	}

	return &CapabilityKey{
		segments: segments,
	}, nil
}

// Segments returns the segments of the capability identifier
func (c *CapabilityKey) Segments() []string {
	result := make([]string, len(c.segments))
	copy(result, c.segments)
	return result
}

/**
 * Check if this capability produces binary output
 * @return YES if the capability has the "bin:" prefix
 */
func (c *CapabilityKey) IsBinaryOutput() bool {
	if len(c.segments) == 0 {
		return false
	}
	return c.segments[0] == "bin"
}


// CanHandle checks if this capability can handle a request
func (c *CapabilityKey) CanHandle(request *CapabilityKey) bool {
	if request == nil {
		return false
	}

	// Check each segment up to the minimum of both lengths
	minLength := len(c.segments)
	if len(request.segments) < minLength {
		minLength = len(request.segments)
	}

	for i := 0; i < minLength; i++ {
		mySegment := c.segments[i]
		requestSegment := request.segments[i]

		// Wildcard in capability matches anything and consumes all remaining segments
		if mySegment == "*" {
			return true
		}

		// Exact match required
		if mySegment != requestSegment {
			return false
		}
	}

	// If we've checked all capability segments and none were wildcards,
	// then we can only handle if the request has no more segments
	return len(request.segments) <= len(c.segments)
}

// IsCompatibleWith checks if this capability is compatible with another
func (c *CapabilityKey) IsCompatibleWith(other *CapabilityKey) bool {
	if other == nil {
		return false
	}

	minLength := len(c.segments)
	if len(other.segments) < minLength {
		minLength = len(other.segments)
	}

	for i := 0; i < minLength; i++ {
		mySegment := c.segments[i]
		otherSegment := other.segments[i]

		// Wildcards are compatible with anything
		if mySegment == "*" || otherSegment == "*" {
			continue
		}

		// Must match exactly
		if mySegment != otherSegment {
			return false
		}
	}

	return true
}

// IsMoreSpecificThan checks if this capability is more specific than another
func (c *CapabilityKey) IsMoreSpecificThan(other *CapabilityKey) bool {
	if other == nil {
		return true
	}

	mySpecificity := c.SpecificityLevel()
	otherSpecificity := other.SpecificityLevel()

	if mySpecificity != otherSpecificity {
		return mySpecificity > otherSpecificity
	}

	// Same specificity level, check segment count
	return len(c.segments) > len(other.segments)
}

// SpecificityLevel returns the specificity level of this capability
func (c *CapabilityKey) SpecificityLevel() int {
	count := 0
	for _, segment := range c.segments {
		if segment != "*" {
			count++
		}
	}
	return count
}

// IsWildcardAtLevel checks if this capability is a wildcard at a given level
func (c *CapabilityKey) IsWildcardAtLevel(level int) bool {
	if level >= len(c.segments) {
		return false
	}
	return c.segments[level] == "*"
}

// ToString returns the string representation of this capability
func (c *CapabilityKey) ToString() string {
	return strings.Join(c.segments, ":")
}

// String implements the Stringer interface
func (c *CapabilityKey) String() string {
	return c.ToString()
}

// Equals checks if this capability identifier is equal to another
func (c *CapabilityKey) Equals(other *CapabilityKey) bool {
	if other == nil {
		return false
	}

	if len(c.segments) != len(other.segments) {
		return false
	}

	for i, segment := range c.segments {
		if segment != other.segments[i] {
			return false
		}
	}

	return true
}

// MarshalJSON implements the json.Marshaler interface
func (c *CapabilityKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.ToString())
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (c *CapabilityKey) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	capId, err := NewCapabilityKeyFromString(s)
	if err != nil {
		return err
	}

	c.segments = capId.segments
	return nil
}