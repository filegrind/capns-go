package standard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TEST307: Test ModelAvailabilityUrn builds valid cap URN with correct op and media specs
func Test307_model_availability_urn(t *testing.T) {
	urnStr := ModelAvailabilityUrn()
	assert.True(t, strings.Contains(urnStr, "op=model-availability"), "URN must have op=model-availability")
	assert.True(t, strings.Contains(urnStr, "in=media:model-spec"), "input must be model-spec")
	assert.True(t, strings.Contains(urnStr, "out=media:availability-output"), "output must be availability output")
}

// TEST308: Test ModelPathUrn builds valid cap URN with correct op and media specs
func Test308_model_path_urn(t *testing.T) {
	urnStr := ModelPathUrn()
	assert.True(t, strings.Contains(urnStr, "op=model-path"), "URN must have op=model-path")
	assert.True(t, strings.Contains(urnStr, "in=media:model-spec"), "input must be model-spec")
	assert.True(t, strings.Contains(urnStr, "out=media:path-output"), "output must be path output")
}

// TEST309: Test ModelAvailabilityUrn and ModelPathUrn produce distinct URNs
func Test309_model_availability_and_path_are_distinct(t *testing.T) {
	availStr := ModelAvailabilityUrn()
	pathStr := ModelPathUrn()
	assert.NotEqual(t, availStr, pathStr,
		"availability and path must be distinct cap URNs")
}

// TEST310: Test LlmConversationUrn uses unconstrained tag (not constrained)
func Test310_llm_conversation_urn_unconstrained(t *testing.T) {
	urnStr := LlmConversationUrn("en")
	assert.True(t, strings.Contains(urnStr, "unconstrained"), "LLM conversation URN must have 'unconstrained' tag")
	assert.True(t, strings.Contains(urnStr, "op=conversation"), "must have op=conversation")
	assert.True(t, strings.Contains(urnStr, "language=en"), "must have language=en")
}

// TEST311: Test LlmConversationUrn in/out specs match the expected media URNs semantically
func Test311_llm_conversation_urn_specs(t *testing.T) {
	urnStr := LlmConversationUrn("fr")

	// Verify contains expected media types
	assert.True(t, strings.Contains(urnStr, "in=media:string"), "must have string input")
	assert.True(t, strings.Contains(urnStr, "out=media:llm-inference-output"), "must have llm-inference-output")
}

// TEST312: Test all URN builders produce parseable cap URNs
func Test312_all_urn_builders_produce_valid_urns(t *testing.T) {
	// Each of these must not panic and must start with "cap:"
	availStr := ModelAvailabilityUrn()
	assert.True(t, strings.HasPrefix(availStr, "cap:"), "must be a cap URN")

	pathStr := ModelPathUrn()
	assert.True(t, strings.HasPrefix(pathStr, "cap:"), "must be a cap URN")

	llmStr := LlmConversationUrn("en")
	assert.True(t, strings.HasPrefix(llmStr, "cap:"), "must be a cap URN")
}
