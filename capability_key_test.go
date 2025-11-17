package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityKeyCreation(t *testing.T) {
	capKey, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	
	assert.NoError(t, err)
	assert.NotNil(t, capKey)
	
	capType, exists := capKey.GetTag("type")
	assert.True(t, exists)
	assert.Equal(t, "data_processing", capType)
	
	action, exists := capKey.GetTag("action")
	assert.True(t, exists)
	assert.Equal(t, "transform", action)
	
	format, exists := capKey.GetTag("format")
	assert.True(t, exists)
	assert.Equal(t, "json", format)
}

func TestCanonicalStringFormat(t *testing.T) {
	capKey, err := NewCapabilityKeyFromString("type=document;action=generate;target=thumbnail;format=pdf")
	require.NoError(t, err)
	
	// Should be sorted alphabetically
	assert.Equal(t, "action=generate;format=pdf;target=thumbnail;type=document", capKey.ToString())
}

func TestInvalidCapabilityKey(t *testing.T) {
	capKey, err := NewCapabilityKeyFromString("")
	
	assert.Nil(t, capKey)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidFormat, err.(*CapabilityKeyError).Code)
}

func TestInvalidTagFormat(t *testing.T) {
	capKey, err := NewCapabilityKeyFromString("type=document;invalid_tag")
	
	assert.Nil(t, capKey)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidTagFormat, err.(*CapabilityKeyError).Code)
}

func TestInvalidCharacters(t *testing.T) {
	capKey, err := NewCapabilityKeyFromString("type@invalid=value")
	
	assert.Nil(t, capKey)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidCharacter, err.(*CapabilityKeyError).Code)
}

func TestTagMatching(t *testing.T) {
	cap, err := NewCapabilityKeyFromString("action=generate;format=pdf;target=thumbnail;type=document")
	require.NoError(t, err)
	
	// Exact match
	request1, err := NewCapabilityKeyFromString("action=generate;format=pdf;target=thumbnail;type=document")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1))
	
	// Subset match
	request2, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request2))
	
	// Wildcard match
	request3, err := NewCapabilityKeyFromString("type=document;format=*")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request3))
	
	// No match - conflicting value
	request4, err := NewCapabilityKeyFromString("type=image")
	require.NoError(t, err)
	assert.False(t, cap.Matches(request4))
}

func TestMissingTagHandling(t *testing.T) {
	cap, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	
	// Request with missing tag should fail if specific value required
	request1, err := NewCapabilityKeyFromString("type=document;format=pdf")
	require.NoError(t, err)
	assert.True(t, cap.Matches(request1)) // cap missing format tag = wildcard, can handle any format
	
	// But capability with extra tags can match subset requests
	cap2, err := NewCapabilityKeyFromString("type=document;action=generate;format=pdf")
	require.NoError(t, err)
	request2, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	assert.True(t, cap2.Matches(request2))
}

func TestSpecificity(t *testing.T) {
	cap1, err := NewCapabilityKeyFromString("type=document")
	require.NoError(t, err)
	
	cap2, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	
	cap3, err := NewCapabilityKeyFromString("type=document;action=*;format=pdf")
	require.NoError(t, err)
	
	assert.Equal(t, 1, cap1.Specificity())
	assert.Equal(t, 2, cap2.Specificity())
	assert.Equal(t, 2, cap3.Specificity()) // wildcard doesn't count
	
	assert.True(t, cap2.IsMoreSpecificThan(cap1))
}

func TestCompatibility(t *testing.T) {
	cap1, err := NewCapabilityKeyFromString("type=document;action=generate;format=pdf")
	require.NoError(t, err)
	
	cap2, err := NewCapabilityKeyFromString("type=document;action=generate;format=*")
	require.NoError(t, err)
	
	cap3, err := NewCapabilityKeyFromString("type=image;action=generate")
	require.NoError(t, err)
	
	assert.True(t, cap1.IsCompatibleWith(cap2))
	assert.True(t, cap2.IsCompatibleWith(cap1))
	assert.False(t, cap1.IsCompatibleWith(cap3))
	
	// Missing tags are treated as wildcards for compatibility
	cap4, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	assert.True(t, cap1.IsCompatibleWith(cap4))
	assert.True(t, cap4.IsCompatibleWith(cap1))
}

func TestConvenienceMethods(t *testing.T) {
	cap, err := NewCapabilityKeyFromString("action=generate;format=pdf;output=binary;target=thumbnail;type=document")
	require.NoError(t, err)
	
	capType, exists := cap.GetType()
	assert.True(t, exists)
	assert.Equal(t, "document", capType)
	
	action, exists := cap.GetAction()
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
	
	target, exists := cap.GetTarget()
	assert.True(t, exists)
	assert.Equal(t, "thumbnail", target)
	
	format, exists := cap.GetFormat()
	assert.True(t, exists)
	assert.Equal(t, "pdf", format)
	
	output, exists := cap.GetOutput()
	assert.True(t, exists)
	assert.Equal(t, "binary", output)
	
	assert.True(t, cap.IsBinaryOutput())
}

func TestBuilder(t *testing.T) {
	cap, err := NewCapabilityKeyBuilder().
		Type("document").
		Action("generate").
		Target("thumbnail").
		Format("pdf").
		BinaryOutput().
		Build()
	require.NoError(t, err)
	
	capType, exists := cap.GetType()
	assert.True(t, exists)
	assert.Equal(t, "document", capType)
	
	action, exists := cap.GetAction()
	assert.True(t, exists)
	assert.Equal(t, "generate", action)
	
	assert.True(t, cap.IsBinaryOutput())
}

func TestWithTag(t *testing.T) {
	original, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	
	modified := original.WithTag("format", "pdf")
	
	assert.Equal(t, "action=generate;format=pdf;type=document", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "action=generate;type=document", original.ToString())
}

func TestWithoutTag(t *testing.T) {
	original, err := NewCapabilityKeyFromString("action=generate;format=pdf;type=document")
	require.NoError(t, err)
	
	modified := original.WithoutTag("format")
	
	assert.Equal(t, "action=generate;type=document", modified.ToString())
	
	// Original should be unchanged
	assert.Equal(t, "action=generate;format=pdf;type=document", original.ToString())
}

func TestWildcardTag(t *testing.T) {
	cap, err := NewCapabilityKeyFromString("type=document;format=pdf")
	require.NoError(t, err)
	
	wildcarded := cap.WithWildcardTag("format")
	
	assert.Equal(t, "format=*;type=document", wildcarded.ToString())
	
	// Test that wildcarded capability can match more requests
	request, err := NewCapabilityKeyFromString("type=document;format=jpg")
	require.NoError(t, err)
	assert.False(t, cap.Matches(request))
	
	wildcardRequest, err := NewCapabilityKeyFromString("type=document;format=*")
	require.NoError(t, err)
	assert.True(t, wildcarded.Matches(wildcardRequest))
}

func TestSubset(t *testing.T) {
	cap, err := NewCapabilityKeyFromString("action=generate;format=pdf;output=binary;target=thumbnail;type=document")
	require.NoError(t, err)
	
	subset := cap.Subset([]string{"type", "format"})
	
	assert.Equal(t, "format=pdf;type=document", subset.ToString())
}

func TestMerge(t *testing.T) {
	cap1, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	
	cap2, err := NewCapabilityKeyFromString("format=pdf;output=binary")
	require.NoError(t, err)
	
	merged := cap1.Merge(cap2)
	
	assert.Equal(t, "action=generate;format=pdf;output=binary;type=document", merged.ToString())
}

func TestEquality(t *testing.T) {
	cap1, err := NewCapabilityKeyFromString("action=generate;type=document")
	require.NoError(t, err)
	
	cap2, err := NewCapabilityKeyFromString("type=document;action=generate") // different order
	require.NoError(t, err)
	
	cap3, err := NewCapabilityKeyFromString("action=generate;type=image")
	require.NoError(t, err)
	
	assert.True(t, cap1.Equals(cap2)) // order doesn't matter
	assert.False(t, cap1.Equals(cap3))
}

func TestCapabilityMatcher(t *testing.T) {
	matcher := &CapabilityMatcher{}
	
	capabilities := []*CapabilityKey{}
	
	cap1, err := NewCapabilityKeyFromString("type=document")
	require.NoError(t, err)
	capabilities = append(capabilities, cap1)
	
	cap2, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	capabilities = append(capabilities, cap2)
	
	cap3, err := NewCapabilityKeyFromString("type=document;action=generate;format=pdf")
	require.NoError(t, err)
	capabilities = append(capabilities, cap3)
	
	request, err := NewCapabilityKeyFromString("type=document;action=generate")
	require.NoError(t, err)
	
	best := matcher.FindBestMatch(capabilities, request)
	require.NotNil(t, best)
	
	// Most specific capability that can handle the request
	assert.Equal(t, "action=generate;format=pdf;type=document", best.ToString())
}

func TestJSONSerialization(t *testing.T) {
	original, err := NewCapabilityKeyFromString("action=generate;type=document")
	require.NoError(t, err)
	
	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	
	var decoded CapabilityKey
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}