// Package capns provides the fundamental cap URN system used across
// all FGND plugins and providers. It defines the formal structure for cap
// identifiers with flat tag-based naming, pattern matching, and graded specificity.
//
// Special pattern values (from tagged-urn):
//   - K=v: Must have key K with exact value v
//   - K=*: Must have key K with any value (presence required)
//   - K=!: Must NOT have key K (absence required)
//   - K=?: No constraint on key K
//   - (missing): Same as K=? - no constraint
//
// Uses TaggedUrn for parsing to ensure consistency across implementations.
package capns

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/filegrind/tagged-urn-go"
)

// CapUrn represents a cap URN using flat, ordered tags with required direction specifiers
//
// Direction (in→out) is integral to a cap's identity. The `inSpec` and `outSpec`
// fields specify the input and output media URNs respectively.
//
// Examples:
// - cap:in="media:binary";op=generate;out="media:binary";target=thumbnail
// - cap:in="media:void";op=dimensions;out="media:integer"
// - cap:in="media:string";key="Value With Spaces";out="media:object"
type CapUrn struct {
	// inSpec is the input media URN - required (use media:void for caps with no input)
	inSpec string
	// outSpec is the output media URN - required
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
	ErrorInvalidMediaUrn       = 12
)

// isValidMediaUrnOrWildcard checks if a value is a valid media URN or wildcard
func isValidMediaUrnOrWildcard(value string) bool {
	return value == "*" || strings.HasPrefix(value, "media:")
}

// Note: needsQuoting and quoteValue are delegated to TaggedUrn

// capUrnErrorFromTaggedUrn converts TaggedUrn errors to CapUrn errors
func capUrnErrorFromTaggedUrn(err error) *CapUrnError {
	if err == nil {
		return nil
	}
	msg := err.Error()
	msgLower := strings.ToLower(msg)

	var code int
	switch {
	case strings.Contains(msgLower, "invalid character"):
		code = ErrorInvalidCharacter
	case strings.Contains(msgLower, "duplicate"):
		code = ErrorDuplicateKey
	case strings.Contains(msgLower, "unterminated") || strings.Contains(msgLower, "unclosed"):
		code = ErrorUnterminatedQuote
	case strings.Contains(msgLower, "expected") && strings.Contains(msgLower, "after quoted"):
		code = ErrorUnterminatedQuote
	case strings.Contains(msgLower, "numeric"):
		code = ErrorNumericKey
	case strings.Contains(msgLower, "escape"):
		code = ErrorInvalidEscapeSequence
	case strings.Contains(msgLower, "incomplete") || strings.Contains(msgLower, "missing value"):
		code = ErrorInvalidTagFormat
	default:
		code = ErrorInvalidFormat
	}

	return &CapUrnError{Code: code, Message: msg}
}

// NewCapUrnFromString creates a cap URN from a string
// Format: cap:in="media:...";out="media:...";key1=value1;...
// The "cap:" prefix is mandatory
// The 'in' and 'out' tags are REQUIRED (direction is part of cap identity)
// The in/out values must be valid media URNs (starting with "media:") or wildcards (*)
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

	// Check for "cap:" prefix early (case-insensitive)
	if len(s) < 4 || !strings.EqualFold(s[:4], "cap:") {
		return nil, &CapUrnError{
			Code:    ErrorMissingCapPrefix,
			Message: "cap URN must start with 'cap:'",
		}
	}

	// Use TaggedUrn for parsing
	taggedUrn, err := taggedurn.NewTaggedUrnFromString(s)
	if err != nil {
		return nil, capUrnErrorFromTaggedUrn(err)
	}

	// Verify prefix is "cap"
	if taggedUrn.GetPrefix() != "cap" {
		return nil, &CapUrnError{
			Code:    ErrorMissingCapPrefix,
			Message: "cap URN must start with 'cap:'",
		}
	}

	// Extract required 'in' tag
	inSpec, hasIn := taggedUrn.GetTag("in")
	if !hasIn || inSpec == "" {
		return nil, &CapUrnError{
			Code:    ErrorMissingInSpec,
			Message: "cap URN is missing required 'in' tag - caps must declare their input type (use media:void for no input)",
		}
	}

	// Validate in is a valid media URN or wildcard
	if !isValidMediaUrnOrWildcard(inSpec) {
		return nil, &CapUrnError{
			Code:    ErrorInvalidMediaUrn,
			Message: fmt.Sprintf("'in' value must be a media URN (starting with 'media:') or wildcard '*', got: %s", inSpec),
		}
	}

	// Extract required 'out' tag
	outSpec, hasOut := taggedUrn.GetTag("out")
	if !hasOut || outSpec == "" {
		return nil, &CapUrnError{
			Code:    ErrorMissingOutSpec,
			Message: "cap URN is missing required 'out' tag - caps must declare their output type",
		}
	}

	// Validate out is a valid media URN or wildcard
	if !isValidMediaUrnOrWildcard(outSpec) {
		return nil, &CapUrnError{
			Code:    ErrorInvalidMediaUrn,
			Message: fmt.Sprintf("'out' value must be a media URN (starting with 'media:') or wildcard '*', got: %s", outSpec),
		}
	}

	// Build tags map without in/out
	tags := make(map[string]string)
	for key, value := range taggedUrn.AllTags() {
		if key != "in" && key != "out" {
			tags[key] = value
		}
	}

	return &CapUrn{inSpec: inSpec, outSpec: outSpec, tags: tags}, nil
}

// NewCapUrnFromTags creates a cap URN from tags that must contain 'in' and 'out'
// Keys are normalized to lowercase; values are preserved as-is
// Returns error if 'in' or 'out' tags are missing or invalid
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
			Message: "cap URN is missing required 'in' tag - caps must declare their input type (use media:void for no input)",
		}
	}
	delete(result, "in")

	// Validate in is a valid media URN or wildcard
	if !isValidMediaUrnOrWildcard(inSpec) {
		return nil, &CapUrnError{
			Code:    ErrorInvalidMediaUrn,
			Message: fmt.Sprintf("'in' value must be a media URN (starting with 'media:') or wildcard '*', got: %s", inSpec),
		}
	}

	outSpec, hasOut := result["out"]
	if !hasOut {
		return nil, &CapUrnError{
			Code:    ErrorMissingOutSpec,
			Message: "cap URN is missing required 'out' tag - caps must declare their output type",
		}
	}
	delete(result, "out")

	// Validate out is a valid media URN or wildcard
	if !isValidMediaUrnOrWildcard(outSpec) {
		return nil, &CapUrnError{
			Code:    ErrorInvalidMediaUrn,
			Message: fmt.Sprintf("'out' value must be a media URN (starting with 'media:') or wildcard '*', got: %s", outSpec),
		}
	}

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

// WithTagValidated adds or updates a tag, rejecting empty values (matches Rust with_tag)
func (c *CapUrn) WithTagValidated(key, value string) (*CapUrn, error) {
	if value == "" {
		return nil, errors.New("tag value cannot be empty")
	}
	return c.WithTag(key, value), nil
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

// Matches checks if this cap (instance) matches a request based on tag compatibility
//
// Direction (in/out) uses `TaggedUrn.Matches()` semantics:
//   - Input: `request_input.Matches(&cap_input)` — does request's data satisfy cap's requirement?
//   - Output: `cap_output.Matches(&request_output)` — does cap's output satisfy what request expects?
//
// For other tags:
//   - (missing) or K=?: no constraint - matches anything
//   - K=!: must-not-have - instance must NOT have this key
//   - K=*: must-have-any - instance must have this key with any value
//   - K=v: must-have-exact - instance must have this key with exact value v
//
// Special values work symmetrically on both instance and pattern sides.
func (c *CapUrn) Matches(request *CapUrn) bool {
	if request == nil {
		return true
	}

	// Direction specs: TaggedUrn semantic matching
	// Check in_urn: request's input must satisfy cap's input requirement
	if c.inSpec != "*" && request.inSpec != "*" {
		capIn, err := taggedurn.NewTaggedUrnFromString(c.inSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: cap in_spec '%s' is not a valid media URN: %v", c.inSpec, err))
		}
		requestIn, err := taggedurn.NewTaggedUrnFromString(request.inSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: request in_spec '%s' is not a valid media URN: %v", request.inSpec, err))
		}
		matches, err := requestIn.Matches(capIn)
		if err != nil {
			panic(fmt.Sprintf("CU2: media URN prefix mismatch in direction spec matching: %v", err))
		}
		if !matches {
			return false
		}
	}

	// Check out_urn: cap's output must satisfy what the request expects
	if c.outSpec != "*" && request.outSpec != "*" {
		capOut, err := taggedurn.NewTaggedUrnFromString(c.outSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: cap out_spec '%s' is not a valid media URN: %v", c.outSpec, err))
		}
		requestOut, err := taggedurn.NewTaggedUrnFromString(request.outSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: request out_spec '%s' is not a valid media URN: %v", request.outSpec, err))
		}
		matches, err := capOut.Matches(requestOut)
		if err != nil {
			panic(fmt.Sprintf("CU2: media URN prefix mismatch in direction spec matching: %v", err))
		}
		if !matches {
			return false
		}
	}

	// Collect all tag keys from both instance and pattern
	allKeys := make(map[string]bool)
	for key := range c.tags {
		allKeys[key] = true
	}
	for key := range request.tags {
		allKeys[key] = true
	}

	// Check each tag using the valuesMatch logic
	for key := range allKeys {
		instVal, instExists := c.tags[key]
		pattVal, pattExists := request.tags[key]

		var inst, patt *string
		if instExists {
			inst = &instVal
		}
		if pattExists {
			patt = &pattVal
		}

		if !valuesMatch(inst, patt) {
			return false
		}
	}
	return true
}

// valuesMatch checks if instance value matches pattern constraint
// Uses the same semantics as tagged-urn matching
func valuesMatch(inst, patt *string) bool {
	// Pattern has no constraint (no entry or explicit ?)
	if patt == nil || *patt == "?" {
		return true
	}

	// Instance doesn't care (explicit ?)
	if inst != nil && *inst == "?" {
		return true
	}

	// Pattern: must-not-have (!)
	if *patt == "!" {
		if inst == nil {
			return true // Instance absent, pattern wants absent
		}
		if *inst == "!" {
			return true // Both say absent
		}
		return false // Instance has value, pattern wants absent
	}

	// Instance: must-not-have conflicts with pattern wanting value
	if inst != nil && *inst == "!" {
		return false // Conflict: absent vs value or present
	}

	// Pattern: must-have-any (*)
	if *patt == "*" {
		if inst == nil {
			return false // Instance missing, pattern wants present
		}
		return true // Instance has value, pattern wants any
	}

	// Pattern: exact value
	if inst == nil {
		// Cap (inst) is missing this tag - treat as wildcard (can handle any value)
		// This matches Rust semantics: missing tag in cap = wildcard
		// Enables fallback scenarios where generic cap handles specific requests
		return true
	}
	if *inst == "*" {
		return true // Instance accepts any, pattern's value is fine
	}
	return *inst == *patt // Both have values, must match exactly
}

// CanHandle checks if this cap can handle a request
func (c *CapUrn) CanHandle(request *CapUrn) bool {
	return c.Matches(request)
}

// Specificity returns the specificity score for cap matching.
// More specific caps have higher scores and are preferred.
//
// Direction specs contribute their media URN tag count (more tags = more specific).
// Other tags use graded scoring:
//   - Exact value (K=v): 3 points (most specific)
//   - Must-have-any (K=*): 2 points
//   - Must-not-have (K=!): 1 point
//   - Unspecified (K=?) or missing: 0 points (least specific)
func (c *CapUrn) Specificity() int {
	score := 0
	// Direction specs contribute their media URN tag count
	if c.inSpec != "*" && c.inSpec != "?" {
		inParsed, err := taggedurn.NewTaggedUrnFromString(c.inSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: in_spec '%s' is not a valid media URN: %v", c.inSpec, err))
		}
		score += len(inParsed.AllTags())
	}
	if c.outSpec != "*" && c.outSpec != "?" {
		outParsed, err := taggedurn.NewTaggedUrnFromString(c.outSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: out_spec '%s' is not a valid media URN: %v", c.outSpec, err))
		}
		score += len(outParsed.AllTags())
	}
	// Score other tags using graded scoring
	for _, value := range c.tags {
		score += valueScore(value)
	}
	return score
}

// valueScore returns the graded specificity score for a single value
func valueScore(value string) int {
	switch value {
	case "?":
		return 0
	case "!":
		return 1
	case "*":
		return 2
	default:
		return 3 // exact value
	}
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
// Direction specs are compatible if either is a subtype of the other via TaggedUrn matching
func (c *CapUrn) IsCompatibleWith(other *CapUrn) bool {
	if other == nil {
		return true
	}

	// Check inSpec compatibility: either direction of Matches succeeds
	if c.inSpec != "*" && other.inSpec != "*" {
		selfIn, err := taggedurn.NewTaggedUrnFromString(c.inSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: self in_spec '%s' is not a valid media URN: %v", c.inSpec, err))
		}
		otherIn, err := taggedurn.NewTaggedUrnFromString(other.inSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: other in_spec '%s' is not a valid media URN: %v", other.inSpec, err))
		}
		fwd, err := selfIn.Matches(otherIn)
		if err != nil {
			panic(fmt.Sprintf("CU2: media URN prefix mismatch in direction spec compatibility: %v", err))
		}
		rev, err := otherIn.Matches(selfIn)
		if err != nil {
			panic(fmt.Sprintf("CU2: media URN prefix mismatch in direction spec compatibility: %v", err))
		}
		if !fwd && !rev {
			return false
		}
	}

	// Check outSpec compatibility
	if c.outSpec != "*" && other.outSpec != "*" {
		selfOut, err := taggedurn.NewTaggedUrnFromString(c.outSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: self out_spec '%s' is not a valid media URN: %v", c.outSpec, err))
		}
		otherOut, err := taggedurn.NewTaggedUrnFromString(other.outSpec)
		if err != nil {
			panic(fmt.Sprintf("CU2: other out_spec '%s' is not a valid media URN: %v", other.outSpec, err))
		}
		fwd, err := selfOut.Matches(otherOut)
		if err != nil {
			panic(fmt.Sprintf("CU2: media URN prefix mismatch in direction spec compatibility: %v", err))
		}
		rev, err := otherOut.Matches(selfOut)
		if err != nil {
			panic(fmt.Sprintf("CU2: media URN prefix mismatch in direction spec compatibility: %v", err))
		}
		if !fwd && !rev {
			return false
		}
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
// Uses TaggedUrn for serialization to ensure consistency across implementations
func (c *CapUrn) ToString() string {
	// Build complete tags map including in and out
	allTags := make(map[string]string, len(c.tags)+2)
	allTags["in"] = c.inSpec
	allTags["out"] = c.outSpec
	for k, v := range c.tags {
		allTags[k] = v
	}

	// Use TaggedUrn for serialization
	taggedUrn := taggedurn.NewTaggedUrnFromTags("cap", allTags)
	return taggedUrn.ToString()
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
			Message: "cap URN is missing required 'in' spec - caps must declare their input type (use media:void for no input)",
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
