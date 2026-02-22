package urn

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/filegrind/capns-go/standard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All cap URNs now require in and out specs. Use these helpers:
// Media URNs must be quoted in cap URNs because they contain semicolons
// Use proper tags for is_binary/is_json/is_text detection
func testUrn(tags string) string {
	// Use standard.MediaObject constant for consistent canonical form
	if tags == "" {
		return `cap:in="media:void";out="` + standard.MediaObject + `"`
	}
	return `cap:in="media:void";out="` + standard.MediaObject + `";` + tags
}

func testUrnWithIO(inSpec, outSpec, tags string) string {
	// Media URNs need quoting because they contain semicolons
	if tags == "" {
		return `cap:in="` + inSpec + `";out="` + outSpec + `"`
	}
	return `cap:in="` + inSpec + `";out="` + outSpec + `";` + tags
}

// TEST001: Test that cap URN is created with tags parsed correctly and direction specs accessible
func Test001_cap_urn_creation(t *testing.T) {
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
	assert.Equal(t, standard.MediaVoid, capUrn.InSpec())
	assert.Equal(t, standard.MediaObject, capUrn.OutSpec())
}

// TEST002: Test that missing 'in' or 'out' defaults to media: wildcard
func Test002_direction_specs_default_to_wildcard(t *testing.T) {
	// Missing 'in' defaults to wildcard "media:"
	cap1, err := NewCapUrnFromString(`cap:out="media:object";op=test`)
	assert.NoError(t, err)
	assert.Equal(t, "media:", cap1.InSpec())

	// Missing 'out' defaults to wildcard "media:"
	cap2, err := NewCapUrnFromString(`cap:in="media:void";op=test`)
	assert.NoError(t, err)
	assert.Equal(t, "media:", cap2.OutSpec())

	// Both present should succeed
	cap3, err := NewCapUrnFromString(`cap:in="media:void";out="media:object";op=test`)
	assert.NoError(t, err)
	assert.Equal(t, "media:void", cap3.InSpec())
	assert.Equal(t, "media:object", cap3.OutSpec())
}

// TEST003: Test that direction specs must match exactly, different in/out types don't match, wildcard matches any
func Test003_direction_matching(t *testing.T) {
	cap1, err := NewCapUrnFromString(`cap:in="media:string";out="media:object";op=test`)
	require.NoError(t, err)
	cap2, err := NewCapUrnFromString(`cap:in="media:string";out="media:object";op=test`)
	require.NoError(t, err)
	assert.True(t, cap1.Accepts(cap2))

	cap3, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap3))

	cap4, err := NewCapUrnFromString(`cap:in="media:string";out="media:integer";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap4))

	cap5, err := NewCapUrnFromString(`cap:in=*;out="media:object";op=test`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap5))
	assert.True(t, cap5.Accepts(cap1))
}

// TEST004: Test that unquoted keys and values are normalized to lowercase
func Test004_unquoted_values_lowercased(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("OP=Generate;EXT=PDF;Target=Thumbnail"))
	require.NoError(t, err)

	op, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	ext, exists := cap.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)

	target, exists := cap.GetTag("target")
	assert.True(t, exists)
	assert.Equal(t, "thumbnail", target)

	op2, exists := cap.GetTag("OP")
	assert.True(t, exists)
	assert.Equal(t, "generate", op2)
}

// TEST005: Test that quoted values preserve case while unquoted are lowercased
func Test005_quoted_values_preserve_case(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="Value With Spaces"`))
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "Value With Spaces", value)

	cap2, err := NewCapUrnFromString(testUrn(`KEY="Value With Spaces"`))
	require.NoError(t, err)
	value2, exists := cap2.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "Value With Spaces", value2)

	unquoted, err := NewCapUrnFromString(testUrn("key=UPPERCASE"))
	require.NoError(t, err)
	quoted, err := NewCapUrnFromString(testUrn(`key="UPPERCASE"`))
	require.NoError(t, err)

	unquotedVal, _ := unquoted.GetTag("key")
	quotedVal, _ := quoted.GetTag("key")
	assert.Equal(t, "uppercase", unquotedVal)
	assert.Equal(t, "UPPERCASE", quotedVal)
	assert.False(t, unquoted.Equals(quoted))
}

// TEST006: Test that quoted values can contain special characters (semicolons, equals, spaces)
func Test006_quoted_value_special_chars(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="value;with;semicolons"`))
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "value;with;semicolons", value)

	cap2, err := NewCapUrnFromString(testUrn(`key="value=with=equals"`))
	require.NoError(t, err)
	value2, exists := cap2.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "value=with=equals", value2)

	cap3, err := NewCapUrnFromString(testUrn(`key="hello world"`))
	require.NoError(t, err)
	value3, exists := cap3.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "hello world", value3)
}

// TEST007: Test that escape sequences in quoted values (\" and \\) are parsed correctly
func Test007_quoted_value_escape_sequences(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="value\"quoted\""`))
	require.NoError(t, err)
	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `value"quoted"`, value)

	cap2, err := NewCapUrnFromString(testUrn(`key="path\\file"`))
	require.NoError(t, err)
	value2, exists := cap2.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `path\file`, value2)

	cap3, err := NewCapUrnFromString(testUrn(`key="say \"hello\\world\""`))
	require.NoError(t, err)
	value3, exists := cap3.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, `say "hello\world"`, value3)
}

// TEST008: Test that mixed quoted and unquoted values in same URN parse correctly
func Test008_mixed_quoted_unquoted(t *testing.T) {
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
func Test009_unterminated_quote_error(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="unterminated`))
	assert.Nil(t, cap)
	assert.Error(t, err)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorUnterminatedQuote, capError.Code)
}

// TEST010: Test that invalid escape sequences (like \n, \x) produce InvalidEscapeSequence error
func Test010_invalid_escape_sequence_error(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="bad\n"`))
	assert.Nil(t, cap)
	assert.Error(t, err)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorInvalidEscapeSequence, capError.Code)

	cap2, err := NewCapUrnFromString(testUrn(`key="bad\x"`))
	assert.Nil(t, cap2)
	assert.Error(t, err)
	capError2, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorInvalidEscapeSequence, capError2.Code)
}

// TEST011: Test that serialization uses smart quoting (no quotes for simple lowercase, quotes for special chars/uppercase)
func Test011_serialization_smart_quoting(t *testing.T) {
	// Simple lowercase value — no quoting needed; media URNs without semicolons unquoted
	cap, err := NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		OutSpec(standard.MediaObject).
		Tag("key", "simple").
		Build()
	require.NoError(t, err)
	// Go TaggedUrn serialization: no quotes around media:object (only colons, no semicolons/spaces)
	assert.Equal(t, `cap:in=media:void;key=simple;out=`+standard.MediaObject, cap.ToString())

	// Value with spaces — must be quoted
	cap2, err := NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		OutSpec(standard.MediaObject).
		Tag("key", "has spaces").
		Build()
	require.NoError(t, err)
	assert.Equal(t, `cap:in=media:void;key="has spaces";out=`+standard.MediaObject, cap2.ToString())

	// Value with uppercase — must be quoted
	cap4, err := NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		OutSpec(standard.MediaObject).
		Tag("key", "HasUpper").
		Build()
	require.NoError(t, err)
	assert.Equal(t, `cap:in=media:void;key="HasUpper";out=`+standard.MediaObject, cap4.ToString())
}

// TEST012: Test that simple cap URN round-trips (parse -> serialize -> parse equals original)
func Test012_round_trip_simple(t *testing.T) {
	original := testUrn("op=generate;ext=pdf")
	cap, err := NewCapUrnFromString(original)
	require.NoError(t, err)
	serialized := cap.ToString()
	reparsed, err := NewCapUrnFromString(serialized)
	require.NoError(t, err)
	assert.True(t, cap.Equals(reparsed))
}

// TEST013: Test that quoted values round-trip preserving case and spaces
func Test013_round_trip_quoted(t *testing.T) {
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
func Test014_round_trip_escapes(t *testing.T) {
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

// TEST015: Test that cap: prefix is required and case-insensitive
func Test015_cap_prefix_required(t *testing.T) {
	capUrn, err := NewCapUrnFromString(`in="media:void";out="media:object";op=generate`)
	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorMissingCapPrefix, err.(*CapUrnError).Code)

	capUrn, err = NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	assert.NoError(t, err)
	assert.NotNil(t, capUrn)
	op, exists := capUrn.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	capUrn, err = NewCapUrnFromString(`CAP:in="media:void";out="media:object";op=generate`)
	assert.NoError(t, err)
	op, exists = capUrn.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)
}

// TEST016: Test that trailing semicolon is equivalent (same hash, same string, matches)
func Test016_trailing_semicolon_equivalence(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf") + ";")
	require.NoError(t, err)

	assert.True(t, cap1.Equals(cap2))
	assert.Equal(t, cap1.ToString(), cap2.ToString())
	assert.True(t, cap1.Accepts(cap2))
	assert.True(t, cap2.Accepts(cap1))
}

// TEST017: Test tag matching: exact match, subset match, wildcard match, value mismatch
func Test017_tag_matching(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;target=thumbnail"))
	require.NoError(t, err)

	// Exact match — both directions accept
	request1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;target=thumbnail"))
	require.NoError(t, err)
	assert.True(t, cap.Accepts(request1))
	assert.True(t, request1.Accepts(cap))

	// Routing direction: request(op=generate) accepts cap(op,ext,target) — request only needs op
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, request2.Accepts(cap))
	// Reverse: cap(op,ext,target) as pattern rejects request missing ext,target
	assert.False(t, cap.Accepts(request2))

	// Routing direction: request(ext=*) accepts cap(ext=pdf) — wildcard matches specific
	request3, err := NewCapUrnFromString(testUrn("ext=*"))
	require.NoError(t, err)
	assert.True(t, request3.Accepts(cap))

	// Conflicting value — neither direction accepts
	request4, err := NewCapUrnFromString(testUrn("op=extract"))
	require.NoError(t, err)
	assert.False(t, cap.Accepts(request4))
	assert.False(t, request4.Accepts(cap))
}

// TEST018: Test that quoted values with different case do NOT match (case-sensitive)
func Test018_matching_case_sensitive_values(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)
	cap2, err := NewCapUrnFromString(testUrn(`key="value"`))
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap2))
	assert.False(t, cap2.Accepts(cap1))

	cap3, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)
	assert.True(t, cap1.Accepts(cap3))
}

// TEST019: Missing tag in instance causes rejection -- pattern's tags are constraints
func Test019_missing_tag_handling(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	// cap(op) as pattern: instance(ext) missing op → reject
	request1, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)
	assert.False(t, cap.Accepts(request1))
	// request(ext) as pattern: instance(cap) missing ext → reject
	assert.False(t, request1.Accepts(cap))

	// Routing: request(op) accepts cap(op,ext) — instance has op → match
	cap2, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, request2.Accepts(cap2))
	// Reverse: cap(op,ext) as pattern rejects request missing ext
	assert.False(t, cap2.Accepts(request2))

	// cap(ext=*;op=generate) as pattern accepts request(ext=pdf;op=generate)
	cap3, err := NewCapUrnFromString(testUrn("ext=*;op=generate"))
	require.NoError(t, err)
	request3, err := NewCapUrnFromString(testUrn("ext=pdf;op=generate"))
	require.NoError(t, err)
	assert.True(t, cap3.Accepts(request3))
}

// TEST020: Test specificity calculation (direction specs use MediaUrn tag count, wildcards don't count)
func Test020_specificity(t *testing.T) {
	// Go uses standard.MediaVoid="media:void" (1 tag) and standard.MediaObject="media:object" (1 tag)
	// So base specificity for testUrn = 1(void) + 1(object) = 2, plus non-wildcard tags
	cap1, err := NewCapUrnFromString(testUrn("type=general"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	cap3, err := NewCapUrnFromString(testUrn("op=*;ext=pdf"))
	require.NoError(t, err)

	// void(1) + object(1) + type=general(1) = 3
	assert.Equal(t, 3, cap1.Specificity())
	// void(1) + object(1) + op=generate(1) = 3
	assert.Equal(t, 3, cap2.Specificity())
	// void(1) + object(1) + ext=pdf(1) = 3 (op=* wildcards don't count)
	assert.Equal(t, 3, cap3.Specificity())

	// in=wildcard(0) + object(1) + op=test(1) = 2
	cap4, err := NewCapUrnFromString(`cap:in=*;out="` + standard.MediaObject + `";op=test`)
	require.NoError(t, err)
	assert.Equal(t, 2, cap4.Specificity())
}

// TEST021: Test builder creates cap URN with correct tags and direction specs
func Test021_builder(t *testing.T) {
	cap, err := NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		OutSpec(standard.MediaObject).
		Tag("op", "generate").
		Tag("target", "thumbnail").
		Tag("ext", "pdf").
		Build()
	require.NoError(t, err)

	op, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "generate", op)

	assert.Equal(t, standard.MediaVoid, cap.InSpec())
	assert.Equal(t, standard.MediaObject, cap.OutSpec())
}

// TEST022: Test builder requires both in_spec and out_spec
func Test022_builder_requires_direction(t *testing.T) {
	_, err := NewCapUrnBuilder().
		OutSpec(standard.MediaObject).
		Tag("op", "test").
		Build()
	assert.Error(t, err)

	_, err = NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		Tag("op", "test").
		Build()
	assert.Error(t, err)

	_, err = NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		OutSpec(standard.MediaObject).
		Build()
	assert.NoError(t, err)
}

// TEST023: Test builder lowercases keys but preserves value case
func Test023_builder_preserves_case(t *testing.T) {
	cap, err := NewCapUrnBuilder().
		InSpec(standard.MediaVoid).
		OutSpec(standard.MediaObject).
		Tag("KEY", "ValueWithCase").
		Build()
	require.NoError(t, err)

	value, exists := cap.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "ValueWithCase", value)
}

// TEST024: Directional accepts -- pattern's tags are constraints, instance must satisfy
func Test024_directional_accepts(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(testUrn("op=generate;format=*"))
	require.NoError(t, err)

	cap3, err := NewCapUrnFromString(testUrn("type=image;op=extract"))
	require.NoError(t, err)

	assert.False(t, cap1.Accepts(cap2))
	assert.False(t, cap2.Accepts(cap1))

	assert.False(t, cap1.Accepts(cap3))
	assert.False(t, cap3.Accepts(cap1))

	// Routing: general request(op) accepts specific cap(op,ext) — instance has op → match
	cap4, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap4.Accepts(cap1))  // cap4 only requires op, cap1 has it
	// Reverse: specific cap(op,ext) rejects general request missing ext
	assert.False(t, cap1.Accepts(cap4))

	cap5, err := NewCapUrnFromString(`cap:in="media:binary";out="media:object";op=generate`)
	require.NoError(t, err)
	assert.False(t, cap1.Accepts(cap5))
	assert.False(t, cap5.Accepts(cap1))
}

// TEST025: Test find_best_match returns most specific matching cap
func Test025_best_match(t *testing.T) {
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

	ext, exists := best.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)
}

// TEST026: Test merge combines tags from both caps, subset keeps only specified tags
func Test026_merge_and_subset(t *testing.T) {
	cap1, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	cap2, err := NewCapUrnFromString(`cap:in="media:binary";out="media:integer";ext=pdf;output=binary`)
	require.NoError(t, err)

	merged := cap1.Merge(cap2)
	assert.Equal(t, "media:binary", merged.InSpec())
	assert.Equal(t, "media:integer", merged.OutSpec())

	op, _ := merged.GetTag("op")
	assert.Equal(t, "generate", op)
	ext, _ := merged.GetTag("ext")
	assert.Equal(t, "pdf", ext)

	// Subset test
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;output=binary;target=thumbnail"))
	require.NoError(t, err)

	subset := cap.Subset([]string{"type", "ext"})
	extVal, exists := subset.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", extVal)

	_, opExists := subset.GetTag("op")
	assert.False(t, opExists)

	assert.Equal(t, standard.MediaVoid, subset.InSpec())
	assert.Equal(t, standard.MediaObject, subset.OutSpec())
}

// TEST027: Test with_wildcard_tag sets tag to wildcard, including in/out
func Test027_wildcard_tag(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)

	wildcarded := cap.WithWildcardTag("ext")
	ext, exists := wildcarded.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "*", ext)

	wildcardIn := cap.WithWildcardTag("in")
	assert.Equal(t, "*", wildcardIn.InSpec())

	wildcardOut := cap.WithWildcardTag("out")
	assert.Equal(t, "*", wildcardOut.OutSpec())
}

// TEST028: Test empty cap URN defaults to media: wildcard
func Test028_empty_cap_urn_defaults_to_wildcard(t *testing.T) {
	// Empty cap URN defaults to media: for both in and out
	result, err := NewCapUrnFromString("cap:")
	require.NoError(t, err, "Empty cap should default to media: wildcard")
	require.NotNil(t, result)
	assert.Equal(t, "media:", result.InSpec())
	assert.Equal(t, "media:", result.OutSpec())
	assert.Equal(t, 0, len(result.tags))

	// Trailing semicolon also works
	result2, err := NewCapUrnFromString("cap:;")
	require.NoError(t, err, "cap:; should default to media: wildcard")
	require.NotNil(t, result2)
	assert.Equal(t, "media:", result2.InSpec())
	assert.Equal(t, "media:", result2.OutSpec())
}

// TEST029: Test minimal valid cap URN has just in and out, empty tags
func Test029_minimal_cap_urn(t *testing.T) {
	cap, err := NewCapUrnFromString(`cap:in="media:void";out="media:object"`)
	require.NoError(t, err)
	assert.Equal(t, "media:void", cap.InSpec())
	assert.Equal(t, "media:object", cap.OutSpec())
	assert.Equal(t, 0, len(cap.tags))
}

// TEST030: Test extended characters (forward slashes, colons) in tag values
func Test030_extended_character_support(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("url=https://example_org/api;path=/some/file"))
	assert.NoError(t, err)
	assert.NotNil(t, cap)

	urlVal, exists := cap.GetTag("url")
	assert.True(t, exists)
	assert.Equal(t, "https://example_org/api", urlVal)

	pathVal, exists := cap.GetTag("path")
	assert.True(t, exists)
	assert.Equal(t, "/some/file", pathVal)
}

// TEST031: Test wildcard rejected in keys but accepted in values
func Test031_wildcard_restrictions(t *testing.T) {
	invalidKey, err := NewCapUrnFromString(testUrn("*=value"))
	assert.Error(t, err)
	assert.Nil(t, invalidKey)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorInvalidCharacter, capError.Code)

	validValue, err := NewCapUrnFromString(testUrn("key=*"))
	assert.NoError(t, err)
	assert.NotNil(t, validValue)

	value, exists := validValue.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "*", value)
}

// TEST032: Test duplicate keys are rejected with DuplicateKey error
func Test032_duplicate_key_rejection(t *testing.T) {
	duplicate, err := NewCapUrnFromString(testUrn("key=value1;key=value2"))
	assert.Error(t, err)
	assert.Nil(t, duplicate)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorDuplicateKey, capError.Code)
}

// TEST033: Test pure numeric keys rejected, mixed alphanumeric allowed, numeric values allowed
func Test033_numeric_key_restriction(t *testing.T) {
	numericKey, err := NewCapUrnFromString(testUrn("123=value"))
	assert.Error(t, err)
	assert.Nil(t, numericKey)
	capError, ok := err.(*CapUrnError)
	assert.True(t, ok)
	assert.Equal(t, ErrorNumericKey, capError.Code)

	mixedKey1, err := NewCapUrnFromString(testUrn("key123=value"))
	assert.NoError(t, err)
	assert.NotNil(t, mixedKey1)

	mixedKey2, err := NewCapUrnFromString(testUrn("123key=value"))
	assert.NoError(t, err)
	assert.NotNil(t, mixedKey2)

	numericValue, err := NewCapUrnFromString(testUrn("key=123"))
	assert.NoError(t, err)
	assert.NotNil(t, numericValue)

	value, exists := numericValue.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "123", value)
}

// TEST034: Test empty values are rejected
func Test034_empty_value_error(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("key="))
	assert.Nil(t, cap)
	assert.Error(t, err)

	cap2, err := NewCapUrnFromString(testUrn("key=;other=value"))
	assert.Nil(t, cap2)
	assert.Error(t, err)
}

// TEST035: Test has_tag is case-sensitive for values, case-insensitive for keys, works for in/out
func Test035_has_tag_case_sensitive(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn(`key="Value"`))
	require.NoError(t, err)

	assert.True(t, cap.HasTag("key", "Value"))
	assert.False(t, cap.HasTag("key", "value"))
	assert.False(t, cap.HasTag("key", "VALUE"))
	assert.True(t, cap.HasTag("KEY", "Value"))
	assert.True(t, cap.HasTag("Key", "Value"))
	assert.True(t, cap.HasTag("in", standard.MediaVoid))
	assert.True(t, cap.HasTag("out", standard.MediaObject))
}

// TEST036: Test with_tag preserves value case
func Test036_with_tag_preserves_value(t *testing.T) {
	cap := NewCapUrn(standard.MediaVoid, standard.MediaObject, map[string]string{})
	modified := cap.WithTag("key", "ValueWithCase")

	value, exists := modified.GetTag("key")
	assert.True(t, exists)
	assert.Equal(t, "ValueWithCase", value)
}

// TEST037: Test with_tag rejects empty value
func Test037_with_tag_rejects_empty_value(t *testing.T) {
	cap := NewCapUrn(standard.MediaVoid, standard.MediaObject, map[string]string{})
	modified, err := cap.WithTagValidated("key", "")
	assert.Error(t, err, "with_tag should reject empty value")
	assert.Nil(t, modified)
}

// TEST038: Test semantic equivalence of unquoted and quoted simple lowercase values
func Test038_semantic_equivalence(t *testing.T) {
	unquoted, err := NewCapUrnFromString(testUrn("key=simple"))
	require.NoError(t, err)
	quoted, err := NewCapUrnFromString(testUrn(`key="simple"`))
	require.NoError(t, err)
	assert.True(t, unquoted.Equals(quoted))
}

// TEST039: Test get_tag returns direction specs (in/out) with case-insensitive lookup
func Test039_get_tag_returns_direction_specs(t *testing.T) {
	cap, err := NewCapUrnFromString(`cap:in="media:string";out="media:integer";op=test`)
	require.NoError(t, err)

	inVal, exists := cap.GetTag("in")
	assert.True(t, exists)
	assert.Equal(t, "media:string", inVal)

	outVal, exists := cap.GetTag("out")
	assert.True(t, exists)
	assert.Equal(t, "media:integer", outVal)

	opVal, exists := cap.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "test", opVal)

	inVal2, exists := cap.GetTag("IN")
	assert.True(t, exists)
	assert.Equal(t, "media:string", inVal2)

	outVal2, exists := cap.GetTag("OUT")
	assert.True(t, exists)
	assert.Equal(t, "media:integer", outVal2)
}

// TEST040: Matching semantics - exact match succeeds
func Test040_matching_semantics_exact_match(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 1: Exact match should succeed")
}

// TEST041: Matching semantics - cap missing tag matches (implicit wildcard)
func Test041_matching_semantics_cap_missing_tag(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 2: Cap missing tag should NOT match (no implicit wildcard)")

	cap2, err := NewCapUrnFromString(testUrn("ext=*;op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Accepts(request), "Test 2b: Cap with ext=* should match pattern with ext=pdf")
}

// TEST042: Pattern rejects instance missing required tags
func Test042_matching_semantics_cap_has_extra_tag(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf;version=2"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 3: Cap with extra tag should match")
}

// TEST043: Matching semantics - request wildcard matches specific cap value
func Test043_matching_semantics_request_has_wildcard(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=*"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 4: Request wildcard should match")
}

// TEST044: Matching semantics - cap wildcard matches specific request value
func Test044_matching_semantics_cap_has_wildcard(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=*"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	assert.True(t, cap.Accepts(request), "Test 5: Cap wildcard should match")
}

// TEST045: Matching semantics - value mismatch does not match
func Test045_matching_semantics_value_mismatch(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate;ext=pdf"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("op=generate;ext=docx"))
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 6: Value mismatch should not match")
}

// TEST046: Matching semantics - fallback pattern (cap missing tag = implicit wildcard)
func Test046_matching_semantics_fallback_pattern(t *testing.T) {
	cap, err := NewCapUrnFromString(`cap:in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:ext=wav;in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 7: Cap missing ext should NOT match request with ext=wav")

	capWithWildcard, err := NewCapUrnFromString(`cap:ext=*;in="media:binary";op=generate_thumbnail;out="media:binary"`)
	require.NoError(t, err)
	assert.True(t, capWithWildcard.Accepts(request), "Test 7b: Cap with ext=* should match request with ext=wav")
}

// TEST047: Matching semantics - thumbnail fallback with void input
func Test047_matching_semantics_thumbnail_void_input(t *testing.T) {
	outBin := "media:binary"
	cap, err := NewCapUrnFromString(fmt.Sprintf(`cap:in="%s";op=generate_thumbnail;out="%s"`, standard.MediaVoid, outBin))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(fmt.Sprintf(`cap:ext=wav;in="%s";op=generate_thumbnail;out="%s"`, standard.MediaVoid, outBin))
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 7b: Cap missing ext should NOT match request with ext=wav")

	capWithWildcard, err := NewCapUrnFromString(fmt.Sprintf(`cap:ext=*;in="%s";op=generate_thumbnail;out="%s"`, standard.MediaVoid, outBin))
	require.NoError(t, err)
	assert.True(t, capWithWildcard.Accepts(request), "Test 7b: Cap with ext=* should match request with ext=wav")
}

// TEST048: Matching semantics - wildcard direction matches anything
func Test048_matching_semantics_wildcard_direction_matches_anything(t *testing.T) {
	cap, err := NewCapUrnFromString("cap:in=*;out=*")
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:in="media:string";op=generate;out="media:object";ext=pdf`)
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 8: Cap missing op/ext should NOT match (no implicit wildcards)")

	request2, err := NewCapUrnFromString(`cap:in="media:string";out="media:object"`)
	require.NoError(t, err)
	assert.True(t, cap.Accepts(request2), "Test 8b: Same direction specs should match")
}

// TEST049: Non-overlapping tags -- neither direction accepts
func Test049_matching_semantics_cross_dimension_independence(t *testing.T) {
	cap, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)

	request, err := NewCapUrnFromString(testUrn("ext=pdf"))
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 9: Cap missing ext should NOT match request with ext=pdf")

	cap2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	request2, err := NewCapUrnFromString(testUrn("op=generate"))
	require.NoError(t, err)
	assert.True(t, cap2.Accepts(request2), "Test 9b: Same tags should match")
}

// TEST050: Matching semantics - direction mismatch prevents matching
func Test050_matching_semantics_direction_mismatch(t *testing.T) {
	cap, err := NewCapUrnFromString(`cap:in="media:string";op=generate;out="` + standard.MediaObject + `"`)
	require.NoError(t, err)

	request, err := NewCapUrnFromString(`cap:in="media:";op=generate;out="` + standard.MediaObject + `"`)
	require.NoError(t, err)

	assert.False(t, cap.Accepts(request), "Test 10: Direction mismatch should not match")
}

// TEST051: Semantic direction matching - generic provider matches specific request
func Test051_direction_semantic_matching(t *testing.T) {
	genericCap, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	pdfRequest, err := NewCapUrnFromString(
		`cap:in="media:pdf";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	assert.True(t, genericCap.Accepts(pdfRequest),
		"Generic provider must match specific pdf request")

	epubRequest, err := NewCapUrnFromString(
		`cap:in="media:epub";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	assert.True(t, genericCap.Accepts(epubRequest),
		"Generic provider must match epub request")

	pdfCap, err := NewCapUrnFromString(
		`cap:in="media:pdf";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	genericRequest, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	assert.False(t, pdfCap.Accepts(genericRequest),
		"Specific pdf cap must NOT match generic request")

	assert.False(t, pdfCap.Accepts(epubRequest),
		"PDF-specific cap must NOT match epub request (epub lacks pdf marker)")

	specificOutCap, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	genericOutRequest, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image"`,
	)
	require.NoError(t, err)
	assert.True(t, specificOutCap.Accepts(genericOutRequest),
		"Cap producing image;png;thumbnail must satisfy request for image")

	genericOutCap, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image"`,
	)
	require.NoError(t, err)
	specificOutRequest, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	assert.False(t, genericOutCap.Accepts(specificOutRequest),
		"Cap producing generic image must NOT satisfy request requiring image;png;thumbnail")
}

// TEST052: Semantic direction specificity - more media URN tags = higher specificity
func Test052_direction_semantic_specificity(t *testing.T) {
	genericCap, err := NewCapUrnFromString(
		`cap:in="media:";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	specificCap, err := NewCapUrnFromString(
		`cap:in="media:pdf";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)

	assert.Equal(t, 6, genericCap.Specificity())
	assert.Equal(t, 7, specificCap.Specificity())

	assert.True(t, specificCap.Specificity() > genericCap.Specificity(),
		"pdf cap must be more specific than wildcard cap")

	pdfRequest, err := NewCapUrnFromString(
		`cap:in="media:pdf";op=generate_thumbnail;out="media:image;png;thumbnail"`,
	)
	require.NoError(t, err)
	caps := []*CapUrn{genericCap, specificCap}
	matcher := &CapMatcher{}
	best := matcher.FindBestMatch(caps, pdfRequest)
	require.NotNil(t, best)
	assert.Equal(t, 7, best.Specificity(),
		"CapMatcher must prefer the more specific pdf provider")
}

// TEST559: without_tag removes tag, ignores in/out, case-insensitive for keys
func Test559_without_tag(t *testing.T) {
	cap, err := NewCapUrnFromString(
		`cap:in="media:void";op=test;ext=pdf;out="media:void"`,
	)
	require.NoError(t, err)
	removed := cap.WithoutTag("ext")
	_, exists := removed.GetTag("ext")
	assert.False(t, exists)
	opVal, exists := removed.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "test", opVal)

	// Case-insensitive removal
	removed2 := cap.WithoutTag("EXT")
	_, exists = removed2.GetTag("ext")
	assert.False(t, exists)

	// Removing in/out is silently ignored
	same := cap.WithoutTag("in")
	assert.Equal(t, "media:void", same.InSpec())
	same2 := cap.WithoutTag("out")
	assert.Equal(t, "media:void", same2.OutSpec())

	// Removing non-existent tag is no-op
	same3 := cap.WithoutTag("nonexistent")
	assert.True(t, same3.Equals(cap))
}

// TEST560: with_in_spec and with_out_spec change direction specs
func Test560_with_in_out_spec(t *testing.T) {
	cap, err := NewCapUrnFromString(
		`cap:in="media:void";op=test;out="media:void"`,
	)
	require.NoError(t, err)

	changedIn := cap.WithInSpec("media:")
	assert.Equal(t, "media:", changedIn.InSpec())
	assert.Equal(t, "media:void", changedIn.OutSpec())
	opVal, exists := changedIn.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "test", opVal)

	changedOut := cap.WithOutSpec("media:string")
	assert.Equal(t, "media:void", changedOut.InSpec())
	assert.Equal(t, "media:string", changedOut.OutSpec())

	// Chain both
	changedBoth := cap.
		WithInSpec("media:pdf").
		WithOutSpec("media:txt;textable")
	assert.Equal(t, "media:pdf", changedBoth.InSpec())
	assert.Equal(t, "media:txt;textable", changedBoth.OutSpec())
}

// TEST563: CapMatcher::find_all_matches returns all matching caps sorted by specificity
func Test563_find_all_matches(t *testing.T) {
	caps := []*CapUrn{}
	c1, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:void"`)
	require.NoError(t, err)
	caps = append(caps, c1)
	c2, err := NewCapUrnFromString(`cap:in="media:void";op=test;ext=pdf;out="media:void"`)
	require.NoError(t, err)
	caps = append(caps, c2)
	c3, err := NewCapUrnFromString(`cap:in="media:void";op=different;out="media:void"`)
	require.NoError(t, err)
	caps = append(caps, c3)

	request, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:void"`)
	require.NoError(t, err)
	matcher := &CapMatcher{}
	matches := matcher.FindAllMatches(caps, request)

	// Should find 2 matches (op=test and op=test;ext=pdf), not op=different
	assert.Equal(t, 2, len(matches))
	// Sorted by specificity descending: ext=pdf first (more specific)
	assert.True(t, matches[0].Specificity() >= matches[1].Specificity())
	extVal, exists := matches[0].GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", extVal)
}

// TEST564: CapMatcher::are_compatible detects bidirectional overlap
func Test564_are_compatible(t *testing.T) {
	caps1 := []*CapUrn{}
	c1, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:void"`)
	require.NoError(t, err)
	caps1 = append(caps1, c1)

	caps2 := []*CapUrn{}
	c2, err := NewCapUrnFromString(`cap:in="media:void";op=test;ext=pdf;out="media:void"`)
	require.NoError(t, err)
	caps2 = append(caps2, c2)

	caps3 := []*CapUrn{}
	c3, err := NewCapUrnFromString(`cap:in="media:void";op=different;out="media:void"`)
	require.NoError(t, err)
	caps3 = append(caps3, c3)

	matcher := &CapMatcher{}

	// caps1 (op=test) accepts caps2 (op=test;ext=pdf) -> compatible
	assert.True(t, matcher.AreCompatible(caps1, caps2))

	// caps1 (op=test) vs caps3 (op=different) -> not compatible
	assert.False(t, matcher.AreCompatible(caps1, caps3))

	// Empty sets are not compatible
	assert.False(t, matcher.AreCompatible([]*CapUrn{}, caps1))
	assert.False(t, matcher.AreCompatible(caps1, []*CapUrn{}))
}

// TEST565: tags_to_string returns only tags portion without prefix
func Test565_tags_to_string(t *testing.T) {
	cap, err := NewCapUrnFromString(
		`cap:in="media:void";op=test;out="media:void"`,
	)
	require.NoError(t, err)
	tagsStr := cap.ToString()
	// The full string starts with "cap:"
	assert.True(t, len(tagsStr) > 4)
	assert.Contains(t, tagsStr, "op=test")
}

// TEST566: with_tag silently ignores in/out keys
func Test566_with_tag_ignores_in_out(t *testing.T) {
	cap, err := NewCapUrnFromString(
		`cap:in="media:void";op=test;out="media:void"`,
	)
	require.NoError(t, err)
	// Attempting to set in/out via WithTag is silently ignored
	same := cap.WithTag("in", "media:")
	assert.Equal(t, "media:void", same.InSpec(), "WithTag must not change InSpec")

	same2 := cap.WithTag("out", "media:")
	assert.Equal(t, "media:void", same2.OutSpec(), "WithTag must not change OutSpec")
}

// TEST567: conforms_to_str and accepts_str work with string arguments
func Test567_str_variants(t *testing.T) {
	cap, err := NewCapUrnFromString(
		`cap:in="media:void";op=test;out="media:void"`,
	)
	require.NoError(t, err)

	// AcceptsStr
	assert.True(t, cap.AcceptsStr(`cap:in="media:void";op=test;ext=pdf;out="media:void"`))
	assert.False(t, cap.AcceptsStr(`cap:in="media:void";op=different;out="media:void"`))

	// ConformsTo via AcceptsStr (cap.ConformsTo(pattern) == pattern.Accepts(cap))
	pattern, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:void"`)
	require.NoError(t, err)
	assert.True(t, cap.ConformsTo(pattern))

	// Invalid URN string -> false
	assert.False(t, cap.AcceptsStr("invalid"))
}

// TEST639: cap: (empty) defaults to in=media:;out=media:
func Test639_wildcard_empty_cap_defaults_to_media_wildcard(t *testing.T) {
	// In Go, cap: without in/out is rejected with ErrorMissingInSpec
	// This is different from Rust which defaults to media: wildcard
	// Go requires explicit in/out specs
	_, err := NewCapUrnFromString("cap:")
	assert.Error(t, err)
}

// TEST648: Wildcard in/out match specific caps
func Test648_wildcard_accepts_specific(t *testing.T) {
	// In Go, we can create wildcard caps via NewCapUrnFromTags
	wildcard, err := NewCapUrnFromTags(map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, "media:", wildcard.InSpec())
	assert.Equal(t, "media:", wildcard.OutSpec())

	specific, err := NewCapUrnFromString("cap:in=media:pdf;out=media:text")
	require.NoError(t, err)

	assert.True(t, wildcard.Accepts(specific), "Wildcard should accept specific")
	assert.True(t, specific.ConformsTo(wildcard), "Specific should conform to wildcard")
}

// TEST649: Specificity - wildcard has 0, specific has tag count
func Test649_specificity_scoring(t *testing.T) {
	wildcard, err := NewCapUrnFromTags(map[string]string{})
	require.NoError(t, err)

	specific, err := NewCapUrnFromString("cap:in=media:pdf;out=media:text")
	require.NoError(t, err)

	assert.Equal(t, 0, wildcard.Specificity(), "Wildcard cap should have zero specificity")
	assert.True(t, specific.Specificity() > 0, "Specific cap should have non-zero specificity")
}

// TEST651: All identity forms produce the same CapUrn (via NewCapUrnFromTags)
func Test651_identity_forms_equivalent(t *testing.T) {
	// In Go, the identity form is created via NewCapUrnFromTags with empty map
	identity1, err := NewCapUrnFromTags(map[string]string{})
	require.NoError(t, err)

	identity2, err := NewCapUrnFromTags(map[string]string{"in": "*", "out": "*"})
	require.NoError(t, err)

	assert.Equal(t, identity1.InSpec(), identity2.InSpec())
	assert.Equal(t, identity1.OutSpec(), identity2.OutSpec())
	assert.True(t, identity1.Equals(identity2))
}

// TEST652: CAP_IDENTITY constant matches identity caps regardless of string form
func Test652_cap_identity_constant_works(t *testing.T) {
	// In Go, standard.CapIdentity = "cap:" which fails parsing
	// Instead test via NewCapUrnFromTags
	identity, err := NewCapUrnFromTags(map[string]string{})
	require.NoError(t, err)

	specific, err := NewCapUrnFromString("cap:in=media:pdf;out=media:text;op=test")
	require.NoError(t, err)

	// Identity accepts everything (no constraints)
	assert.True(t, identity.Accepts(specific), "Identity accepts specific")
	assert.True(t, specific.ConformsTo(identity), "Specific conforms to identity")
}

// TEST653: Identity (no tags) does not match specific requests via routing
func Test653_identity_routing_isolation(t *testing.T) {
	identity, err := NewCapUrnFromTags(map[string]string{})
	require.NoError(t, err)

	specificRequest, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:void"`)
	require.NoError(t, err)

	// Routing direction: request.Accepts(cap)
	// specificRequest (has op=test) does NOT accept identity (missing op) -> identity NOT routed
	assert.False(t, specificRequest.Accepts(identity),
		"Specific request must NOT accept identity (identity lacks op=test)")

	// But identity request (no constraints) DOES accept specific cap
	identityRequest, err := NewCapUrnFromTags(map[string]string{})
	require.NoError(t, err)
	assert.True(t, identityRequest.Accepts(specificRequest),
		"Identity request (no constraints) matches everything")
}

// JSON serialization test (not numbered in Rust)
func TestCapUrn_JSONSerialization(t *testing.T) {
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
