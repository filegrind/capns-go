package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapCardCreation(t *testing.T) {
	capCard, err := NewCapCardFromString("action=transform;format=json;type=data_processing")
	
	assert.NoError(t, err)
	assert.NotNil(t, capCard)
	
	capType, exists := capCard.GetTag("type")
	assert.True(t, exists)
	assert.Equal(t, "data_processing", capType)
	
	action, exists := capCard.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "transform", action)
	
	format, exists := capCard.GetTag("ext")
	assert.True(t, exists)
	assert.Equal(t, "json", format)
}

func TestCanonicalStringFormat(t *testing.T) {
	capCard, err := NewCapCardFromString("action=generate;target=thumbnail;ext=pdf")
	require.NoError(t, err)
	
	// Should be sorted alphabetically
	assert.Equal(t, "action=generate;ext=pdf;target=thumbnail;", capCard.ToString())
}

func TestInvalidCapCard(t *testing.T) {
	capCard, err := NewCapCardFromString("")
	
	assert.Nil(t, capCard)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidFormat, err.(*CapCardError).Code)
}

func TestInvalidTagFormat(t *testing.T) {
	capCard, err := NewCapCardFromString("invalid_tag")
	
	assert.Nil(t, capCard)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidTagFormat, err.(*CapCardError).Code)
}

func TestInvalidCharacters(t *testing.T) {
	capCard, err := NewCapCardFromString("type@invalid=value")
	
	assert.Nil(t, capCard)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidCharacter, err.(*CapCardError).Code)
}

func TestTagMatching(t *testing.T) {
	cap, err := NewCapCardFromString("action=generate;ext=pdf;target=thumbnail;")
	require.NoError(t, err)
	
	// Exact match
	request1, err := NewCapCardFromString("action=generate;ext=pdf;target=thumbnail;")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1))
	
	// Subset match
	request2, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request2))
	
	// Wildcard match
	request3, err := NewCapCardFromString("format=*")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request3))
	
	// No match - conflicting value
	request4, err := NewCapCardFromString("type=image")
	require.NoError(t, err)
	assert.False(t, cap.Matches(request4))
}

func TestMissingTagHandling(t *testing.T) {
	cap, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	
	// Request with missing tag should fail if specific value required
	request1, err := NewCapCardFromString("ext=pdf")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1)) // cap missing format tag = wildcard, can handle any format
	
	// But cap with extra tags can match subset requests
	cap2, err := NewCapCardFromString("action=generate;ext=pdf")
	require.NoError(t, err)
	request2, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	assert.True(t, cap2.Matches(request2))
}

func TestSpecificity(t *testing.T) {
	cap1, err := NewCapCardFromString("")
	require.NoError(t, err)
	
	cap2, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	
	cap3, err := NewCapCardFromString("action=*;ext=pdf")
	require.NoError(t, err)
	
	assert.Equal(t, 1, cap1.Specificity())
	assert.Equal(t, 2, cap2.Specificity())
	assert.Equal(t, 2, cap3.Specificity()) // wildcard doesn't count
	
	assert.True(t, cap2.IsMoreSpecificThan(cap1))
}

func TestCompatibility(t *testing.T) {
	cap1, err := NewCapCardFromString("action=generate;ext=pdf")
	require.NoError(t, err)
	
	cap2, err := NewCapCardFromString("action=generate;format=*")
	require.NoError(t, err)
	
	cap3, err := NewCapCardFromString("type=image;action=generate")
	require.NoError(t, err)
	
	assert.True(t, cap1.IsCompatibleWith(cap2))
	assert.True(t, cap2.IsCompatibleWith(cap1))
	assert.False(t, cap1.IsCompatibleWith(cap3))
	
	// Missing tags are treated as wildcards for compatibility
	cap4, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	assert.True(t, cap1.IsCompatibleWith(cap4))
	assert.True(t, cap4.IsCompatibleWith(cap1))
}

func TestConvenienceMethods(t *testing.T) {
	cap, err := NewCapCardFromString("action=generate;ext=pdf;output=binary;target=thumbnail;")
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
	cap, err := NewCapCardBuilder().
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
	original, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	
	modified := original.WithTag("ext", "pdf")
	
	assert.Equal(t, "action=generate;ext=pdf;", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "action=generate;", original.ToString())
}

func TestWithoutTag(t *testing.T) {
	original, err := NewCapCardFromString("action=generate;ext=pdf;")
	require.NoError(t, err)
	
	modified := original.WithoutTag("ext")
	
	assert.Equal(t, "action=generate;", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "action=generate;ext=pdf;", original.ToString())
}

func TestWildcardTag(t *testing.T) {
	cap, err := NewCapCardFromString("ext=pdf")
	require.NoError(t, err)
	
	wildcarded := cap.WithWildcardTag("ext")
	
	assert.Equal(t, "format=*;", wildcarded.ToString())
	
	// Test that wildcarded cap can match more requests
	request, err := NewCapCardFromString("format=jpg")
	require.NoError(t, err)
	assert.False(t, cap.Matches(request))
	
	wildcardRequest, err := NewCapCardFromString("format=*")
	require.NoError(t, err)
	assert.True(t, wildcarded.Matches(wildcardRequest))
}

func TestSubset(t *testing.T) {
	cap, err := NewCapCardFromString("action=generate;ext=pdf;output=binary;target=thumbnail;")
	require.NoError(t, err)
	
	subset := cap.Subset([]string{"type", "ext"})
	
	assert.Equal(t, "ext=pdf;", subset.ToString())
}

func TestMerge(t *testing.T) {
	cap1, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	
	cap2, err := NewCapCardFromString("ext=pdf;output=binary")
	require.NoError(t, err)
	
	merged := cap1.Merge(cap2)
	
	assert.Equal(t, "action=generate;ext=pdf;output=binary;", merged.ToString())
}

func TestEquality(t *testing.T) {
	cap1, err := NewCapCardFromString("action=generate;")
	require.NoError(t, err)
	
	cap2, err := NewCapCardFromString("action=generate") // different order
	require.NoError(t, err)
	
	cap3, err := NewCapCardFromString("action=generate;type=image")
	require.NoError(t, err)
	
	assert.True(t, cap1.Equals(cap2)) // order doesn't matter
	assert.False(t, cap1.Equals(cap3))
}

func TestCapMatcher(t *testing.T) {
	matcher := &CapMatcher{}
	
	caps := []*CapCard{}
	
	cap1, err := NewCapCardFromString("")
	require.NoError(t, err)
	caps = append(caps, cap1)
	
	cap2, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	caps = append(caps, cap2)
	
	cap3, err := NewCapCardFromString("action=generate;ext=pdf")
	require.NoError(t, err)
	caps = append(caps, cap3)
	
	request, err := NewCapCardFromString("action=generate")
	require.NoError(t, err)
	
	best := matcher.FindBestMatch(caps, request)
	require.NotNil(t, best)
	
	// Most specific cap that can handle the request
	assert.Equal(t, "action=generate;ext=pdf;", best.ToString())
}

func TestJSONSerialization(t *testing.T) {
	original, err := NewCapCardFromString("action=generate;")
	require.NoError(t, err)
	
	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	
	var decoded CapCard
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}