// Package capns provides the fundamental cap URN system used across
// all FGND plugins and providers. It defines the formal structure for cap
// identifiers with flat tag-based naming, wildcard support, and specificity comparison.
package capns

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// CapUrn represents a cap URN using flat, ordered tags with required direction specifiers
//
// Direction (in→out) is integral to a cap's identity. The `inSpec` and `outSpec`
// fields specify the input and output media spec IDs respectively.
//
// Examples:
// - cap:in=std:binary.v1;op=generate;out=std:binary.v1;target=thumbnail
// - cap:in=std:void.v1;op=dimensions;out=std:int.v1
// - cap:in=std:str.v1;out=std:obj.v1;key="Value With Spaces"
type CapUrn struct {
	// inSpec is the input media spec ID - required (use std:void.v1 for caps with no input)
	inSpec string
	// outSpec is the output media spec ID - required
	outSpec string
	// tags are additional tags that define this cap (not including in/out)
	tags map[string]string
}

// CapUrnError represents errors that can occur during cap URN operations
type CapUrnError struct {
	Code    int
	Message string
}

func (e *CapUrnError) Error() string {
	return e.Message
}

// Error codes for cap URN operations
const (
	ErrorInvalidFormat         = 1
	ErrorEmptyTag              = 2
	ErrorInvalidCharacter      = 3
	ErrorInvalidTagFormat      = 4
	ErrorMissingCapPrefix      = 5
	ErrorDuplicateKey          = 6
	ErrorNumericKey            = 7
	ErrorUnterminatedQuote     = 8
	ErrorInvalidEscapeSequence = 9
	ErrorMissingInSpec         = 10
	ErrorMissingOutSpec        = 11
)

// Parser states for state machine
type parseState int

const (
	stateExpectingKey parseState = iota
	stateInKey
	stateExpectingValue
	stateInUnquotedValue
	stateInQuotedValue
	stateInQuotedValueEscape
	stateExpectingSemiOrEnd
)

var numericPattern = regexp.MustCompile(`^[0-9]+$`)

// isValidKeyChar checks if a character is valid for a key
func isValidKeyChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-' || c == '/' || c == ':' || c == '.'
}

// isValidUnquotedValueChar checks if a character is valid for an unquoted value
func isValidUnquotedValueChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-' || c == '/' || c == ':' || c == '.' || c == '*'
}

// needsQuoting checks if a value needs quoting for serialization
func needsQuoting(value string) bool {
	for _, c := range value {
		if c == ';' || c == '=' || c == '"' || c == '\\' || c == ' ' || unicode.IsUpper(c) {
			return true
		}
	}
	return false
}

// quoteValue quotes a value for serialization
func quoteValue(value string) string {
	var result strings.Builder
	result.WriteRune('"')
	for _, c := range value {
		if c == '"' || c == '\\' {
			result.WriteRune('\\')
		}
		result.WriteRune(c)
	}
	result.WriteRune('"')
	return result.String()
}

// NewCapUrnFromString creates a cap URN from a string
// Format: cap:in=spec;out=spec;key1=value1;... or cap:in=spec;key="value";out=spec
// The "cap:" prefix is mandatory
// The 'in' and 'out' tags are REQUIRED (direction is part of cap identity)
// Trailing semicolons are optional and ignored
// Tags are automatically sorted alphabetically for canonical form
//
// Case handling:
// - Keys: Always normalized to lowercase
// - Unquoted values: Normalized to lowercase
// - Quoted values: Case preserved exactly as specified
func NewCapUrnFromString(s string) (*CapUrn, error) {
	if s == "" {
		return nil, &CapUrnError{
			Code:    ErrorInvalidFormat,
			Message: "cap URN cannot be empty",
		}
	}

	// Check for "cap:" prefix (case-insensitive)
	if len(s) < 4 || !strings.EqualFold(s[:4], "cap:") {
		return nil, &CapUrnError{
			Code:    ErrorMissingCapPrefix,
			Message: "cap URN must start with 'cap:'",
		}
	}

	tagsPart := s[4:]
	tags := make(map[string]string)

	// Handle empty cap URN (cap: with no tags) - this is now an error since in/out are required
	if tagsPart == "" || tagsPart == ";" {
		return nil, &CapUrnError{
			Code:    ErrorMissingInSpec,
			Message: "cap URN is missing required 'in' tag - caps must declare their input type (use std:void.v1 for no input)",
		}
	}

	state := stateExpectingKey
	var currentKey strings.Builder
	var currentValue strings.Builder
	chars := []rune(tagsPart)
	pos := 0

	finishTag := func() error {
		key := currentKey.String()
		value := currentValue.String()

		if key == "" {
			return &CapUrnError{
				Code:    ErrorEmptyTag,
				Message: "empty key",
			}
		}
		if value == "" {
			return &CapUrnError{
				Code:    ErrorEmptyTag,
				Message: fmt.Sprintf("empty value for key '%s'", key),
			}
		}

		// Check for duplicate keys
		if _, exists := tags[key]; exists {
			return &CapUrnError{
				Code:    ErrorDuplicateKey,
				Message: fmt.Sprintf("duplicate tag key: %s", key),
			}
		}

		// Validate key cannot be purely numeric
		if numericPattern.MatchString(key) {
			return &CapUrnError{
				Code:    ErrorNumericKey,
				Message: fmt.Sprintf("tag key cannot be purely numeric: %s", key),
			}
		}

		tags[key] = value
		currentKey.Reset()
		currentValue.Reset()
		return nil
	}

	for pos < len(chars) {
		c := chars[pos]

		switch state {
		case stateExpectingKey:
			if c == ';' {
				// Empty segment, skip
				pos++
				continue
			} else if isValidKeyChar(c) {
				currentKey.WriteRune(unicode.ToLower(c))
				state = stateInKey
			} else {
				return nil, &CapUrnError{
					Code:    ErrorInvalidCharacter,
					Message: fmt.Sprintf("invalid character '%c' at position %d", c, pos),
				}
			}

		case stateInKey:
			if c == '=' {
				if currentKey.Len() == 0 {
					return nil, &CapUrnError{
						Code:    ErrorEmptyTag,
						Message: "empty key",
					}
				}
				state = stateExpectingValue
			} else if isValidKeyChar(c) {
				currentKey.WriteRune(unicode.ToLower(c))
			} else {
				return nil, &CapUrnError{
					Code:    ErrorInvalidCharacter,
					Message: fmt.Sprintf("invalid character '%c' in key at position %d", c, pos),
				}
			}

		case stateExpectingValue:
			if c == '"' {
				state = stateInQuotedValue
			} else if c == ';' {
				return nil, &CapUrnError{
					Code:    ErrorEmptyTag,
					Message: fmt.Sprintf("empty value for key '%s'", currentKey.String()),
				}
			} else if isValidUnquotedValueChar(c) {
				currentValue.WriteRune(unicode.ToLower(c))
				state = stateInUnquotedValue
			} else {
				return nil, &CapUrnError{
					Code:    ErrorInvalidCharacter,
					Message: fmt.Sprintf("invalid character '%c' in value at position %d", c, pos),
				}
			}

		case stateInUnquotedValue:
			if c == ';' {
				if err := finishTag(); err != nil {
					return nil, err
				}
				state = stateExpectingKey
			} else if isValidUnquotedValueChar(c) {
				currentValue.WriteRune(unicode.ToLower(c))
			} else {
				return nil, &CapUrnError{
					Code:    ErrorInvalidCharacter,
					Message: fmt.Sprintf("invalid character '%c' in unquoted value at position %d", c, pos),
				}
			}

		case stateInQuotedValue:
			if c == '"' {
				state = stateExpectingSemiOrEnd
			} else if c == '\\' {
				state = stateInQuotedValueEscape
			} else {
				// Any character allowed in quoted value, preserve case
				currentValue.WriteRune(c)
			}

		case stateInQuotedValueEscape:
			if c == '"' || c == '\\' {
				currentValue.WriteRune(c)
				state = stateInQuotedValue
			} else {
				return nil, &CapUrnError{
					Code:    ErrorInvalidEscapeSequence,
					Message: fmt.Sprintf("invalid escape sequence at position %d (only \\\" and \\\\ allowed)", pos),
				}
			}

		case stateExpectingSemiOrEnd:
			if c == ';' {
				if err := finishTag(); err != nil {
					return nil, err
				}
				state = stateExpectingKey
			} else {
				return nil, &CapUrnError{
					Code:    ErrorInvalidCharacter,
					Message: fmt.Sprintf("expected ';' or end after quoted value, got '%c' at position %d", c, pos),
				}
			}
		}

		pos++
	}

	// Handle end of input
	switch state {
	case stateInUnquotedValue, stateExpectingSemiOrEnd:
		if err := finishTag(); err != nil {
			return nil, err
		}
	case stateExpectingKey:
		// Valid - trailing semicolon or empty input after prefix
	case stateInQuotedValue, stateInQuotedValueEscape:
		return nil, &CapUrnError{
			Code:    ErrorUnterminatedQuote,
			Message: fmt.Sprintf("unterminated quote at position %d", pos),
		}
	case stateInKey:
		return nil, &CapUrnError{
			Code:    ErrorInvalidTagFormat,
			Message: fmt.Sprintf("incomplete tag '%s'", currentKey.String()),
		}
	case stateExpectingValue:
		return nil, &CapUrnError{
			Code:    ErrorEmptyTag,
			Message: fmt.Sprintf("empty value for key '%s'", currentKey.String()),
		}
	}

	// Extract required in and out specs
	inSpec, hasIn := tags["in"]
	if !hasIn {
		return nil, &CapUrnError{
			Code:    ErrorMissingInSpec,
			Message: "cap URN is missing required 'in' tag - caps must declare their input type (use std:void.v1 for no input)",
		}
	}
	delete(tags, "in")

	outSpec, hasOut := tags["out"]
	if !hasOut {
		return nil, &CapUrnError{
			Code:    ErrorMissingOutSpec,
			Message: "cap URN is missing required 'out' tag - caps must declare their output type",
		}
	}
	delete(tags, "out")

	return &CapUrn{inSpec: inSpec, outSpec: outSpec, tags: tags}, nil
}

// NewCapUrnFromTags creates a cap URN from tags that must contain 'in' and 'out'
// Keys are normalized to lowercase; values are preserved as-is
// Returns error if 'in' or 'out' tags are missing
func NewCapUrnFromTags(tags map[string]string) (*CapUrn, error) {
	// Normalize keys to lowercase
	result := make(map[string]string)
	for k, v := range tags {
		result[strings.ToLower(k)] = v
	}

	// Extract required in and out specs
	inSpec, hasIn := result["in"]
	if !hasIn {
		return nil, &CapUrnError{
			Code:    ErrorMissingInSpec,
			Message: "cap URN is missing required 'in' tag - caps must declare their input type (use std:void.v1 for no input)",
		}
	}
	delete(result, "in")

	outSpec, hasOut := result["out"]
	if !hasOut {
		return nil, &CapUrnError{
			Code:    ErrorMissingOutSpec,
			Message: "cap URN is missing required 'out' tag - caps must declare their output type",
		}
	}
	delete(result, "out")

	return &CapUrn{inSpec: inSpec, outSpec: outSpec, tags: result}, nil
}

// NewCapUrn creates a cap URN from direction specs and additional tags
// Keys are normalized to lowercase; values are preserved as-is
// inSpec and outSpec are required direction specifiers
func NewCapUrn(inSpec, outSpec string, tags map[string]string) *CapUrn {
	normalizedTags := make(map[string]string)
	for k, v := range tags {
		keyLower := strings.ToLower(k)
		// Ensure in and out are not in tags
		if keyLower != "in" && keyLower != "out" {
			normalizedTags[keyLower] = v
		}
	}
	return &CapUrn{inSpec: inSpec, outSpec: outSpec, tags: normalizedTags}
}

// InSpec returns the input spec ID
func (c *CapUrn) InSpec() string {
	return c.inSpec
}

// OutSpec returns the output spec ID
func (c *CapUrn) OutSpec() string {
	return c.outSpec
}

// GetTag returns the value of a specific tag
// Key is normalized to lowercase for lookup
// For 'in' and 'out', returns the direction spec fields
func (c *CapUrn) GetTag(key string) (string, bool) {
	keyLower := strings.ToLower(key)
	switch keyLower {
	case "in":
		return c.inSpec, true
	case "out":
		return c.outSpec, true
	default:
		value, exists := c.tags[keyLower]
		return value, exists
	}
}

// HasTag checks if this cap has a specific tag with a specific value
// Key is normalized to lowercase; value comparison is case-sensitive
// For 'in' and 'out', checks the direction spec fields
func (c *CapUrn) HasTag(key, value string) bool {
	keyLower := strings.ToLower(key)
	switch keyLower {
	case "in":
		return c.inSpec == value
	case "out":
		return c.outSpec == value
	default:
		tagValue, exists := c.tags[keyLower]
		return exists && tagValue == value
	}
}

// WithTag returns a new cap URN with an added or updated tag
// Key is normalized to lowercase; value is preserved as-is
// Note: Cannot modify 'in' or 'out' tags - use WithInSpec/WithOutSpec
func (c *CapUrn) WithTag(key, value string) *CapUrn {
	keyLower := strings.ToLower(key)
	// Silently ignore attempts to set in/out via WithTag
	// Use WithInSpec/WithOutSpec instead
	if keyLower == "in" || keyLower == "out" {
		return c
	}
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	newTags[keyLower] = value
	return &CapUrn{inSpec: c.inSpec, outSpec: c.outSpec, tags: newTags}
}

// WithInSpec returns a new cap URN with a different input spec
func (c *CapUrn) WithInSpec(inSpec string) *CapUrn {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	return &CapUrn{inSpec: inSpec, outSpec: c.outSpec, tags: newTags}
}

// WithOutSpec returns a new cap URN with a different output spec
func (c *CapUrn) WithOutSpec(outSpec string) *CapUrn {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	return &CapUrn{inSpec: c.inSpec, outSpec: outSpec, tags: newTags}
}

// WithoutTag returns a new cap URN with a tag removed
// Key is normalized to lowercase for case-insensitive removal
// Note: Cannot remove 'in' or 'out' tags - they are required
func (c *CapUrn) WithoutTag(key string) *CapUrn {
	keyLower := strings.ToLower(key)
	// Silently ignore attempts to remove in/out
	if keyLower == "in" || keyLower == "out" {
		return c
	}
	newTags := make(map[string]string)
	for k, v := range c.tags {
		if k != keyLower {
			newTags[k] = v
		}
	}
	return &CapUrn{inSpec: c.inSpec, outSpec: c.outSpec, tags: newTags}
}

// Matches checks if this cap matches another based on tag compatibility
//
// Direction (in/out) is ALWAYS part of matching - they must match exactly or with wildcards.
// For other tags:
// - For each tag in the request: cap has same value, wildcard (*), or missing tag
// - For each tag in the cap: if request is missing that tag, that's fine (cap is more specific)
// Missing tags (except in/out) are treated as wildcards (less specific, can handle any value).
func (c *CapUrn) Matches(request *CapUrn) bool {
	if request == nil {
		return true
	}

	// Direction specs must match (wildcards allowed)
	// Check inSpec
	if c.inSpec != "*" && request.inSpec != "*" && c.inSpec != request.inSpec {
		return false
	}

	// Check outSpec
	if c.outSpec != "*" && request.outSpec != "*" && c.outSpec != request.outSpec {
		return false
	}

	// Check all other tags that the request specifies
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
// Includes direction specs (in/out) in the count
func (c *CapUrn) Specificity() int {
	// Count non-wildcard direction specs
	count := 0
	if c.inSpec != "*" {
		count++
	}
	if c.outSpec != "*" {
		count++
	}
	// Count non-wildcard tags
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
// Direction specs must be compatible (same value or one is wildcard)
func (c *CapUrn) IsCompatibleWith(other *CapUrn) bool {
	if other == nil {
		return true
	}

	// Check inSpec compatibility
	if c.inSpec != "*" && other.inSpec != "*" && c.inSpec != other.inSpec {
		return false
	}

	// Check outSpec compatibility
	if c.outSpec != "*" && other.outSpec != "*" && c.outSpec != other.outSpec {
		return false
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
// For 'in' or 'out', sets the corresponding direction spec to wildcard
func (c *CapUrn) WithWildcardTag(key string) *CapUrn {
	keyLower := strings.ToLower(key)
	switch keyLower {
	case "in":
		return c.WithInSpec("*")
	case "out":
		return c.WithOutSpec("*")
	default:
		if _, exists := c.tags[keyLower]; exists {
			newTags := make(map[string]string)
			for k, v := range c.tags {
				newTags[k] = v
			}
			newTags[keyLower] = "*"
			return &CapUrn{inSpec: c.inSpec, outSpec: c.outSpec, tags: newTags}
		}
		return c
	}
}

// Subset returns a new cap with only specified tags
// Note: 'in' and 'out' are always included as they are required
func (c *CapUrn) Subset(keys []string) *CapUrn {
	newTags := make(map[string]string)
	for _, key := range keys {
		keyLower := strings.ToLower(key)
		// Skip in/out as they're handled separately
		if keyLower == "in" || keyLower == "out" {
			continue
		}
		if value, exists := c.tags[keyLower]; exists {
			newTags[keyLower] = value
		}
	}
	return &CapUrn{inSpec: c.inSpec, outSpec: c.outSpec, tags: newTags}
}

// Merge returns a new cap merged with another (other takes precedence for conflicts)
// Direction specs from other override this one's
func (c *CapUrn) Merge(other *CapUrn) *CapUrn {
	newTags := make(map[string]string)
	for k, v := range c.tags {
		newTags[k] = v
	}
	for k, v := range other.tags {
		newTags[k] = v
	}
	return &CapUrn{inSpec: other.inSpec, outSpec: other.outSpec, tags: newTags}
}

// ToString returns the canonical string representation of this cap URN
// Always includes "cap:" prefix
// All tags (including in/out) are sorted alphabetically
// No trailing semicolon in canonical form
// Values are quoted only when necessary (smart quoting)
func (c *CapUrn) ToString() string {
	// Build a full sorted list of all tags including in/out
	type tagPair struct {
		key   string
		value string
	}
	allTags := make([]tagPair, 0, len(c.tags)+2)

	// Add inSpec and outSpec as tags
	allTags = append(allTags, tagPair{"in", c.inSpec})
	allTags = append(allTags, tagPair{"out", c.outSpec})

	// Add all other tags
	for k, v := range c.tags {
		allTags = append(allTags, tagPair{k, v})
	}

	// Sort alphabetically by key
	sort.Slice(allTags, func(i, j int) bool {
		return allTags[i].key < allTags[j].key
	})

	// Build the string
	parts := make([]string, 0, len(allTags))
	for _, tag := range allTags {
		if needsQuoting(tag.value) {
			parts = append(parts, fmt.Sprintf("%s=%s", tag.key, quoteValue(tag.value)))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", tag.key, tag.value))
		}
	}

	return fmt.Sprintf("cap:%s", strings.Join(parts, ";"))
}

// String implements the Stringer interface
func (c *CapUrn) String() string {
	return c.ToString()
}

// Equals checks if this cap URN is equal to another
func (c *CapUrn) Equals(other *CapUrn) bool {
	if other == nil {
		return false
	}

	// Check direction specs
	if c.inSpec != other.inSpec || c.outSpec != other.outSpec {
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

// Hash returns a hash of this cap URN
// Two equivalent cap URNs will have the same hash
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
		return fmt.Errorf("failed to unmarshal CapUrn: expected string, got: %s", string(data))
	}

	capUrn, err := NewCapUrnFromString(s)
	if err != nil {
		return err
	}

	c.inSpec = capUrn.inSpec
	c.outSpec = capUrn.outSpec
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
// Direction specs (in/out) are required and must be set before building
type CapUrnBuilder struct {
	inSpec  *string
	outSpec *string
	tags    map[string]string
}

// NewCapUrnBuilder creates a new builder
func NewCapUrnBuilder() *CapUrnBuilder {
	return &CapUrnBuilder{
		tags: make(map[string]string),
	}
}

// InSpec sets the input spec ID (required)
func (b *CapUrnBuilder) InSpec(spec string) *CapUrnBuilder {
	b.inSpec = &spec
	return b
}

// OutSpec sets the output spec ID (required)
func (b *CapUrnBuilder) OutSpec(spec string) *CapUrnBuilder {
	b.outSpec = &spec
	return b
}

// Tag adds or updates a tag
// Key is normalized to lowercase; value is preserved as-is
// Note: 'in' and 'out' are ignored here - use InSpec() and OutSpec()
func (b *CapUrnBuilder) Tag(key, value string) *CapUrnBuilder {
	keyLower := strings.ToLower(key)
	if keyLower == "in" || keyLower == "out" {
		return b
	}
	b.tags[keyLower] = value
	return b
}

// Build creates the final CapUrn
func (b *CapUrnBuilder) Build() (*CapUrn, error) {
	if b.inSpec == nil {
		return nil, &CapUrnError{
			Code:    ErrorMissingInSpec,
			Message: "cap URN is missing required 'in' spec - caps must declare their input type (use std:void.v1 for no input)",
		}
	}

	if b.outSpec == nil {
		return nil, &CapUrnError{
			Code:    ErrorMissingOutSpec,
			Message: "cap URN is missing required 'out' spec - caps must declare their output type",
		}
	}

	return &CapUrn{inSpec: *b.inSpec, outSpec: *b.outSpec, tags: b.tags}, nil
}
