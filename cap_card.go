// Package capdef provides the fundamental cap identifier system used across
// all LBVR plugins and providers. It defines the formal structure for cap
// identifiers with flat tag-based naming, wildcard support, and specificity comparison.
package capdef

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// CapCard represents a cap identifier using flat, ordered tags
//
// Examples:
// - action=generate;ext=pdf;output=binary;target=thumbnail;
// - action=extract;target=metadata;
// - action=analysis;format=en;type=inference
type CapCard struct {
	tags map[string]string
}

// CapCardError represents errors that can occur during cap identifier operations
type CapCardError struct {
	Code    int
	Message string
}

func (e *CapCardError) Error() string {
	return e.Message
}

// Error codes for cap identifier operations
const (
	ErrorInvalidFormat     = 1
	ErrorEmptyTag         = 2
	ErrorInvalidCharacter = 3
	ErrorInvalidTagFormat = 4
)

var validTagComponentPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\*]+$`)

// NewCapCardFromString creates a cap identifier from a string
// Format: key1=value1;key2=value2;...
// Tags are automatically sorted alphabetically for canonical form
func NewCapCardFromString(s string) (*CapCard, error) {
	if s == "" {
		return nil, &CapCardError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	tags := make(map[string]string)

	for _, tagStr := range strings.Split(s, ";") {
		tagStr = strings.TrimSpace(tagStr)
		if tagStr == "" {
			continue
		}

		parts := strings.Split(tagStr, "=")
		if len(parts) != 2 {
			return nil, &CapCardError{
				Code:    ErrorInvalidTagFormat,
				Message: fmt.Sprintf("invalid tag format (must be key=value): %s", tagStr),
			}
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" || value == "" {
			return nil, &CapCardError{
				Code:    ErrorEmptyTag,
				Message: fmt.Sprintf("tag key or value cannot be empty: %s", tagStr),
			}
		}

		// Validate key and value characters
		if !validTagComponentPattern.MatchString(key) || !validTagComponentPattern.MatchString(value) {
			return nil, &CapCardError{
				Code:    ErrorInvalidCharacter,
				Message: fmt.Sprintf("invalid character in tag (use alphanumeric, _, -): %s", tagStr),
			}
		}

		tags[key] = value
	}

	if len(tags) == 0 {
		return nil, &CapCardError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	return &CapCard{
		tags: tags,
	}, nil
}

// NewCapCardFromTags creates a cap identifier from tags
func NewCapCardFromTags(tags map[string]string) *CapCard {
	result := make(map[string]string)
	for k, v := range tags {
		result[k] = v
	}
	return &CapCard{
		tags: result,
	}
}

// GetTag returns the value of a specific tag
func (c *CapCard) GetTag(key string) (string, bool) {
	value, exists := c.tags[key]
	return value, exists
}

// HasTag checks if this cap has a specific tag with a specific value
func (c *CapCard) HasTag(key, value string) bool {
	tagValue, exists := c.tags[key]
	return exists && tagValue == value
}

// WithTag returns a new cap card with an added or updated tag
func (c *CapCard) WithTag(key, value string) *CapCard {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	newTags[key] = value
	return &CapCard{tags: newTags}
}

// WithoutTag returns a new cap card with a tag removed
func (c *CapCard) WithoutTag(key string) *CapCard {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		if k != key {
			newTags[k] = v
		}
	}
	return &CapCard{tags: newTags}
}

// Matches checks if this cap matches another based on tag compatibility
//
// A cap matches a request if:
// - For each tag in the request: cap has same value, wildcard (*), or missing tag
// - For each tag in the cap: if request is missing that tag, that's fine (cap is more specific)
// Missing tags are treated as wildcards (less specific, can handle any value).
func (c *CapCard) Matches(request *CapCard) bool {
	if request == nil {
		return true
	}

	// Check all tags that the request specifies
	for requestKey, requestValue := range request.tags {
		capValue, exists := c.tags[requestKey]
		if !exists {
			// Missing tag in cap is treated as wildcard - can handle any value
			continue
		}

		if capValue == "*" {
			// Cap has wildcard - can handle any value
			continue
		}

		if requestValue == "*" {
			// Request accepts any value - cap's specific value matches
			continue
		}

		if capValue != requestValue {
			// Cap has specific value that doesn't match request's specific value
			return false
		}
	}

	// If cap has additional specific tags that request doesn't specify, that's fine
	// The cap is just more specific than needed
	return true
}

// CanHandle checks if this cap can handle a request
func (c *CapCard) CanHandle(request *CapCard) bool {
	return c.Matches(request)
}

// Specificity returns the specificity score for cap matching
// More specific caps have higher scores and are preferred
func (c *CapCard) Specificity() int {
	// Count non-wildcard tags
	count := 0
	for _, value := range c.tags {
		if value != "*" {
			count++
		}
	}
	return count
}

// IsMoreSpecificThan checks if this cap is more specific than another
func (c *CapCard) IsMoreSpecificThan(other *CapCard) bool {
	if other == nil {
		return true
	}

	// First check if they're compatible
	if !c.IsCompatibleWith(other) {
		return false
	}

	return c.Specificity() > other.Specificity()
}

// IsCompatibleWith checks if this cap is compatible with another
//
// Two caps are compatible if they can potentially match
// the same types of requests (considering wildcards and missing tags as wildcards)
func (c *CapCard) IsCompatibleWith(other *CapCard) bool {
	if other == nil {
		return true
	}

	// Get all unique tag keys from both caps
	allKeys := make(map[string]bool)
	for key := range c.tags {
		allKeys[key] = true
	}
	for key := range other.tags {
		allKeys[key] = true
	}

	for key := range allKeys {
		v1, exists1 := c.tags[key]
		v2, exists2 := other.tags[key]

		if exists1 && exists2 {
			// Both have the tag - they must match or one must be wildcard
			if v1 != "*" && v2 != "*" && v1 != v2 {
				return false
			}
		}
		// If only one has the tag, it's compatible (missing tag is wildcard)
	}

	return true
}

// WithWildcardTag returns a new cap with a specific tag set to wildcard
func (c *CapCard) WithWildcardTag(key string) *CapCard {
	if _, exists := c.tags[key]; exists {
		return c.WithTag(key, "*")
	}
	return c
}

// Subset returns a new cap with only specified tags
func (c *CapCard) Subset(keys []string) *CapCard {
	newTags := make(map[string]string)
	for _, key := range keys {
		if value, exists := c.tags[key]; exists {
			newTags[key] = value
		}
	}
	return &CapCard{tags: newTags}
}

// Merge returns a new cap merged with another (other takes precedence for conflicts)
func (c *CapCard) Merge(other *CapCard) *CapCard {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	for k, v := range other.tags {
		newTags[k] = v
	}
	return &CapCard{tags: newTags}
}

// ToString returns the canonical string representation of this cap identifier
// Tags are sorted alphabetically for consistent representation
func (c *CapCard) ToString() string {
	if len(c.tags) == 0 {
		return ""
	}

	// Sort keys for canonical representation
	keys := make([]string, 0, len(c.tags))
	for key := range c.tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Build tag string
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, c.tags[key]))
	}

	return strings.Join(parts, ";")
}

// String implements the Stringer interface
func (c *CapCard) String() string {
	return c.ToString()
}

// Equals checks if this cap identifier is equal to another
func (c *CapCard) Equals(other *CapCard) bool {
	if other == nil {
		return false
	}

	if len(c.tags) != len(other.tags) {
		return false
	}

	for key, value := range c.tags {
		otherValue, exists := other.tags[key]
		if !exists || value != otherValue {
			return false
		}
	}

	return true
}

// MarshalJSON implements the json.Marshaler interface
func (c *CapCard) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.ToString())
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (c *CapCard) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	capCard, err := NewCapCardFromString(s)
	if err != nil {
		return err
	}

	c.tags = capCard.tags
	return nil
}

// CapMatcher provides utility methods for matching caps
type CapMatcher struct{}

// FindBestMatch finds the most specific cap that can handle a request
func (m *CapMatcher) FindBestMatch(caps []*CapCard, request *CapCard) *CapCard {
	var best *CapCard
	bestSpecificity := -1

	for _, cap := range caps {
		if cap.CanHandle(request) {
			specificity := cap.Specificity()
			if specificity > bestSpecificity {
				best = cap
				bestSpecificity = specificity
			}
		}
	}

	return best
}

// FindAllMatches finds all caps that can handle a request, sorted by specificity
func (m *CapMatcher) FindAllMatches(caps []*CapCard, request *CapCard) []*CapCard {
	var matches []*CapCard

	for _, cap := range caps {
		if cap.CanHandle(request) {
			matches = append(matches, cap)
		}
	}

	// Sort by specificity (most specific first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Specificity() > matches[j].Specificity()
	})

	return matches
}

// AreCompatible checks if two cap sets are compatible
func (m *CapMatcher) AreCompatible(caps1, caps2 []*CapCard) bool {
	for _, c1 := range caps1 {
		for _, c2 := range caps2 {
			if c1.IsCompatibleWith(c2) {
				return true
			}
		}
	}
	return false
}

// CapCardBuilder provides a fluent builder interface for creating cap cards
type CapCardBuilder struct {
	tags map[string]string
}

// NewCapCardBuilder creates a new builder
func NewCapCardBuilder() *CapCardBuilder {
	return &CapCardBuilder{
		tags: make(map[string]string),
	}
}

// Tag adds or updates a tag
func (b *CapCardBuilder) Tag(key, value string) *CapCardBuilder {
	b.tags[key] = value
	return b
}

// Build creates the final CapCard
func (b *CapCardBuilder) Build() (*CapCard, error) {
	if len(b.tags) == 0 {
		return nil, &CapCardError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	return NewCapCardFromTags(b.tags), nil
}