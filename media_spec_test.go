package capns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataPropagationFromObjectDef(t *testing.T) {
	// Create a media spec definition with metadata
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:custom-setting;setting",
			MediaType:   "text/plain",
			ProfileURI:  "https://example.com/schema",
			Title:       "Custom Setting",
			Description: "A custom setting",
			Metadata: map[string]interface{}{
				"category_key":    "interface",
				"ui_type":         "SETTING_UI_TYPE_CHECKBOX",
				"subcategory_key": "appearance",
				"display_index":   5,
			},
		},
	}

	// Resolve and verify metadata is propagated
	resolved, err := ResolveMediaUrn("media:custom-setting;setting", mediaSpecs)
	require.NoError(t, err)
	require.NotNil(t, resolved.Metadata)

	assert.Equal(t, "interface", resolved.Metadata["category_key"])
	assert.Equal(t, "SETTING_UI_TYPE_CHECKBOX", resolved.Metadata["ui_type"])
	assert.Equal(t, "appearance", resolved.Metadata["subcategory_key"])
	assert.Equal(t, 5, resolved.Metadata["display_index"])
}

func TestMetadataNilByDefault(t *testing.T) {
	// Media URNs with no metadata field should have nil metadata
	mediaSpecs := []MediaSpecDef{
		{
			Urn:        MediaString,
			MediaType:  "text/plain",
			ProfileURI: ProfileStr,
		},
	}
	resolved, err := ResolveMediaUrn(MediaString, mediaSpecs)
	require.NoError(t, err)
	assert.Nil(t, resolved.Metadata)
}

func TestMetadataWithValidation(t *testing.T) {
	// Ensure metadata and validation can coexist
	minVal := 0.0
	maxVal := 100.0
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:bounded-number;numeric;setting",
			MediaType:   "text/plain",
			ProfileURI:  "https://example.com/schema",
			Title:       "Bounded Number",
			Description: "",
			Validation: &MediaValidation{
				Min: &minVal,
				Max: &maxVal,
			},
			Metadata: map[string]interface{}{
				"category_key": "inference",
				"ui_type":      "SETTING_UI_TYPE_SLIDER",
			},
		},
	}

	resolved, err := ResolveMediaUrn("media:bounded-number;numeric;setting", mediaSpecs)
	require.NoError(t, err)

	// Verify validation
	require.NotNil(t, resolved.Validation)
	assert.Equal(t, 0.0, *resolved.Validation.Min)
	assert.Equal(t, 100.0, *resolved.Validation.Max)

	// Verify metadata
	require.NotNil(t, resolved.Metadata)
	assert.Equal(t, "inference", resolved.Metadata["category_key"])
	assert.Equal(t, "SETTING_UI_TYPE_SLIDER", resolved.Metadata["ui_type"])
}

func TestResolveMediaUrnNotFound(t *testing.T) {
	// Should fail hard for unknown media URNs
	mediaSpecs := []MediaSpecDef{}
	_, err := ResolveMediaUrn("media:unknown;type", mediaSpecs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be resolved")
}

// Test extensions field propagation from object def to resolved media spec
func TestExtensionsPropagationFromObjectDef(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:pdf;bytes",
			MediaType:   "application/pdf",
			ProfileURI:  "https://capns.org/schema/pdf",
			Title:       "PDF Document",
			Description: "A PDF document",
			Extensions:  []string{"pdf"},
		},
	}

	resolved, err := ResolveMediaUrn("media:pdf;bytes", mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, []string{"pdf"}, resolved.Extensions)
}

// Test extensions is empty slice when not set
func TestExtensionsEmptyWhenNotSet(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		{
			Urn:        "media:text;textable",
			MediaType:  "text/plain",
			ProfileURI: "https://example.com",
		},
	}

	resolved, err := ResolveMediaUrn("media:text;textable", mediaSpecs)
	require.NoError(t, err)
	assert.Empty(t, resolved.Extensions)
}

// Test extensions can coexist with metadata and validation
func TestExtensionsWithMetadataAndValidation(t *testing.T) {
	minLen := 1
	maxLen := 1000
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:custom-output",
			MediaType:   "application/json",
			ProfileURI:  "https://example.com/schema",
			Title:       "Custom Output",
			Description: "",
			Validation: &MediaValidation{
				MinLength: &minLen,
				MaxLength: &maxLen,
			},
			Metadata: map[string]interface{}{
				"category": "output",
			},
			Extensions: []string{"json"},
		},
	}

	resolved, err := ResolveMediaUrn("media:custom-output", mediaSpecs)
	require.NoError(t, err)

	// Verify all fields are present
	require.NotNil(t, resolved.Validation)
	require.NotNil(t, resolved.Metadata)
	assert.Equal(t, []string{"json"}, resolved.Extensions)
}

// Test multiple extensions in a media spec
func TestMultipleExtensions(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:image;jpeg;bytes",
			MediaType:   "image/jpeg",
			ProfileURI:  "https://capns.org/schema/jpeg",
			Title:       "JPEG Image",
			Description: "JPEG image data",
			Extensions:  []string{"jpg", "jpeg"},
		},
	}

	resolved, err := ResolveMediaUrn("media:image;jpeg;bytes", mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, []string{"jpg", "jpeg"}, resolved.Extensions)
	assert.Len(t, resolved.Extensions, 2)
}

// Test ValidateNoMediaSpecDuplicates function
func TestValidateNoMediaSpecDuplicates(t *testing.T) {
	// Test case 1: No duplicates
	mediaSpecs := []MediaSpecDef{
		{Urn: "media:text;textable", MediaType: "text/plain"},
		{Urn: "media:json;textable", MediaType: "application/json"},
	}
	err := ValidateNoMediaSpecDuplicates(mediaSpecs)
	assert.NoError(t, err)

	// Test case 2: With duplicates
	mediaSpecsWithDupes := []MediaSpecDef{
		{Urn: "media:text;textable", MediaType: "text/plain"},
		{Urn: "media:json;textable", MediaType: "application/json"},
		{Urn: "media:text;textable", MediaType: "text/html"}, // Duplicate URN
	}
	err = ValidateNoMediaSpecDuplicates(mediaSpecsWithDupes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "media:text;textable")

	// Test case 3: Empty array
	err = ValidateNoMediaSpecDuplicates([]MediaSpecDef{})
	assert.NoError(t, err)
}

// Test NewMediaSpecDef constructor
func TestNewMediaSpecDef(t *testing.T) {
	def := NewMediaSpecDef("media:test;textable", "text/plain", "https://example.com/schema")
	assert.Equal(t, "media:test;textable", def.Urn)
	assert.Equal(t, "text/plain", def.MediaType)
	assert.Equal(t, "https://example.com/schema", def.ProfileURI)
	assert.Empty(t, def.Title)
	assert.Empty(t, def.Description)
	assert.Nil(t, def.Schema)
}

// Test NewMediaSpecDefWithTitle constructor
func TestNewMediaSpecDefWithTitle(t *testing.T) {
	def := NewMediaSpecDefWithTitle("media:test;textable", "text/plain", "https://example.com/schema", "Test Title")
	assert.Equal(t, "media:test;textable", def.Urn)
	assert.Equal(t, "text/plain", def.MediaType)
	assert.Equal(t, "https://example.com/schema", def.ProfileURI)
	assert.Equal(t, "Test Title", def.Title)
}

// Test NewMediaSpecDefWithSchema constructor
func TestNewMediaSpecDefWithSchema(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	def := NewMediaSpecDefWithSchema("media:test;json", "application/json", "https://example.com/schema", schema)
	assert.Equal(t, "media:test;json", def.Urn)
	assert.Equal(t, "application/json", def.MediaType)
	assert.Equal(t, "https://example.com/schema", def.ProfileURI)
	assert.NotNil(t, def.Schema)
}

// TEST304: Test MediaAvailabilityOutput constant parses as valid media URN with correct tags
func TestMediaAvailabilityOutputConstant(t *testing.T) {
	assert.True(t, HasMediaUrnTag(MediaAvailabilityOutput, "textable"),
		"model-availability must be textable")
	assert.True(t, HasMediaUrnTagValue(MediaAvailabilityOutput, "form", "map"),
		"model-availability must be form=map")
	assert.False(t, HasMediaUrnTag(MediaAvailabilityOutput, "bytes"),
		"model-availability must not be binary")
}

// TEST305: Test MediaPathOutput constant parses as valid media URN with correct tags
func TestMediaPathOutputConstant(t *testing.T) {
	assert.True(t, HasMediaUrnTag(MediaPathOutput, "textable"),
		"model-path must be textable")
	assert.True(t, HasMediaUrnTagValue(MediaPathOutput, "form", "map"),
		"model-path must be form=map")
	assert.False(t, HasMediaUrnTag(MediaPathOutput, "bytes"),
		"model-path must not be binary")
}

// TEST306: Test MediaAvailabilityOutput and MediaPathOutput are distinct URNs
func TestAvailabilityAndPathOutputDistinct(t *testing.T) {
	assert.NotEqual(t, MediaAvailabilityOutput, MediaPathOutput,
		"availability and path output must be distinct media URNs")
	// They must NOT be the same type (different model-availability vs model-path marker tags)
	assert.True(t, HasMediaUrnTag(MediaAvailabilityOutput, "model-availability"),
		"availability must have model-availability tag")
	assert.True(t, HasMediaUrnTag(MediaPathOutput, "model-path"),
		"path must have model-path tag")
}
