package capns

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All cap URNs now require in and out specs. Use these helpers:
// Media URNs must be quoted in cap URNs because they contain semicolons
// Use proper tags for is_binary/is_json/is_text detection
func testUrn(tags string) string {
	// Use MediaObject constant for consistent canonical form
	if tags == "" {
		return `cap:in="media:void";out="` + MediaObject + `"`
	}
	return `cap:in="media:void";out="` + MediaObject + `";` + tags
}

func testUrnWithIO(inSpec, outSpec, tags string) string {
	// Media URNs need quoting because they contain semicolons
	if tags == "" {
		return `cap:in="` + inSpec + `";out="` + outSpec + `"`
	}
	return `cap:in="` + inSpec + `";out="` + outSpec + `";` + tags
}

// TEST001: Test that cap URN is created with tags parsed correctly and direction specs accessible
func TestCapUrnCreation(t *testing.T) {
	// Use key=value pairs instead of flags
	capUrn, err := NewCapUrnFromString(testUrn("op=transform;format=json;type=data_processing"))

	assert.NoError(t, err)
	assert.NotNil(t, capUrn)

	capType, exists := capUrn.GetTag("type")
	assert.True(t, exists)
	assert.Equal(t, "data_processing", capType)

	op, exists := capUrn.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "transform", op)

	format, exists := capUrn.GetTag("format")
	assert.True(t, exists)
	assert.Equal(t, "json", format)

	// Direction specs are required and accessible
	assert.Equal(t, MediaVoid, capUrn.InSpec())
	assert.Equal(t, MediaObject, capUrn.OutSpec())
}

// TEST002: Test that missing 'in' spec fails with MissingInSpec, missing 'out' fails with MissingOutSpec
func TestDirectionSpecsRequired(t *testing.T) {
	// Missing 'in' should fail
	_, err := NewCapUrnFromString(`cap:out="media:object";op=test`)
	assert.Error(t, err)
	assert.Equal(t, ErrorMissingInSpec, err.(*CapUrnError).Code)

	// Missing 'out' should fail
	_, err = NewCapUrnFromString(`cap:in="media:void";op=test`)
	assert.Error(t, err)
	assert.Equal(t, ErrorMissingOutSpec, err.(*CapUrnError).Code)

	// Both present should succeed
	_, err = NewCapUrnFromString(`cap:in="media:void";out="media:object";op=test`)
	assert.NoError(t, err)
}

// TEST003: Test that direction specs must match exactly, different in/out types don't match, wildcard matches any
func TestDirectionMatching(t *testing.T) {
	// Direction specs must match for caps to match
	cap1, err := NewCapUrnFromString(`cap:in="media:string";out="media:object";op=test`)
	require.NoError(t, err)
	cap2, err := NewCapUrnFromString(`cap:in="media:string";out="media:object";op=test`)
	require.NoError(t, err)
	assert.True(t, cap1.Accepts(cap2))

	// Different inSpec should not match
	cap3, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap3))

	// Different outSpec should not match
	cap4, err := NewCapUrnFromString(`cap:in="media:string";out="media:integer";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap4))

	// Wildcard in direction should match
	cap5, err := NewCapUrnFromString(`cap:in=*;out="media:object";op=test`)
	require.NoError(t, err)
	assert.True(t, cap1.Accepts(cap5))
	assert.True(t, cap5.Accepts(cap1))
}

func TestCanonicalStringFormat(t *testing.T) {
	capUrn, err := NewCapUrnFromString(testUrn("op=generate;target=thumbnail;ext=pdf"))
	require.NoError(t, err)

	// Should be sorted alphabetically with in/out in their sorted positions
	// Media URNs with semicolons (like MediaObject) need quoting, but simple ones (like MediaVoid) don't
	// Alphabetical order: ext < in < op < out < target
	assert.Equal(t, `cap:ext=pdf;in=`+MediaVoid+`;op=generate;out="`+MediaObject+`";target=thumbnail`, capUrn.ToString())
}

// TEST015: Test that cap: prefix is required and case-insensitive
func TestCapPrefixRequired(t *testing.T) {
	// Missing cap: prefix should fail
	capUrn, err := NewCapUrnFromString(`in="media:void";out="media:object";op=generate`)
	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorMissingCapPrefix, err.(*CapUrnError).Code)

	// Valid cap: prefix should work
	capUrn, err = NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	assert.NoError(t, err)
	assert.NotNil(t, capUrn)
	op, exists := capUrn.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	// Case-insensitive prefix
	capUrn, err = NewCapUrnFromString(`CAP:in="media:void";out="media:object";op=generate`)
	assert.NoError(t, err)
	op, exists = capUrn.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)
}

// TEST016: Test that trailing semicolon is equivalent (same hash, same string, matches)
func TestTrailingSemicolonEquivalence(t *testing.T) {
	// Both with and without trailing semicolon should be equivalent
	cap1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf") + ";")
	require.NoError(t, err)

	// They should be equal
	assert.True(t, cap1.Equals(cap2))

	// They should have same string representation (canonical form)
	assert.Equal(t, cap1.ToString(), cap2.ToString())

	// They should match each other
	assert.True(t, cap1.Accepts(cap2))
	assert.True(t, cap2.Accepts(cap1))
}

func TestInvalidCapUrn(t *testing.T) {
	capUrn, err := NewCapUrnFromString("")

	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidFormat, err.(*CapUrnError).Code)
}

func TestValuelessTagWithMissingSpecs(t *testing.T) {
	// Value-less tag is now valid (parsed as wildcard), but cap URN still requires in/out specs
	capUrn, err := NewCapUrnFromString("cap:optimize")

	assert.Nil(t, capUrn)
	assert.Error(t, err)
	// Should fail because of missing 'in' spec, not invalid tag format
	assert.Equal(t, ErrorMissingInSpec, err.(*CapUrnError).Code)
}

func TestInvalidCharacters(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:type@invalid=value")

	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidCharacter, err.(*CapUrnError).Code)
}

// TEST017: Test tag matching: exact match, subset match, wildcard match, value mismatch
func TestTagMatching(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;target=thumbnail"))
	require.NoError(t, err)

	// Exact match
	request1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;target=thumbnail"))
	require.NoError(t, err)
	assert.True(t, cap.Accepts(request1))

	// Subset match (other tags)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap.Accepts(request2))

	// Wildcard request should match specific cap
	request3, err := NewCapUrnFromString(testUrn("ext=*"))
	require.NoError(t, err)
	assert.True(t, cap.Accepts(request3))

	// No match - conflicting value
	request4, err := NewCapUrnFromString(testUrn("op=extract"))
	require.NoError(t, err)
	assert.False(t, cap.Accepts(request4))
}

// TEST019: Test that missing tags are NOT wildcards (cap without tag does NOT match request with exact value)
func TestMissingTagHandling(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	// Cap missing ext tag → NOT a wildcard → NO MATCH
	// Cap must explicitly declare ext=* to accept any extension
	request1, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)
	assert.False(t, cap.Accepts(request1)) // cap missing ext, request wants ext=pdf → NO MATCH

	// Cap with extra tags can match subset requests (pattern missing = no constraint)
	cap2, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Accepts(request2)) // pattern doesn't constrain ext, so cap with ext=pdf matches

	// Cap with explicit wildcard matches pattern with specific value
	cap3, err := NewCapUrnFromString(testUrn("ext=*;op=generate"))
	require.NoError(t, err)
	request3, err := NewCapUrnFromString(testUrn("ext=pdf;op=generate"))
	require.NoError(t, err)
	assert.True(t, cap3.Accepts(request3)) // cap has ext=*, pattern has ext=pdf → MATCH
}

// TEST020: Test specificity calculation (direction specs use MediaUrn tag count, wildcards don't count)
func TestSpecificity(t *testing.T) {
	// Direction specs contribute their MediaUrn tag count:
	// MEDIA_VOID = "media:void" -> 1 tag (void)
	// MEDIA_OBJECT = "media:form=map;textable" -> 2 tags (form, textable)
	// Other tags use graded scoring:
	// - Exact value (K=v): 3 points
	// - Must-have-any (K=*): 2 points
	// - Must-not-have (K=!): 1 point
	// - Unspecified (K=?) or missing: 0 points
	cap1, err := NewCapUrnFromString(testUrn("type=general"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	cap3, err := NewCapUrnFromString(testUrn("op=*;ext=pdf"))
	require.NoError(t, err)

	// cap1: void(1) + object(2) + type(3) = 6
	assert.Equal(t, 6, cap1.Specificity())
	// cap2: void(1) + object(2) + op(3) = 6
	assert.Equal(t, 6, cap2.Specificity())
	// cap3: void(1) + object(2) + op(2 for *) + ext(3) = 8
	assert.Equal(t, 8, cap3.Specificity())

	// Wildcard in direction doesn't count
	cap4, err := NewCapUrnFromString(`cap:in=*;out="` + MediaObject + `";op=test`)
	require.NoError(t, err)
	// cap4: object(2) + op(3) = 5 (in wildcard doesn't count)
	assert.Equal(t, 5, cap4.Specificity())
}

// TEST024: Test compatibility via directional Accepts (bidirectional)
func TestCompatibility(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate;format=*"))
	require.NoError(t, err)

	cap3, err := NewCapUrnFromString(testUrn("type=image;op=extract"))
	require.NoError(t, err)

	// cap1 and cap2 have non-overlapping required tags: neither direction accepts
	assert.False(t, cap1.Accepts(cap2))
	assert.False(t, cap2.Accepts(cap1))

	// cap1 and cap3: different op values, neither accepts
	assert.False(t, cap1.Accepts(cap3))
	assert.False(t, cap3.Accepts(cap1))

	// General pattern (fewer tags) accepts specific instance
	cap4, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	// cap1 (more specific) satisfies cap4 (general pattern): cap1.Accepts(cap4) = true
	assert.True(t, cap1.Accepts(cap4))
	// cap4 (general) does NOT satisfy cap1 (specific pattern requires ext=pdf): false
	assert.False(t, cap4.Accepts(cap1))

	// Different direction specs: neither accepts
	cap5, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=generate`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap5))
	assert.False(t, cap5.Accepts(cap1))
}

func TestConvenienceMethods(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;output=binary;target=thumbnail"))
	require.NoError(t, err)

	op, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	target, exists := cap.GetTag("target")
	assert.True(t, exists)
	assert.Equal(t, "thumbnail", target)

	format, exists := cap.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", format)

	output, exists := cap.GetTag("output")
	assert.True(t, exists)
	assert.Equal(t, "binary", output)

	// GetTag works for in/out
	inVal, exists := cap.GetTag("in")
	assert.True(t, exists)
	assert.Equal(t, MediaVoid, inVal)

	outVal, exists := cap.GetTag("out")
	assert.True(t, exists)
	assert.Equal(t, MediaObject, outVal)
}

// TEST021: Test builder creates cap URN with correct tags and direction specs
func TestBuilder(t *testing.T) {
	cap, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("op", "generate").
		Tag("target", "thumbnail").
		Tag("ext", "pdf").
		Build()
	require.NoError(t, err)

	op, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	assert.Equal(t, MediaVoid, cap.InSpec())
	assert.Equal(t, MediaObject, cap.OutSpec())
}

// TEST022: Test builder requires both in_spec and out_spec
func TestBuilderRequiresDirection(t *testing.T) {
	// Missing inSpec should fail
	_, err := NewCapUrnBuilder().
		OutSpec(MediaObject).
		Tag("op", "test").
		Build()
	assert.Error(t, err)

	// Missing outSpec should fail
	_, err = NewCapUrnBuilder().
		InSpec(MediaVoid).
		Tag("op", "test").
		Build()
	assert.Error(t, err)

	// Both present should succeed
	_, err = NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Build()
	assert.NoError(t, err)
}

func TestWithTag(t *testing.T) {
	original, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	modified := original.WithTag("ext", "pdf")

	// Original should be unchanged (no ext)
	_, origExists := original.GetTag("ext")
	assert.False(t, origExists)

	// Modified should have ext
	ext, modExists := modified.GetTag("ext")
	assert.True(t, modExists)
	assert.Equal(t, "pdf", ext)

	// Direction specs preserved (testUrn uses constants with tags)
	assert.Equal(t, MediaVoid, modified.InSpec())
	assert.Equal(t, MediaObject, modified.OutSpec())
}

func TestWithoutTag(t *testing.T) {
	original, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	modified := original.WithoutTag("ext")

	// Modified should not have ext
	_, exists := modified.GetTag("ext")
	assert.False(t, exists)

	// Original should still have ext
	ext, origExists := original.GetTag("ext")
	assert.True(t, origExists)
	assert.Equal(t, "pdf", ext)

	// Direction specs preserved (testUrn uses constants with tags)
	assert.Equal(t, MediaVoid, modified.InSpec())
	assert.Equal(t, MediaObject, modified.OutSpec())
}

func TestWithInSpecOutSpec(t *testing.T) {
	original, err := NewCapUrnFromString(testUrn("op=test"))
	require.NoError(t, err)

	// Change input spec (using constant with coercion tags)
	modified1 := original.WithInSpec(MediaBinary)
	assert.Equal(t, MediaBinary, modified1.InSpec())
	assert.Equal(t, MediaObject, modified1.OutSpec()) // testUrn uses MediaObject
	// Original unchanged
	assert.Equal(t, MediaVoid, original.InSpec())

	// Change output spec (using constant with coercion tags)
	modified2 := original.WithOutSpec(MediaInteger)
	assert.Equal(t, MediaVoid, modified2.InSpec()) // testUrn uses MediaVoid
	assert.Equal(t, MediaInteger, modified2.OutSpec())
}

// TEST027: Test with_wildcard_tag sets tag to wildcard, including in/out
func TestWildcardTag(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)

	wildcarded := cap.WithWildcardTag("ext")
	ext, exists := wildcarded.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "*", ext)

	// Test wildcarding in/out
	wildcardIn := cap.WithWildcardTag("in")
	assert.Equal(t, "*", wildcardIn.InSpec())

	wildcardOut := cap.WithWildcardTag("out")
	assert.Equal(t, "*", wildcardOut.OutSpec())
}

// TEST026: Test merge combines tags from both caps, subset keeps only specified tags
func TestSubset(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;output=binary;target=thumbnail"))
	require.NoError(t, err)

	subset := cap.Subset([]string{"type", "ext"})

	// Only ext should be in subset (type doesn't exist)
	ext, exists := subset.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)

	_, opExists := subset.GetTag("op")
	assert.False(t, opExists)

	// Direction specs preserved (testUrn uses constants with tags)
	assert.Equal(t, MediaVoid, subset.InSpec())
	assert.Equal(t, MediaObject, subset.OutSpec())
}

// TEST026: Test merge combines tags from both caps, subset keeps only specified tags
func TestMerge(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(`cap:in="media:binary";out="media:integer";ext=pdf;output=binary`)
	require.NoError(t, err)

	merged := cap1.Merge(cap2)

	// Merged takes in/out from cap2 (parsed values without coercion tags)
	assert.Equal(t, "media:binary", merged.InSpec())
	assert.Equal(t, "media:integer", merged.OutSpec())

	// Has tags from both
	op, _ := merged.GetTag("op")
	assert.Equal(t, "generate", op)
	ext, _ := merged.GetTag("ext")
	assert.Equal(t, "pdf", ext)
}

func TestEquality(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	cap3, err := NewCapUrnFromString(testUrn("op=generate;image"))
	require.NoError(t, err)

	assert.True(t, cap1.Equals(cap2))
	assert.False(t, cap1.Equals(cap3))

	// Different direction specs means not equal
	cap4, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=generate`)
	require.NoError(t, err)
	assert.False(t, cap1.Equals(cap4))
}

// TEST025: Test find_best_match returns most specific matching cap
func TestCapMatcher(t *testing.T) {
	matcher := &CapMatcher{}

	caps := []*CapUrn{}

	cap1, err := NewCapUrnFromString(testUrn("op=*"))
	require.NoError(t, err)
	caps = append(caps, cap1)

	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	caps = append(caps, cap2)

	cap3, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)
	caps = append(caps, cap3)

	request, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	best := matcher.FindBestMatch(caps, request)
	require.NotNil(t, best)

	// Most specific cap that can handle the request
	ext, exists := best.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)
}

func TestJSONSerialization(t *testing.T) {
	original, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	var decoded CapUrn
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}

// TEST004: Test that unquoted keys and values are normalized to lowercase
func TestUnquotedValuesLowercased(t *testing.T) {
	// Unquoted values are normalized to lowercase
	cap, err := NewCapUrnFromString(testUrn("OP=Generate;EXT=PDF;Target=Thumbnail"))
	require.NoError(t, err)

	// Keys are always lowercase
	op, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	ext, exists := cap.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)

	target, exists := cap.GetTag("target")
	assert.True(t, exists)
	assert.Equal(t, "thumbnail", target)

	// Key lookup is case-insensitive
	op2, exists := cap.GetTag("OP")
	assert.True(t, exists)
	assert.Equal(t, "generate", op2)
}

// TEST005: Test that quoted values preserve case while unquoted are lowercased
func TestQuotedValuesPreserveCase(t *testing.T) {
	// Quoted values preserve their case
	cap, err := NewCapUrnFromString(testUrn(`key="Value With Spaces"`))
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "Value With Spaces", value)

	// Key is still lowercase
	cap2, err := NewCapUrnFromString(testUrn(`KEY="Value With Spaces"`))
	require.NoError(t, err)
	value2, exists := cap2.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "Value With Spaces", value2)

	// Unquoted vs quoted case difference
	unquoted, err := NewCapUrnFromString(testUrn("key=UPPERCASE"))
	require.NoError(t, err)
	quoted, err := NewCapUrnFromString(testUrn(`key="UPPERCASE"`))
	require.NoError(t, err)

	unquotedVal, _ := unquoted.GetTag("key")
	quotedVal, _ := quoted.GetTag("key")
	assert.Equal(t, "uppercase", unquotedVal) // lowercase
	assert.Equal(t, "UPPERCASE", quotedVal)   // preserved
	assert.False(t, unquoted.Equals(quoted))  // NOT equal
}

// TEST006: Test that quoted values can contain special characters (semicolons, equals, spaces)
func TestQuotedValueSpecialChars(t *testing.T) {
	// Semicolons in quoted values
	cap, err := NewCapUrnFromString(testUrn(`key="value;with;semicolons"`))
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "value;with;semicolons", value)

	// Equals in quoted values
	cap2, err := NewCapUrnFromString(testUrn(`key="value=with=equals"`))
	require.NoError(t, err)
	value2, exists := cap2.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "value=with=equals", value2)

	// Spaces in quoted values
	cap3, err := NewCapUrnFromString(testUrn(`key="hello world"`))
	require.NoError(t, err)
	value3, exists := cap3.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "hello world", value3)
}

// TEST007: Test that escape sequences in quoted values (\" and \\) are parsed correctly
func TestQuotedValueEscapeSequences(t *testing.T) {
	// Escaped quotes
	cap, err := NewCapUrnFromString(testUrn(`key="value\"quoted\""`))
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `value"quoted"`, value)

	// Escaped backslashes
	cap2, err := NewCapUrnFromString(testUrn(`key="path\\file"`))
	require.NoError(t, err)
	value2, exists := cap2.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `path\file`, value2)

	// Mixed escapes
	cap3, err := NewCapUrnFromString(testUrn(`key="say \"hello\\world\""`))
	require.NoError(t, err)
	value3, exists := cap3.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `say "hello\world"`, value3)
}

// TEST008: Test that mixed quoted and unquoted values in same URN parse correctly
func TestMixedQuotedUnquoted(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`a="Quoted";b=simple`))
	require.NoError(t, err)

	a, exists := cap.GetTag("a")
	assert.True(t, exists)
	assert.Equal(t, "Quoted", a)

	b, exists := cap.GetTag("b")
	assert.True(t, exists)
	assert.Equal(t, "simple", b)
}

// TEST009: Test that unterminated quote produces UnterminatedQuote error
func TestUnterminatedQuoteError(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="unterminated`))
	assert.Nil(t, cap)
	assert.Error(t, err)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorUnterminatedQuote, capError.Code)
}

// TEST010: Test that invalid escape sequences (like \n, \x) produce InvalidEscapeSequence error
func TestInvalidEscapeSequenceError(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="bad\n"`))
	assert.Nil(t, cap)
	assert.Error(t, err)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorInvalidEscapeSequence, capError.Code)

	// Invalid escape at end
	cap2, err := NewCapUrnFromString(testUrn(`key="bad\x"`))
	assert.Nil(t, cap2)
	assert.Error(t, err)
	capError2, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorInvalidEscapeSequence, capError2.Code)
}

// TEST011: Test that serialization uses smart quoting (no quotes for simple lowercase, quotes for special chars/uppercase)
func TestSerializationSmartQuoting(t *testing.T) {
	// Simple lowercase value - no quoting needed (but media URNs in in/out are quoted)
	// MediaVoid has no coercion tags (no quotes needed), MediaObject has ;textable;form=map (quotes needed)
	cap, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("key", "simple").
		Build()
	require.NoError(t, err)
	assert.Equal(t, `cap:in=media:void;key=simple;out="`+MediaObject+`"`, cap.ToString())

	// Value with spaces - needs quoting
	cap2, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("key", "has spaces").
		Build()
	require.NoError(t, err)
	assert.Equal(t, `cap:in=media:void;key="has spaces";out="`+MediaObject+`"`, cap2.ToString())

	// Value with uppercase - needs quoting to preserve
	cap4, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("key", "HasUpper").
		Build()
	require.NoError(t, err)
	assert.Equal(t, `cap:in=media:void;key="HasUpper";out="`+MediaObject+`"`, cap4.ToString())
}

// TEST012: Test that simple cap URN round-trips (parse -> serialize -> parse equals original)
func TestRoundTripSimple(t *testing.T) {
	original := testUrn("op=generate;ext=pdf")
	cap, err := NewCapUrnFromString(original)
	require.NoError(t, err)
	serialized := cap.ToString()
	reparsed, err := NewCapUrnFromString(serialized)
	require.NoError(t, err)
	assert.True(t, cap.Equals(reparsed))
}

// TEST013: Test that quoted values round-trip preserving case and spaces
func TestRoundTripQuoted(t *testing.T) {
	original := testUrn(`key="Value With Spaces"`)
	cap, err := NewCapUrnFromString(original)
	require.NoError(t, err)
	serialized := cap.ToString()
	reparsed, err := NewCapUrnFromString(serialized)
	require.NoError(t, err)
	assert.True(t, cap.Equals(reparsed))
	value, exists := reparsed.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "Value With Spaces", value)
}

// TEST014: Test that escape sequences round-trip correctly
func TestRoundTripEscapes(t *testing.T) {
	original := testUrn(`key="value\"with\\escapes"`)
	cap, err := NewCapUrnFromString(original)
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `value"with\escapes`, value)
	serialized := cap.ToString()
	reparsed, err := NewCapUrnFromString(serialized)
	require.NoError(t, err)
	assert.True(t, cap.Equals(reparsed))
}

// TEST018: Test that quoted values with different case do NOT match (case-sensitive)
func TestMatchingCaseSensitiveValues(t *testing.T) {
	// Values with different case should NOT match
	cap1, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)
	cap2, err := NewCapUrnFromString(testUrn(`key="value"`))
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap2))
	assert.False(t, cap2.Accepts(cap1))

	// Same case should match
	cap3, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)
	assert.True(t, cap1.Accepts(cap3))
}

// TEST023: Test builder lowercases keys but preserves value case
func TestBuilderPreservesCase(t *testing.T) {
	cap, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("KEY", "ValueWithCase").
		Build()
	require.NoError(t, err)

	// Key is lowercase
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "ValueWithCase", value)
}

// TEST035: Test has_tag is case-sensitive for values, case-insensitive for keys, works for in/out
func TestHasTagCaseSensitive(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)

	// Exact case match works
	assert.True(t, cap.HasTag("key", "Value"))

	// Different case does not match
	assert.False(t, cap.HasTag("key", "value"))
	assert.False(t, cap.HasTag("key", "VALUE"))

	// Key lookup is case-insensitive
	assert.True(t, cap.HasTag("KEY", "Value"))
	assert.True(t, cap.HasTag("Key", "Value"))

	// HasTag works for in/out (testUrn uses constants with tags)
	assert.True(t, cap.HasTag("in", MediaVoid))
	assert.True(t, cap.HasTag("out", MediaObject))
}

// TEST036: Test with_tag preserves value case
func TestWithTagPreservesValue(t *testing.T) {
	cap := NewCapUrn(MediaVoid, MediaObject, map[string]string{})
	modified := cap.WithTag("key", "ValueWithCase")

	value, exists := modified.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "ValueWithCase", value)
}

// TEST037: Test with_tag rejects empty value
func TestWithTagRejectsEmptyValue(t *testing.T) {
	cap := NewCapUrn(MediaVoid, MediaObject, map[string]string{})
	modified, err := cap.WithTagValidated("key", "")
	assert.Error(t, err, "with_tag should reject empty value")
	assert.Nil(t, modified)
}

// TEST038: Test semantic equivalence of unquoted and quoted simple lowercase values
func TestSemanticEquivalence(t *testing.T) {
	// Unquoted and quoted simple lowercase values are equivalent
	unquoted, err := NewCapUrnFromString(testUrn("key=simple"))
	require.NoError(t, err)
	quoted, err := NewCapUrnFromString(testUrn(`key="simple"`))
	require.NoError(t, err)
	assert.True(t, unquoted.Equals(quoted))
}

// TEST028: Test empty cap URN fails with MissingInSpec
func TestEmptyCapUrnNotAllowed(t *testing.T) {
	// Empty cap URN is no longer valid since in/out are required
	result, err := NewCapUrnFromString("cap:")
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Equal(t, ErrorMissingInSpec, err.(*CapUrnError).Code)

	// With trailing semicolon - still fails
	result, err = NewCapUrnFromString("cap:;")
	assert.Nil(t, result)
	assert.Error(t, err)
}

// TEST029: Test minimal valid cap URN has just in and out, empty tags
func TestMinimalCapUrn(t *testing.T) {
	// Minimal valid cap URN has just in and out
	cap, err := NewCapUrnFromString(`cap:in="media:void";out="media:object"`)
	require.NoError(t, err)
	// InSpec and OutSpec return actual values from parsed string
	assert.Equal(t, "media:void", cap.InSpec())
	assert.Equal(t, "media:object", cap.OutSpec())
	assert.Equal(t, 0, len(cap.tags))
}

// TEST030: Test extended characters (forward slashes, colons) in tag values
func TestExtendedCharacterSupport(t *testing.T) {
	// Test forward slashes and colons in tag components
	cap, err := NewCapUrnFromString(testUrn("url=https://example_org/api;path=/some/file"))
	assert.NoError(t, err)
	assert.NotNil(t, cap)

	url, exists := cap.GetTag("url")
	assert.True(t, exists)
	assert.Equal(t, "https://example_org/api", url)

	path, exists := cap.GetTag("path")
	assert.True(t, exists)
	assert.Equal(t, "/some/file", path)
}

// TEST031: Test wildcard rejected in keys but accepted in values
func TestWildcardRestrictions(t *testing.T) {
	// Wildcard should be rejected in keys
	invalidKey, err := NewCapUrnFromString(testUrn("*=value"))
	assert.Error(t, err)
	assert.Nil(t, invalidKey)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorInvalidCharacter, capError.Code)

	// Wildcard should be accepted in values
	validValue, err := NewCapUrnFromString(testUrn("key=*"))
	assert.NoError(t, err)
	assert.NotNil(t, validValue)

	value, exists := validValue.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "*", value)
}

// TEST032: Test duplicate keys are rejected with DuplicateKey error
func TestDuplicateKeyRejection(t *testing.T) {
	// Duplicate keys should be rejected
	duplicate, err := NewCapUrnFromString(testUrn("key=value1;key=value2"))
	assert.Error(t, err)
	assert.Nil(t, duplicate)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorDuplicateKey, capError.Code)
}

// TEST033: Test pure numeric keys rejected, mixed alphanumeric allowed, numeric values allowed
func TestNumericKeyRestriction(t *testing.T) {
	// Pure numeric keys should be rejected
	numericKey, err := NewCapUrnFromString(testUrn("123=value"))
	assert.Error(t, err)
	assert.Nil(t, numericKey)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorNumericKey, capError.Code)

	// Mixed alphanumeric keys should be allowed
	mixedKey1, err := NewCapUrnFromString(testUrn("key123=value"))
	assert.NoError(t, err)
	assert.NotNil(t, mixedKey1)

	mixedKey2, err := NewCapUrnFromString(testUrn("123key=value"))
	assert.NoError(t, err)
	assert.NotNil(t, mixedKey2)

	// Pure numeric values should be allowed
	numericValue, err := NewCapUrnFromString(testUrn("key=123"))
	assert.NoError(t, err)
	assert.NotNil(t, numericValue)

	value, exists := numericValue.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "123", value)
}

// TEST034: Test empty values are rejected
func TestEmptyValueError(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("key="))
	assert.Nil(t, cap)
	assert.Error(t, err)

	cap2, err := NewCapUrnFromString(testUrn("key=;other=value"))
	assert.Nil(t, cap2)
	assert.Error(t, err)
}

// TEST039: Test get_tag returns direction specs (in/out) with case-insensitive lookup
func TestGetTagReturnsDirectionSpecs(t *testing.T) {
	cap, err := NewCapUrnFromString(`cap:in="media:string";out="media:integer";op=test`)
	require.NoError(t, err)

	// GetTag works for in/out - returns actual value from URN, not constants
	inVal, exists := cap.GetTag("in")
	assert.True(t, exists)
	assert.Equal(t, "media:string", inVal)

	outVal, exists := cap.GetTag("out")
	assert.True(t, exists)
	assert.Equal(t, "media:integer", outVal)

	opVal, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "test", opVal)

	// Case-insensitive lookup for in/out
	inVal2, exists := cap.GetTag("IN")
	assert.True(t, exists)
	assert.Equal(t, "media:string", inVal2)

	outVal2, exists := cap.GetTag("OUT")
	assert.True(t, exists)
	assert.Equal(t, "media:integer", outVal2)
}

// ============================================================================
// MATCHING SEMANTICS SPECIFICATION TESTS
// These tests verify the exact matching semantics from RULES.md Sections 12-17
// All implementations (Rust, Go, JS, ObjC) must pass these identically
// Note: All tests now require in/out direction specs
// ============================================================================

// TEST040: Matching semantics - exact match succeeds
func TestMatchingSemantics_Test1_ExactMatch(t *testing.T) {
	// Test 1: Exact match
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 1: Exact match should succeed")
}

// TEST041: Matching semantics - cap missing tag does NOT match request with exact value
func TestMatchingSemantics_Test2_CapMissingTag(t *testing.T) {
	// Test 2: Cap missing tag is NOT an implicit wildcard
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	// Cap missing ext tag → NOT a wildcard → NO MATCH
	assert.False(t, cap.Accepts(request), "Test 2: Cap missing tag should NOT match (no implicit wildcard)")

	// Cap with explicit wildcard still matches
	cap2, err := NewCapUrnFromString(testUrn("ext=*;op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Accepts(request), "Test 2b: Cap with ext=* should match pattern with ext=pdf")
}

// TEST042: Matching semantics - cap with extra tag matches
func TestMatchingSemantics_Test3_CapHasExtraTag(t *testing.T) {
	// Test 3: Cap has extra tag
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;version=2"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 3: Cap with extra tag should match")
}

// TEST043: Matching semantics - request wildcard matches specific cap value
func TestMatchingSemantics_Test4_RequestHasWildcard(t *testing.T) {
	// Test 4: Request has wildcard
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=*"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 4: Request wildcard should match")
}

// TEST044: Matching semantics - cap wildcard matches specific request value
func TestMatchingSemantics_Test5_CapHasWildcard(t *testing.T) {
	// Test 5: Cap has wildcard
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=*"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 5: Cap wildcard should match")
}

// TEST045: Matching semantics - value mismatch does not match
func TestMatchingSemantics_Test6_ValueMismatch(t *testing.T) {
	// Test 6: Value mismatch
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=docx"))
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 6: Value mismatch should not match")
}

// TEST046: Matching semantics - cap missing tag does NOT match request with exact value (use ext=* for fallback)
func TestMatchingSemantics_Test7_FallbackPattern(t *testing.T) {
	// Test 7: Cap missing tag does NOT match request with specific tag value
	cap, err := NewCapUrnFromString(`cap:in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:ext=wav;in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)

	// Cap missing ext → NOT a wildcard → NO MATCH (use ext=* for fallback)
	assert.False(t, cap.Accepts(request), "Test 7: Cap missing ext should NOT match request with ext=wav")

	// Cap with explicit ext=* DOES match (proper fallback pattern)
	capWithWildcard, err := NewCapUrnFromString(`cap:ext=*;in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)
	assert.True(t, capWithWildcard.Accepts(request), "Test 7b: Cap with ext=* should match request with ext=wav")
}

// TEST047: Matching semantics - cap missing ext does NOT match request with ext=wav (use ext=* for fallback)
func TestMatchingSemantics_Test7b_ThumbnailVoidInput(t *testing.T) {
	// Test 7b: Cap missing ext does NOT match request with ext=wav
	outBin := "media:binary"
	cap, err := NewCapUrnFromString(fmt.Sprintf(`cap:in="%s";op=generate_thumbnail;out="%s"`, MediaVoid, outBin))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(fmt.Sprintf(`cap:ext=wav;in="%s";op=generate_thumbnail;out="%s"`, MediaVoid, outBin))
	require.NoError(t, err)

	// Cap missing ext → NOT a wildcard → NO MATCH
	assert.False(t, cap.Accepts(request), "Test 7b: Cap missing ext should NOT match request with ext=wav")

	// Cap with ext=* properly declares fallback
	capWithWildcard, err := NewCapUrnFromString(fmt.Sprintf(`cap:ext=*;in="%s";op=generate_thumbnail;out="%s"`, MediaVoid, outBin))
	require.NoError(t, err)
	assert.True(t, capWithWildcard.Accepts(request), "Test 7b: Cap with ext=* should match request with ext=wav")
}

// TEST048: Matching semantics - wildcard direction matches anything, but missing tags do NOT
func TestMatchingSemantics_Test8_WildcardDirectionMatchesAnything(t *testing.T) {
	// Test 8: Wildcard direction matches any direction, but missing tags are NOT wildcards
	cap, err := NewCapUrnFromString("cap:in=*;out=*")
	require.NoError(t, err)

	// Request with tags cap doesn't have - cap missing op/ext → NO MATCH
	request, err := NewCapUrnFromString(`cap:in="media:string";op=generate;out="media:object";ext=pdf`)
	require.NoError(t, err)

	// Cap missing op/ext → NOT wildcards → NO MATCH
	assert.False(t, cap.Accepts(request), "Test 8: Cap missing op/ext should NOT match (no implicit wildcards)")

	// If request only has direction specifiers, it should match (no tag constraints)
	request2, err := NewCapUrnFromString(`cap:in="media:string";out="media:object"`)
	require.NoError(t, err)
	assert.True(t, cap.Accepts(request2), "Test 8b: Wildcard directions should match any directions")
}

// TEST049: Matching semantics - missing tags are NOT implicit wildcards
func TestMatchingSemantics_Test9_CrossDimensionIndependence(t *testing.T) {
	// Test 9: Cap missing tag is NOT a wildcard
	// Cap has op=generate, request has ext=pdf
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)

	// Cap missing ext → NOT a wildcard → NO MATCH (cap also missing op in request)
	// Actually: cap has op=generate, request doesn't have op (patt=nil -> OK)
	// But cap doesn't have ext, request has ext=pdf (inst=nil, patt="pdf" -> NO MATCH)
	assert.False(t, cap.Accepts(request), "Test 9: Cap missing ext should NOT match request with ext=pdf")

	// Same tags should match
	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Accepts(request2), "Test 9b: Same tags should match")
}

// TEST050: Matching semantics - direction mismatch prevents matching
func TestMatchingSemantics_Test10_DirectionMismatch(t *testing.T) {
	// Test 10: Direction mismatch prevents matching
	// media:string has tags {textable:*, form:scalar}, media:bytes has tags {bytes:*}
	// Neither can provide input for the other (completely different marker tags)
	cap, err := NewCapUrnFromString(`cap:in="media:string";op=generate;out="` + MediaObject + `"`)
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:in="media:bytes";op=generate;out="` + MediaObject + `"`)
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 10: Direction mismatch should not match")
}

// TEST051: Semantic direction matching - generic provider matches specific request
func TestDirectionSemanticMatching(t *testing.T) {
	// A cap accepting media:bytes (generic) should match a request with media:pdf;bytes (specific)
	// because media:pdf;bytes has all marker tags that media:bytes requires (bytes=*)
	genericCap, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	pdfRequest, err := NewCapUrnFromString(
		`cap:in="media:pdf;bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	assert.True(t, genericCap.Accepts(pdfRequest),
		"Generic bytes provider must match specific pdf;bytes request")

	// Generic cap also matches epub;bytes (any bytes subtype)
	epubRequest, err := NewCapUrnFromString(
		`cap:in="media:epub;bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	assert.True(t, genericCap.Accepts(epubRequest),
		"Generic bytes provider must match epub;bytes request")

	// Reverse: specific cap does NOT match generic request
	// A pdf-only handler cannot accept arbitrary bytes
	pdfCap, err := NewCapUrnFromString(
		`cap:in="media:pdf;bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	genericRequest, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	assert.False(t, pdfCap.Accepts(genericRequest),
		"Specific pdf;bytes cap must NOT match generic bytes request")

	// Incompatible types: pdf cap does NOT match epub request
	assert.False(t, pdfCap.Accepts(epubRequest),
		"PDF-specific cap must NOT match epub request (epub lacks pdf marker)")

	// Output direction: cap producing more specific output matches less specific request
	specificOutCap, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	genericOutRequest, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;bytes"`,
	)
	require.NoError(t, err)
	assert.True(t, specificOutCap.Accepts(genericOutRequest),
		"Cap producing image;png;bytes;thumbnail must satisfy request for image;bytes")

	// Reverse output: generic output cap does NOT match specific output request
	genericOutCap, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;bytes"`,
	)
	require.NoError(t, err)
	specificOutRequest, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	assert.False(t, genericOutCap.Accepts(specificOutRequest),
		"Cap producing generic image;bytes must NOT satisfy request requiring image;png;bytes;thumbnail")
}

// TEST052: Semantic direction specificity - more media URN tags = higher specificity
func TestDirectionSemanticSpecificity(t *testing.T) {
	// media:bytes has 1 tag, media:pdf;bytes has 2 tags
	// media:image;png;bytes;thumbnail has 4 tags
	genericCap, err := NewCapUrnFromString(
		`cap:in="media:bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	specificCap, err := NewCapUrnFromString(
		`cap:in="media:pdf;bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)

	// generic: bytes(1) + image;png;bytes;thumbnail(4) + op(3) = 8
	assert.Equal(t, 8, genericCap.Specificity())
	// specific: pdf;bytes(2) + image;png;bytes;thumbnail(4) + op(3) = 9
	assert.Equal(t, 9, specificCap.Specificity())

	assert.True(t, specificCap.Specificity() > genericCap.Specificity(),
		"pdf;bytes cap must be more specific than bytes cap")

	// CapMatcher should prefer the more specific cap when both match
	pdfRequest, err := NewCapUrnFromString(
		`cap:in="media:pdf;bytes";op=generate_thumbnail;out="media:image;png;bytes;thumbnail"`,
	)
	require.NoError(t, err)
	caps := []*CapUrn{genericCap, specificCap}
	matcher := &CapMatcher{}
	best := matcher.FindBestMatch(caps, pdfRequest)
	require.NotNil(t, best)
	assert.Equal(t, 9, best.Specificity(),
		"CapMatcher must prefer the more specific pdf;bytes provider")
}
