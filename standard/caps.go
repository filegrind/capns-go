// Package standard provides standard capability URN builders
package standard

// =============================================================================
// STANDARD CAP URN CONSTANTS
// =============================================================================

// CapIdentity is the standard identity capability URN
// Accepts any media type as input and outputs the same type
const CapIdentity = "cap:in=media:;out=media:"

// CapDiscard is the standard discard capability URN
// Accepts any media type as input and produces void output
const CapDiscard = "cap:in=media:;out=media:void"

// =============================================================================
// STANDARD CAP URN BUILDERS
// These return URN strings that can be parsed with urn.NewCapUrnFromString()
// =============================================================================

// LlmConversationUrn builds a URN string for LLM conversation capability
func LlmConversationUrn(langCode string) string {
	return "cap:op=conversation;unconstrained=*;language=" + langCode + ";in=media:string;out=media:llm-inference-output"
}

// ModelAvailabilityUrn builds a URN string for model-availability capability
func ModelAvailabilityUrn() string {
	return "cap:op=model-availability;in=media:model-spec;out=media:availability-output"
}

// ModelPathUrn builds a URN string for model-path capability
func ModelPathUrn() string {
	return "cap:op=model-path;in=media:model-spec;out=media:path-output"
}
