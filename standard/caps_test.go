package standard

import (
	"testing"

	taggedurn "github.com/filegrind/tagged-urn-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST307: Test ModelAvailabilityUrn builds valid cap URN with correct op and media specs
func TestModelAvailabilityUrn(t *testing.T) {
	urn := ModelAvailabilityUrn()
	assert.True(t, urn.HasTag("op", "model-availability"), "URN must have op=model-availability")
	assert.Equal(t, MediaModelSpec, urn.InSpec(), "input must be model-spec")
	assert.Equal(t, MediaAvailabilityOutput, urn.OutSpec(), "output must be availability output")
}

// TEST308: Test ModelPathUrn builds valid cap URN with correct op and media specs
func TestModelPathUrn(t *testing.T) {
	urn := ModelPathUrn()
	assert.True(t, urn.HasTag("op", "model-path"), "URN must have op=model-path")
	assert.Equal(t, MediaModelSpec, urn.InSpec(), "input must be model-spec")
	assert.Equal(t, MediaPathOutput, urn.OutSpec(), "output must be path output")
}

// TEST309: Test ModelAvailabilityUrn and ModelPathUrn produce distinct URNs
func TestModelAvailabilityAndPathAreDistinct(t *testing.T) {
	avail := ModelAvailabilityUrn()
	path := ModelPathUrn()
	assert.NotEqual(t, avail.ToString(), path.ToString(),
		"availability and path must be distinct cap URNs")
}

// TEST310: Test LlmConversationUrn uses unconstrained tag (not constrained)
func TestLlmConversationUrnUnconstrained(t *testing.T) {
	urn := LlmConversationUrn("en")
	_, hasUnconstrained := urn.GetTag("unconstrained")
	assert.True(t, hasUnconstrained, "LLM conversation URN must have 'unconstrained' tag")
	assert.True(t, urn.HasTag("op", "conversation"), "must have op=conversation")
	assert.True(t, urn.HasTag("language", "en"), "must have language=en")
}

// TEST311: Test LlmConversationUrn in/out specs match the expected media URNs semantically
func TestLlmConversationUrnSpecs(t *testing.T) {
	urn := LlmConversationUrn("fr")

	// Compare semantically via TaggedUrn matching (tag order may differ)
	inSpec, err := taggedurn.NewTaggedUrnFromString(urn.InSpec())
	require.NoError(t, err, "in_spec must parse")
	expectedIn, err := taggedurn.NewTaggedUrnFromString(standard.MediaString)
	require.NoError(t, err, "standard.MediaString must parse")
	matches, err := inSpec.ConformsTo(expectedIn)
	require.NoError(t, err)
	assert.True(t, matches,
		"in_spec '%s' must match standard.MediaString '%s'", urn.InSpec(), standard.MediaString)

	outSpec, err := taggedurn.NewTaggedUrnFromString(urn.OutSpec())
	require.NoError(t, err, "out_spec must parse")
	expectedOut, err := taggedurn.NewTaggedUrnFromString(MediaLlmInferenceOutput)
	require.NoError(t, err, "MediaLlmInferenceOutput must parse")
	matches, err = outSpec.ConformsTo(expectedOut)
	require.NoError(t, err)
	assert.True(t, matches,
		"out_spec '%s' must match '%s'", urn.OutSpec(), MediaLlmInferenceOutput)
}

// TEST312: Test all URN builders produce parseable cap URNs
func TestAllUrnBuildersProduceValidUrns(t *testing.T) {
	// Each of these must not panic
	_ = ModelAvailabilityUrn()
	_ = ModelPathUrn()
	_ = LlmConversationUrn("en")

	// Verify they roundtrip through CapUrn parsing
	availStr := ModelAvailabilityUrn().ToString()
	_, err := NewCapUrnFromString(availStr)
	assert.NoError(t, err, "ModelAvailabilityUrn must be parseable: %v", err)

	pathStr := ModelPathUrn().ToString()
	_, err = NewCapUrnFromString(pathStr)
	assert.NoError(t, err, "ModelPathUrn must be parseable: %v", err)
}
