package capns

import (
	"encoding/json"
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

func TestDirectionMatching(t *testing.T) {
	// Direction specs must match for caps to match
	cap1, err := NewCapUrnFromString(`cap:in="media:string";out="media:object";op=test`)
	require.NoError(t, err)
	cap2, err := NewCapUrnFromString(`cap:in="media:string";out="media:object";op=test`)
	require.NoError(t, err)
	assert.True(t, cap1.Matches(cap2))

	// Different inSpec should not match
	cap3, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Matches(cap3))

	// Different outSpec should not match
	cap4, err := NewCapUrnFromString(`cap:in="media:string";out="media:integer";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Matches(cap4))

	// Wildcard in direction should match
	cap5, err := NewCapUrnFromString(`cap:in=*;out="media:object";op=test`)
	require.NoError(t, err)
	assert.True(t, cap1.Matches(cap5))
	assert.True(t, cap5.Matches(cap1))
}

func TestCanonicalStringFormat(t *testing.T) {
	capUrn, err := NewCapUrnFromString(testUrn("op=generate;target=thumbnail;ext=pdf"))
	require.NoError(t, err)

	// Should be sorted alphabetically with in/out in their sorted positions
	// Media URNs with semicolons (like MediaObject) need quoting, but simple ones (like MediaVoid) don't
	// Alphabetical order: ext < in < op < out < target
	assert.Equal(t, `cap:ext=pdf;in=`+MediaVoid+`;op=generate;out="`+MediaObject+`";target=thumbnail`, capUrn.ToString())
}

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
	assert.True(t, cap1.Matches(cap2))
	assert.True(t, cap2.Matches(cap1))
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

func TestTagMatching(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;target=thumbnail"))
	require.NoError(t, err)

	// Exact match
	request1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;target=thumbnail"))
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1))

	// Subset match (other tags)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap.Matches(request2))

	// Wildcard request should match specific cap
	request3, err := NewCapUrnFromString(testUrn("ext=*"))
	require.NoError(t, err)
	assert.True(t, cap.Matches(request3))

	// No match - conflicting value
	request4, err := NewCapUrnFromString(testUrn("op=extract"))
	require.NoError(t, err)
	assert.False(t, cap.Matches(request4))
}

func TestMissingTagHandling(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	// Under new semantics: pattern has ext=pdf, instance missing ext → NO MATCH
	// Instance missing a tag that pattern requires is not a match
	request1, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)
	assert.False(t, cap.Matches(request1)) // pattern requires ext, cap doesn't have it

	// Cap with extra tags can match subset requests (pattern missing = no constraint)
	cap2, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Matches(request2)) // pattern doesn't constrain ext, so cap with ext=pdf matches

	// Cap with explicit wildcard matches pattern with specific value
	cap3, err := NewCapUrnFromString(testUrn("ext=*;op=generate"))
	require.NoError(t, err)
	request3, err := NewCapUrnFromString(testUrn("ext=pdf;op=generate"))
	require.NoError(t, err)
	assert.True(t, cap3.Matches(request3)) // cap has ext=*, pattern has ext=pdf → MATCH
}

func TestSpecificity(t *testing.T) {
	// Specificity uses graded scoring:
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

	// cap1: in (3) + out (3) + type (3) = 9
	assert.Equal(t, 9, cap1.Specificity())
	// cap2: in (3) + out (3) + op (3) = 9
	assert.Equal(t, 9, cap2.Specificity())
	// cap3: in (3) + out (3) + op (2 for *) + ext (3) = 11
	assert.Equal(t, 11, cap3.Specificity())

	// Wildcard in direction scores 2 points
	cap4, err := NewCapUrnFromString(`cap:in=*;out="media:object";op=test`)
	require.NoError(t, err)
	// cap4: in (2 for *) + out (3) + op (3) = 8
	assert.Equal(t, 8, cap4.Specificity())
}

func TestCompatibility(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate;format=*"))
	require.NoError(t, err)

	cap3, err := NewCapUrnFromString(testUrn("type=image;op=extract"))
	require.NoError(t, err)

	assert.True(t, cap1.IsCompatibleWith(cap2))
	assert.True(t, cap2.IsCompatibleWith(cap1))
	assert.False(t, cap1.IsCompatibleWith(cap3))

	// Missing tags are treated as wildcards for compatibility
	cap4, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap1.IsCompatibleWith(cap4))
	assert.True(t, cap4.IsCompatibleWith(cap1))

	// Different direction specs are incompatible
	cap5, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=generate`)
	require.NoError(t, err)
	assert.False(t, cap1.IsCompatibleWith(cap5))
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

func TestUnterminatedQuoteError(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="unterminated`))
	assert.Nil(t, cap)
	assert.Error(t, err)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorUnterminatedQuote, capError.Code)
}

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

func TestRoundTripSimple(t *testing.T) {
	original := testUrn("op=generate;ext=pdf")
	cap, err := NewCapUrnFromString(original)
	require.NoError(t, err)
	serialized := cap.ToString()
	reparsed, err := NewCapUrnFromString(serialized)
	require.NoError(t, err)
	assert.True(t, cap.Equals(reparsed))
}

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

func TestMatchingCaseSensitiveValues(t *testing.T) {
	// Values with different case should NOT match
	cap1, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)
	cap2, err := NewCapUrnFromString(testUrn(`key="value"`))
	require.NoError(t, err)
	assert.False(t, cap1.Matches(cap2))
	assert.False(t, cap2.Matches(cap1))

	// Same case should match
	cap3, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)
	assert.True(t, cap1.Matches(cap3))
}

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

func TestWithTagPreservesValue(t *testing.T) {
	cap := NewCapUrn(MediaVoid, MediaObject, map[string]string{})
	modified := cap.WithTag("key", "ValueWithCase")

	value, exists := modified.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "ValueWithCase", value)
}

func TestSemanticEquivalence(t *testing.T) {
	// Unquoted and quoted simple lowercase values are equivalent
	unquoted, err := NewCapUrnFromString(testUrn("key=simple"))
	require.NoError(t, err)
	quoted, err := NewCapUrnFromString(testUrn(`key="simple"`))
	require.NoError(t, err)
	assert.True(t, unquoted.Equals(quoted))
}

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

func TestMinimalCapUrn(t *testing.T) {
	// Minimal valid cap URN has just in and out
	cap, err := NewCapUrnFromString(`cap:in="media:void";out="media:object"`)
	require.NoError(t, err)
	// InSpec and OutSpec return actual values from parsed string
	assert.Equal(t, "media:void", cap.InSpec())
	assert.Equal(t, "media:object", cap.OutSpec())
	assert.Equal(t, 0, len(cap.tags))
}

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

func TestDuplicateKeyRejection(t *testing.T) {
	// Duplicate keys should be rejected
	duplicate, err := NewCapUrnFromString(testUrn("key=value1;key=value2"))
	assert.Error(t, err)
	assert.Nil(t, duplicate)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorDuplicateKey, capError.Code)
}

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

func TestEmptyValueError(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("key="))
	assert.Nil(t, cap)
	assert.Error(t, err)

	cap2, err := NewCapUrnFromString(testUrn("key=;other=value"))
	assert.Nil(t, cap2)
	assert.Error(t, err)
}

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

func TestMatchingSemantics_Test1_ExactMatch(t *testing.T) {
	// Test 1: Exact match
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Matches(request), "Test 1: Exact match should succeed")
}

func TestMatchingSemantics_Test2_CapMissingTag(t *testing.T) {
	// Test 2: Under new semantics, pattern with specific value requires instance to have it
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	// Pattern has ext=pdf, instance missing ext → NO MATCH
	assert.False(t, cap.Matches(request), "Test 2: Cap missing tag pattern requires should NOT match")

	// But cap with explicit wildcard should match
	cap2, err := NewCapUrnFromString(testUrn("ext=*;op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Matches(request), "Test 2b: Cap with ext=* should match pattern with ext=pdf")
}

func TestMatchingSemantics_Test3_CapHasExtraTag(t *testing.T) {
	// Test 3: Cap has extra tag
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;version=2"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Matches(request), "Test 3: Cap with extra tag should match")
}

func TestMatchingSemantics_Test4_RequestHasWildcard(t *testing.T) {
	// Test 4: Request has wildcard
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=*"))
	require.NoError(t, err)

	assert.True(t, cap.Matches(request), "Test 4: Request wildcard should match")
}

func TestMatchingSemantics_Test5_CapHasWildcard(t *testing.T) {
	// Test 5: Cap has wildcard
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=*"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Matches(request), "Test 5: Cap wildcard should match")
}

func TestMatchingSemantics_Test6_ValueMismatch(t *testing.T) {
	// Test 6: Value mismatch
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=docx"))
	require.NoError(t, err)

	assert.False(t, cap.Matches(request), "Test 6: Value mismatch should not match")
}

func TestMatchingSemantics_Test7_FallbackPattern(t *testing.T) {
	// Test 7: Under new semantics, fallback requires explicit wildcard
	// Cap without ext does NOT match pattern with specific ext
	cap, err := NewCapUrnFromString(`cap:in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:ext=wav;in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)

	// Pattern has ext=wav, instance missing ext → NO MATCH
	assert.False(t, cap.Matches(request), "Test 7: Cap missing ext should NOT match pattern with ext=wav")

	// Fallback cap with ext=* matches any ext
	capFallback, err := NewCapUrnFromString(`cap:ext=*;in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)
	assert.True(t, capFallback.Matches(request), "Test 7b: Cap with ext=* should match pattern with ext=wav")
}

func TestMatchingSemantics_Test8_WildcardDirectionMatchesAnything(t *testing.T) {
	// Test 8: Wildcard direction matches any direction, but other tags still matter
	cap, err := NewCapUrnFromString("cap:in=*;out=*")
	require.NoError(t, err)

	// Request with tags cap doesn't have - under new semantics, cap missing op/ext → NO MATCH
	request, err := NewCapUrnFromString(`cap:in="media:string";op=generate;out="media:object";ext=pdf`)
	require.NoError(t, err)

	// Cap doesn't have op or ext, pattern requires specific values → NO MATCH
	assert.False(t, cap.Matches(request), "Test 8: Cap missing op/ext should NOT match pattern with them")

	// But if request only has direction specifiers, it should match
	request2, err := NewCapUrnFromString(`cap:in="media:string";out="media:object"`)
	require.NoError(t, err)
	assert.True(t, cap.Matches(request2), "Test 8b: Wildcard directions should match any directions")
}

func TestMatchingSemantics_Test9_CrossDimensionIndependence(t *testing.T) {
	// Test 9: Under new semantics, each side's specific values must be satisfied
	// Cap has op=generate, request has ext=pdf
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)

	// Pattern has ext=pdf (specific), instance missing ext → NO MATCH
	assert.False(t, cap.Matches(request), "Test 9: Pattern with ext=pdf, cap missing ext → NO MATCH")

	// If neither has the other's tag, both missing means no constraint
	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Matches(request2), "Test 9b: Same tags should match")
}

func TestMatchingSemantics_Test10_DirectionMismatch(t *testing.T) {
	// Test 10: Direction mismatch prevents matching
	cap, err := NewCapUrnFromString(`cap:in="media:string";op=generate;out="media:object"`)
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:in="media:binary";op=generate;out="media:object"`)
	require.NoError(t, err)

	assert.False(t, cap.Matches(request), "Test 10: Direction mismatch should not match")
}
