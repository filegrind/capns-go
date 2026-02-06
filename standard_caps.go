// Package capns provides standard capability URN builders
package capns

// LlmConversationUrn builds a URN for LLM conversation capability
func LlmConversationUrn(langCode string) *CapUrn {
	urn, err := NewCapUrnBuilder().
		Tag("op", "conversation").
		Tag("unconstrained", "*").
		Tag("language", langCode).
		InSpec(MediaString).
		OutSpec(MediaLlmInferenceOutput).
		Build()
	if err != nil {
		panic("Failed to build LLM conversation cap URN: " + err.Error())
	}
	return urn
}

// ModelAvailabilityUrn builds a URN for model-availability capability
func ModelAvailabilityUrn() *CapUrn {
	urn, err := NewCapUrnBuilder().
		Tag("op", "model-availability").
		InSpec(MediaModelSpec).
		OutSpec(MediaAvailabilityOutput).
		Build()
	if err != nil {
		panic("Failed to build model-availability cap URN: " + err.Error())
	}
	return urn
}

// ModelPathUrn builds a URN for model-path capability
func ModelPathUrn() *CapUrn {
	urn, err := NewCapUrnBuilder().
		Tag("op", "model-path").
		InSpec(MediaModelSpec).
		OutSpec(MediaPathOutput).
		Build()
	if err != nil {
		panic("Failed to build model-path cap URN: " + err.Error())
	}
	return urn
}
