package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapUrnCreation(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:action=transform;format=json;type=data_processing")
	
	assert.NoError(t, err)
	assert.NotNil(t, capUrn)
	
	capType, exists := capUrn.GetTag("type")
	assert.True(t, exists)
	assert.Equal(t, "data_processing", capType)
	
	action, exists := capUrn.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "transform", action)
	
	format, exists := capUrn.GetTag("format")
	assert.True(t, exists)
	assert.Equal(t, "json", format)
}

func TestCanonicalStringFormat(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:action=generate;target=thumbnail;ext=pdf")
	require.NoError(t, err)
	
	// Should be sorted alphabetically and have no trailing semicolon in canonical form
	assert.Equal(t, "cap:action=generate;ext=pdf;target=thumbnail", capUrn.ToString())
}

func TestCapPrefixRequired(t *testing.T) {
	// Missing cap: prefix should fail
	capUrn, err := NewCapUrnFromString("action=generate;ext=pdf")
	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorMissingCapPrefix, err.(*CapUrnError).Code)
	
	// Valid cap: prefix should work
	capUrn, err = NewCapUrnFromString("cap:action=generate;ext=pdf")
	assert.NoError(t, err)
	assert.NotNil(t, capUrn)
	action, exists := capUrn.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
}

func TestTrailingSemicolonEquivalence(t *testing.T) {
	// Both with and without trailing semicolon should be equivalent
	cap1, err := NewCapUrnFromString("cap:action=generate;ext=pdf")
	require.NoError(t, err)
	
	cap2, err := NewCapUrnFromString("cap:action=generate;ext=pdf;")
	require.NoError(t, err)
	
	// They should be equal
	assert.True(t, cap1.Equals(cap2))
	
	// They should have same hash
	assert.Equal(t, cap1.Hash(), cap2.Hash())
	
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

func TestInvalidTagFormat(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:invalid_tag")
	
	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidTagFormat, err.(*CapUrnError).Code)
}

func TestInvalidCharacters(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:type@invalid=value")
	
	assert.Nil(t, capUrn)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidCharacter, err.(*CapUrnError).Code)
}

func TestTagMatching(t *testing.T) {
	cap, err := NewCapUrnFromString("cap:action=generate;ext=pdf;target=thumbnail")
	require.NoError(t, err)
	
	// Exact match
	request1, err := NewCapUrnFromString("cap:action=generate;ext=pdf;target=thumbnail")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1))
	
	// Subset match
	request2, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request2))
	
	// Wildcard match
	request3, err := NewCapUrnFromString("cap:ext=*")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request3))
	
	// No match - conflicting value
	request4, err := NewCapUrnFromString("cap:action=extract")
	require.NoError(t, err)
	assert.False(t, cap.Matches(request4))
}

func TestMissingTagHandling(t *testing.T) {
	cap, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	// Request with missing tag should fail if specific value required
	request1, err := NewCapUrnFromString("cap:ext=pdf")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1)) // cap missing format tag = wildcard, can handle any format
	
	// But cap with extra tags can match subset requests
	cap2, err := NewCapUrnFromString("cap:action=generate;ext=pdf")
	require.NoError(t, err)
	request2, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	assert.True(t, cap2.Matches(request2))
}

func TestSpecificity(t *testing.T) {
	cap1, err := NewCapUrnFromString("cap:action=*")
	require.NoError(t, err)
	
	cap2, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	cap3, err := NewCapUrnFromString("cap:action=*;ext=pdf")
	require.NoError(t, err)
	
	assert.Equal(t, 0, cap1.Specificity()) // wildcard doesn't count
	assert.Equal(t, 1, cap2.Specificity())
	assert.Equal(t, 1, cap3.Specificity()) // only ext=pdf counts, action=* doesn't count
	
	assert.True(t, cap2.IsMoreSpecificThan(cap1))
}

func TestCompatibility(t *testing.T) {
	cap1, err := NewCapUrnFromString("cap:action=generate;ext=pdf")
	require.NoError(t, err)
	
	cap2, err := NewCapUrnFromString("cap:action=generate;format=*")
	require.NoError(t, err)
	
	cap3, err := NewCapUrnFromString("cap:action=extract;ext=pdf")
	require.NoError(t, err)
	
	assert.True(t, cap1.IsCompatibleWith(cap2))
	assert.True(t, cap2.IsCompatibleWith(cap1))
	assert.False(t, cap1.IsCompatibleWith(cap3))
	
	// Missing tags are treated as wildcards for compatibility
	cap4, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	assert.True(t, cap1.IsCompatibleWith(cap4))
	assert.True(t, cap4.IsCompatibleWith(cap1))
}

func TestConvenienceMethods(t *testing.T) {
	cap, err := NewCapUrnFromString("cap:action=generate;ext=pdf;output=binary;target=thumbnail")
	require.NoError(t, err)
	
	action, exists := cap.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
	
	target, exists := cap.GetTag("target")
	assert.True(t, exists)
	assert.Equal(t, "thumbnail", target)
	
	format, exists := cap.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", format)
	
	output, exists := cap.GetTag("output")
	assert.True(t, exists)
	assert.Equal(t, "binary", output)
}

func TestBuilder(t *testing.T) {
	cap, err := NewCapUrnBuilder().
		Tag("action", "generate").
		Tag("target", "thumbnail").
		Tag("ext", "pdf").
		Tag("output", "binary").
		Build()
	require.NoError(t, err)
	
	action, exists := cap.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
	
	output, exists := cap.GetTag("output")
	assert.True(t, exists)
	assert.Equal(t, "binary", output)
}

func TestWithTag(t *testing.T) {
	original, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	modified := original.WithTag("ext", "pdf")
	
	assert.Equal(t, "cap:action=generate;ext=pdf", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "cap:action=generate", original.ToString())
}

func TestWithoutTag(t *testing.T) {
	original, err := NewCapUrnFromString("cap:action=generate;ext=pdf")
	require.NoError(t, err)
	
	modified := original.WithoutTag("ext")
	
	assert.Equal(t, "cap:action=generate", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "cap:action=generate;ext=pdf", original.ToString())
}

func TestWildcardTag(t *testing.T) {
	cap, err := NewCapUrnFromString("cap:ext=pdf")
	require.NoError(t, err)
	
	wildcarded := cap.WithWildcardTag("ext")
	
	assert.Equal(t, "cap:ext=*", wildcarded.ToString())
	
	// Test that wildcarded cap can match more requests
	request, err := NewCapUrnFromString("cap:ext=jpg")
	require.NoError(t, err)
	assert.False(t, cap.Matches(request))
	
	wildcardRequest, err := NewCapUrnFromString("cap:ext=*")
	require.NoError(t, err)
	assert.True(t, wildcarded.Matches(wildcardRequest))
}

func TestSubset(t *testing.T) {
	cap, err := NewCapUrnFromString("cap:action=generate;ext=pdf;output=binary;target=thumbnail;")
	require.NoError(t, err)
	
	subset := cap.Subset([]string{"type", "ext"})
	
	assert.Equal(t, "cap:ext=pdf", subset.ToString())
}

func TestMerge(t *testing.T) {
	cap1, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	cap2, err := NewCapUrnFromString("cap:ext=pdf;output=binary")
	require.NoError(t, err)
	
	merged := cap1.Merge(cap2)
	
	assert.Equal(t, "cap:action=generate;ext=pdf;output=binary", merged.ToString())
}

func TestEquality(t *testing.T) {
	cap1, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	cap2, err := NewCapUrnFromString("cap:action=generate") // different order
	require.NoError(t, err)
	
	cap3, err := NewCapUrnFromString("cap:action=generate;type=image")
	require.NoError(t, err)
	
	assert.True(t, cap1.Equals(cap2)) // order doesn't matter
	assert.False(t, cap1.Equals(cap3))
}

func TestCapMatcher(t *testing.T) {
	matcher := &CapMatcher{}
	
	caps := []*CapUrn{}
	
	cap1, err := NewCapUrnFromString("cap:action=*")
	require.NoError(t, err)
	caps = append(caps, cap1)
	
	cap2, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	caps = append(caps, cap2)
	
	cap3, err := NewCapUrnFromString("cap:action=generate;ext=pdf")
	require.NoError(t, err)
	caps = append(caps, cap3)
	
	request, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	best := matcher.FindBestMatch(caps, request)
	require.NotNil(t, best)
	
	// Most specific cap that can handle the request
	assert.Equal(t, "cap:action=generate;ext=pdf", best.ToString())
}

func TestJSONSerialization(t *testing.T) {
	original, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	
	var decoded CapUrn
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}

func TestCapUrnCaseInsensitive(t *testing.T) {
	// Test that different casing produces the same URN
	cap1, err := NewCapUrnFromString("cap:ACTION=Generate;EXT=PDF;Target=Thumbnail;")
	require.NoError(t, err)
	
	cap2, err := NewCapUrnFromString("cap:action=generate;ext=pdf;target=thumbnail;")
	require.NoError(t, err)
	
	// Both should be normalized to lowercase
	action, exists := cap1.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
	
	ext, exists := cap1.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)
	
	target, exists := cap1.GetTag("target")
	assert.True(t, exists)
	assert.Equal(t, "thumbnail", target)
	
	// URNs should be identical after normalization
	assert.Equal(t, cap1.ToString(), cap2.ToString())
	
	// PartialEq should work correctly - URNs with different case should be equal
	assert.True(t, cap1.Equals(cap2))
	
	// Case-insensitive tag lookup should work
	action2, exists := cap1.GetTag("ACTION")
	assert.True(t, exists)
	assert.Equal(t, "generate", action2)
	
	action3, exists := cap1.GetTag("Action")
	assert.True(t, exists)
	assert.Equal(t, "generate", action3)
	
	assert.True(t, cap1.HasTag("ACTION", "Generate"))
	assert.True(t, cap1.HasTag("action", "GENERATE"))
	
	// Matching should work case-insensitively
	assert.True(t, cap1.Matches(cap2))
	assert.True(t, cap2.Matches(cap1))
}

func TestCapUrnBuilderCaseInsensitive(t *testing.T) {
	cap, err := NewCapUrnBuilder().
		Tag("ACTION", "Generate").
		Tag("Target", "Thumbnail").
		Tag("EXT", "PDF").
		Tag("output", "BINARY").
		Build()
	require.NoError(t, err)
	
	// All tags should be normalized to lowercase
	action, exists := cap.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
	
	output, exists := cap.GetTag("output")
	assert.True(t, exists)
	assert.Equal(t, "binary", output)
	
	// Should be able to retrieve with different case
	action2, exists := cap.GetTag("ACTION")
	assert.True(t, exists)
	assert.Equal(t, "generate", action2)
}

func TestCapUrnWithTagCaseInsensitive(t *testing.T) {
	original, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	modified := original.WithTag("EXT", "PDF")
	
	// Tag should be normalized to lowercase
	ext, exists := modified.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext)
	
	// Should be retrievable with different case
	ext2, exists := modified.GetTag("EXT")
	assert.True(t, exists)
	assert.Equal(t, "pdf", ext2)
	
	assert.Equal(t, "cap:action=generate;ext=pdf", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "cap:action=generate", original.ToString())
}