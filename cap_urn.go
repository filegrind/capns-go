// Package capns provides the fundamental cap identifier system used across
// all FMIO plugins and providers. It defines the formal structure for cap
// identifiers with flat tag-based naming, wildcard support, and specificity comparison.
package capns

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// CapUrn represents a cap identifier using flat, ordered tags
//
// Examples:
// - action=generate;ext=pdf;output=binary;target=thumbnail;
// - action=extract;target=metadata;
// - action=analysis;format=en;type=constrained
type CapUrn struct {
	tags map[string]string
}

// CapUrnError represents errors that can occur during cap identifier operations
type CapUrnError struct {
	Code    int
	Message string
}

func (e *CapUrnError) Error() string {
	return e.Message
}

// Error codes for cap identifier operations
const (
	ErrorInvalidFormat     = 1
	ErrorEmptyTag         = 2
	ErrorInvalidCharacter = 3
	ErrorInvalidTagFormat = 4
	ErrorMissingCapPrefix = 5
)

var validTagComponentPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\*]+$`)

// NewCapUrnFromString creates a cap identifier from a string
// Format: cap:key1=value1;key2=value2;...
// The "cap:" prefix is mandatory
// Trailing semicolons are optional and ignored
// Tags are automatically sorted alphabetically for canonical form
func NewCapUrnFromString(s string) (*CapUrn, error) {
	if s == "" {
		return nil, &CapUrnError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	// Ensure "cap:" prefix is present
	if !strings.HasPrefix(s, "cap:") {
		return nil, &CapUrnError{
			Code:    ErrorMissingCapPrefix,
			Message: "cap identifier must start with 'cap:'",
		}
	}

	// Remove the "cap:" prefix
	tagsPart := s[4:]
	if tagsPart == "" {
		return nil, &CapUrnError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	tags := make(map[string]string)

	// Remove trailing semicolon if present
	normalizedTagsPart := strings.TrimSuffix(tagsPart, ";")

	for _, tagStr := range strings.Split(normalizedTagsPart, ";") {
		tagStr = strings.TrimSpace(tagStr)
		if tagStr == "" {
			continue
		}

		parts := strings.Split(tagStr, "=")
		if len(parts) != 2 {
			return nil, &CapUrnError{
				Code:    ErrorInvalidTagFormat,
				Message: fmt.Sprintf("invalid tag format (must be key=value): %s", tagStr),
			}
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" || value == "" {
			return nil, &CapUrnError{
				Code:    ErrorEmptyTag,
				Message: fmt.Sprintf("tag key or value cannot be empty: %s", tagStr),
			}
		}

		// Validate key and value characters
		if !validTagComponentPattern.MatchString(key) || !validTagComponentPattern.MatchString(value) {
			return nil, &CapUrnError{
				Code:    ErrorInvalidCharacter,
				Message: fmt.Sprintf("invalid character in tag (use alphanumeric, _, -, *): %s", tagStr),
			}
		}

		tags[key] = value
	}

	if len(tags) == 0 {
		return nil, &CapUrnError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	return &CapUrn{
		tags: tags,
	}, nil
}

// NewCapUrnFromTags creates a cap identifier from tags
func NewCapUrnFromTags(tags map[string]string) *CapUrn {
	result := make(map[string]string)
	for k, v := range tags {
		result[k] = v
	}
	return &CapUrn{
		tags: result,
	}
}

// GetTag returns the value of a specific tag
func (c *CapUrn) GetTag(key string) (string, bool) {
	value, exists := c.tags[key]
	return value, exists
}

// HasTag checks if this cap has a specific tag with a specific value
func (c *CapUrn) HasTag(key, value string) bool {
	tagValue, exists := c.tags[key]
	return exists && tagValue == value
}

// WithTag returns a new cap URN with an added or updated tag
func (c *CapUrn) WithTag(key, value string) *CapUrn {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	newTags[key] = value
	return &CapUrn{tags: newTags}
}

// WithoutTag returns a new cap URN with a tag removed
func (c *CapUrn) WithoutTag(key string) *CapUrn {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		if k != key {
			newTags[k] = v
		}
	}
	return &CapUrn{tags: newTags}
}

// Matches checks if this cap matches another based on tag compatibility
//
// A cap matches a request if:
// - For each tag in the request: cap has same value, wildcard (*), or missing tag
// - For each tag in the cap: if request is missing that tag, that's fine (cap is more specific)
// Missing tags are treated as wildcards (less specific, can handle any value).
func (c *CapUrn) Matches(request *CapUrn) bool {
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
func (c *CapUrn) CanHandle(request *CapUrn) bool {
	return c.Matches(request)
}

// Specificity returns the specificity score for cap matching
// More specific caps have higher scores and are preferred
func (c *CapUrn) Specificity() int {
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
func (c *CapUrn) IsMoreSpecificThan(other *CapUrn) bool {
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
func (c *CapUrn) IsCompatibleWith(other *CapUrn) bool {
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
func (c *CapUrn) WithWildcardTag(key string) *CapUrn {
	if _, exists := c.tags[key]; exists {
		return c.WithTag(key, "*")
	}
	return c
}

// Subset returns a new cap with only specified tags
func (c *CapUrn) Subset(keys []string) *CapUrn {
	newTags := make(map[string]string)
	for _, key := range keys {
		if value, exists := c.tags[key]; exists {
			newTags[key] = value
		}
	}
	return &CapUrn{tags: newTags}
}

// Merge returns a new cap merged with another (other takes precedence for conflicts)
func (c *CapUrn) Merge(other *CapUrn) *CapUrn {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	for k, v := range other.tags {
		newTags[k] = v
	}
	return &CapUrn{tags: newTags}
}

// ToString returns the canonical string representation of this cap identifier
// Always includes "cap:" prefix
// Tags are sorted alphabetically for consistent representation
// No trailing semicolon in canonical form
func (c *CapUrn) ToString() string {
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

	tagsStr := strings.Join(parts, ";")
	return fmt.Sprintf("cap:%s", tagsStr)
}

// String implements the Stringer interface
func (c *CapUrn) String() string {
	return c.ToString()
}

// Equals checks if this cap identifier is equal to another
func (c *CapUrn) Equals(other *CapUrn) bool {
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

// Hash returns a hash of this cap identifier
// Two equivalent cap identifiers will have the same hash
func (c *CapUrn) Hash() string {
	// Use canonical string representation for consistent hashing
	canonical := c.ToString()
	h := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", h)
}

// MarshalJSON implements the json.Marshaler interface
func (c *CapUrn) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.ToString())
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (c *CapUrn) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	capUrn, err := NewCapUrnFromString(s)
	if err != nil {
		return err
	}

	c.tags = capUrn.tags
	return nil
}

// CapMatcher provides utility methods for matching caps
type CapMatcher struct{}

// FindBestMatch finds the most specific cap that can handle a request
func (m *CapMatcher) FindBestMatch(caps []*CapUrn, request *CapUrn) *CapUrn {
	var best *CapUrn
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
func (m *CapMatcher) FindAllMatches(caps []*CapUrn, request *CapUrn) []*CapUrn {
	var matches []*CapUrn

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
func (m *CapMatcher) AreCompatible(caps1, caps2 []*CapUrn) bool {
	for _, c1 := range caps1 {
		for _, c2 := range caps2 {
			if c1.IsCompatibleWith(c2) {
				return true
			}
		}
	}
	return false
}

// CapUrnBuilder provides a fluent builder interface for creating cap URNs
type CapUrnBuilder struct {
	tags map[string]string
}

// NewCapUrnBuilder creates a new builder
func NewCapUrnBuilder() *CapUrnBuilder {
	return &CapUrnBuilder{
		tags: make(map[string]string),
	}
}

// Tag adds or updates a tag
func (b *CapUrnBuilder) Tag(key, value string) *CapUrnBuilder {
	b.tags[key] = value
	return b
}

// Build creates the final CapUrn
func (b *CapUrnBuilder) Build() (*CapUrn, error) {
	if len(b.tags) == 0 {
		return nil, &CapUrnError{
			Code:    ErrorInvalidFormat,
			Message: "cap identifier cannot be empty",
		}
	}

	return NewCapUrnFromTags(b.tags), nil
}