// Package capdef provides the fundamental capability identifier system used across
// all LBVR plugins and providers. It defines the formal structure for capability
// identifiers with flat tag-based naming, wildcard support, and specificity comparison.
package capdef

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// CapabilityKey represents a capability identifier using flat, ordered tags
//
// Examples:
// - action=generate;format=pdf;output=binary;target=thumbnail;type=document
// - action=extract;target=metadata;type=document
// - action=analysis;format=en;type=inference
type CapabilityKey struct {
	tags map[string]string
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
	ErrorEmptyTag         = 2
	ErrorInvalidCharacter = 3
	ErrorInvalidTagFormat = 4
)

var validTagComponentPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\*]+$`)

// NewCapabilityKeyFromString creates a capability identifier from a string
// Format: key1=value1;key2=value2;...
// Tags are automatically sorted alphabetically for canonical form
func NewCapabilityKeyFromString(s string) (*CapabilityKey, error) {
	if s == "" {
		return nil, &CapabilityKeyError{
			Code:    ErrorInvalidFormat,
			Message: "capability identifier cannot be empty",
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
			return nil, &CapabilityKeyError{
				Code:    ErrorInvalidTagFormat,
				Message: fmt.Sprintf("invalid tag format (must be key=value): %s", tagStr),
			}
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" || value == "" {
			return nil, &CapabilityKeyError{
				Code:    ErrorEmptyTag,
				Message: fmt.Sprintf("tag key or value cannot be empty: %s", tagStr),
			}
		}

		// Validate key and value characters
		if !validTagComponentPattern.MatchString(key) || !validTagComponentPattern.MatchString(value) {
			return nil, &CapabilityKeyError{
				Code:    ErrorInvalidCharacter,
				Message: fmt.Sprintf("invalid character in tag (use alphanumeric, _, -): %s", tagStr),
			}
		}

		tags[key] = value
	}

	if len(tags) == 0 {
		return nil, &CapabilityKeyError{
			Code:    ErrorInvalidFormat,
			Message: "capability identifier cannot be empty",
		}
	}

	return &CapabilityKey{
		tags: tags,
	}, nil
}

// NewCapabilityKeyFromTags creates a capability identifier from tags
func NewCapabilityKeyFromTags(tags map[string]string) *CapabilityKey {
	result := make(map[string]string)
	for k, v := range tags {
		result[k] = v
	}
	return &CapabilityKey{
		tags: result,
	}
}

// GetTag returns the value of a specific tag
func (c *CapabilityKey) GetTag(key string) (string, bool) {
	value, exists := c.tags[key]
	return value, exists
}

// HasTag checks if this capability has a specific tag with a specific value
func (c *CapabilityKey) HasTag(key, value string) bool {
	tagValue, exists := c.tags[key]
	return exists && tagValue == value
}

// WithTag returns a new capability key with an added or updated tag
func (c *CapabilityKey) WithTag(key, value string) *CapabilityKey {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	newTags[key] = value
	return &CapabilityKey{tags: newTags}
}

// WithoutTag returns a new capability key with a tag removed
func (c *CapabilityKey) WithoutTag(key string) *CapabilityKey {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		if k != key {
			newTags[k] = v
		}
	}
	return &CapabilityKey{tags: newTags}
}

// Matches checks if this capability matches another based on tag compatibility
//
// A capability matches a request if:
// - For each tag in the request: capability has same value, wildcard (*), or missing tag
// - For each tag in the capability: if request is missing that tag, that's fine (capability is more specific)
// Missing tags are treated as wildcards (less specific, can handle any value).
func (c *CapabilityKey) Matches(request *CapabilityKey) bool {
	if request == nil {
		return true
	}

	// Check all tags that the request specifies
	for requestKey, requestValue := range request.tags {
		capValue, exists := c.tags[requestKey]
		if !exists {
			// Missing tag in capability is treated as wildcard - can handle any value
			continue
		}

		if capValue == "*" {
			// Capability has wildcard - can handle any value
			continue
		}

		if requestValue == "*" {
			// Request accepts any value - capability's specific value matches
			continue
		}

		if capValue != requestValue {
			// Capability has specific value that doesn't match request's specific value
			return false
		}
	}

	// If capability has additional specific tags that request doesn't specify, that's fine
	// The capability is just more specific than needed
	return true
}

// CanHandle checks if this capability can handle a request
func (c *CapabilityKey) CanHandle(request *CapabilityKey) bool {
	return c.Matches(request)
}

// Specificity returns the specificity score for capability matching
// More specific capabilities have higher scores and are preferred
func (c *CapabilityKey) Specificity() int {
	// Count non-wildcard tags
	count := 0
	for _, value := range c.tags {
		if value != "*" {
			count++
		}
	}
	return count
}

// IsMoreSpecificThan checks if this capability is more specific than another
func (c *CapabilityKey) IsMoreSpecificThan(other *CapabilityKey) bool {
	if other == nil {
		return true
	}

	// First check if they're compatible
	if !c.IsCompatibleWith(other) {
		return false
	}

	return c.Specificity() > other.Specificity()
}

// IsCompatibleWith checks if this capability is compatible with another
//
// Two capabilities are compatible if they can potentially match
// the same types of requests (considering wildcards and missing tags as wildcards)
func (c *CapabilityKey) IsCompatibleWith(other *CapabilityKey) bool {
	if other == nil {
		return true
	}

	// Get all unique tag keys from both capabilities
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

// GetType returns the type of this capability (convenience method)
func (c *CapabilityKey) GetType() (string, bool) {
	return c.GetTag("type")
}

// GetAction returns the action of this capability (convenience method)
func (c *CapabilityKey) GetAction() (string, bool) {
	return c.GetTag("action")
}

// GetTarget returns the target of this capability (convenience method)
func (c *CapabilityKey) GetTarget() (string, bool) {
	return c.GetTag("target")
}

// GetFormat returns the format of this capability (convenience method)
func (c *CapabilityKey) GetFormat() (string, bool) {
	return c.GetTag("format")
}

// GetOutput returns the output type of this capability (convenience method)
func (c *CapabilityKey) GetOutput() (string, bool) {
	return c.GetTag("output")
}

// IsBinaryOutput checks if this capability produces binary output
func (c *CapabilityKey) IsBinaryOutput() bool {
	return c.HasTag("output", "binary")
}

// WithWildcardTag returns a new capability with a specific tag set to wildcard
func (c *CapabilityKey) WithWildcardTag(key string) *CapabilityKey {
	if _, exists := c.tags[key]; exists {
		return c.WithTag(key, "*")
	}
	return c
}

// Subset returns a new capability with only specified tags
func (c *CapabilityKey) Subset(keys []string) *CapabilityKey {
	newTags := make(map[string]string)
	for _, key := range keys {
		if value, exists := c.tags[key]; exists {
			newTags[key] = value
		}
	}
	return &CapabilityKey{tags: newTags}
}

// Merge returns a new capability merged with another (other takes precedence for conflicts)
func (c *CapabilityKey) Merge(other *CapabilityKey) *CapabilityKey {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	for k, v := range other.tags {
		newTags[k] = v
	}
	return &CapabilityKey{tags: newTags}
}

// ToString returns the canonical string representation of this capability identifier
// Tags are sorted alphabetically for consistent representation
func (c *CapabilityKey) ToString() string {
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
func (c *CapabilityKey) String() string {
	return c.ToString()
}

// Equals checks if this capability identifier is equal to another
func (c *CapabilityKey) Equals(other *CapabilityKey) bool {
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
func (c *CapabilityKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.ToString())
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (c *CapabilityKey) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	capKey, err := NewCapabilityKeyFromString(s)
	if err != nil {
		return err
	}

	c.tags = capKey.tags
	return nil
}

// CapabilityMatcher provides utility methods for matching capabilities
type CapabilityMatcher struct{}

// FindBestMatch finds the most specific capability that can handle a request
func (m *CapabilityMatcher) FindBestMatch(capabilities []*CapabilityKey, request *CapabilityKey) *CapabilityKey {
	var best *CapabilityKey
	bestSpecificity := -1

	for _, cap := range capabilities {
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

// FindAllMatches finds all capabilities that can handle a request, sorted by specificity
func (m *CapabilityMatcher) FindAllMatches(capabilities []*CapabilityKey, request *CapabilityKey) []*CapabilityKey {
	var matches []*CapabilityKey

	for _, cap := range capabilities {
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

// AreCompatible checks if two capability sets are compatible
func (m *CapabilityMatcher) AreCompatible(caps1, caps2 []*CapabilityKey) bool {
	for _, c1 := range caps1 {
		for _, c2 := range caps2 {
			if c1.IsCompatibleWith(c2) {
				return true
			}
		}
	}
	return false
}

// CapabilityKeyBuilder provides a fluent builder interface for creating capability keys
type CapabilityKeyBuilder struct {
	tags map[string]string
}

// NewCapabilityKeyBuilder creates a new builder
func NewCapabilityKeyBuilder() *CapabilityKeyBuilder {
	return &CapabilityKeyBuilder{
		tags: make(map[string]string),
	}
}

// Tag adds or updates a tag
func (b *CapabilityKeyBuilder) Tag(key, value string) *CapabilityKeyBuilder {
	b.tags[key] = value
	return b
}

// Type sets the type tag
func (b *CapabilityKeyBuilder) Type(value string) *CapabilityKeyBuilder {
	return b.Tag("type", value)
}

// Action sets the action tag
func (b *CapabilityKeyBuilder) Action(value string) *CapabilityKeyBuilder {
	return b.Tag("action", value)
}

// Target sets the target tag
func (b *CapabilityKeyBuilder) Target(value string) *CapabilityKeyBuilder {
	return b.Tag("target", value)
}

// Format sets the format tag
func (b *CapabilityKeyBuilder) Format(value string) *CapabilityKeyBuilder {
	return b.Tag("format", value)
}

// Output sets the output tag
func (b *CapabilityKeyBuilder) Output(value string) *CapabilityKeyBuilder {
	return b.Tag("output", value)
}

// BinaryOutput sets output to binary
func (b *CapabilityKeyBuilder) BinaryOutput() *CapabilityKeyBuilder {
	return b.Output("binary")
}

// JSONOutput sets output to json
func (b *CapabilityKeyBuilder) JSONOutput() *CapabilityKeyBuilder {
	return b.Output("json")
}

// Build creates the final CapabilityKey
func (b *CapabilityKeyBuilder) Build() (*CapabilityKey, error) {
	if len(b.tags) == 0 {
		return nil, &CapabilityKeyError{
			Code:    ErrorInvalidFormat,
			Message: "capability identifier cannot be empty",
		}
	}

	return NewCapabilityKeyFromTags(b.tags), nil
}