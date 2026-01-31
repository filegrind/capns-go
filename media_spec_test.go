package capns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataPropagationFromObjectDef(t *testing.T) {
	// Create a media spec definition with metadata
	mediaSpecs := map[string]MediaSpecDef{
		"media:custom-setting;setting": {
			IsString: false,
			ObjectValue: &MediaSpecDefObject{
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

func TestMetadataNoneForStringDef(t *testing.T) {
	// String form definitions should have no metadata
	mediaSpecs := map[string]MediaSpecDef{
		"media:simple;textable": NewMediaSpecDefString("text/plain; profile=https://example.com"),
	}

	resolved, err := ResolveMediaUrn("media:simple;textable", mediaSpecs)
	require.NoError(t, err)
	assert.Nil(t, resolved.Metadata)
}

func TestMetadataNoneByDefault(t *testing.T) {
	// Media URNs with simple string definitions should have no metadata
	mediaSpecs := map[string]MediaSpecDef{
		MediaString: NewMediaSpecDefString("text/plain; profile=" + ProfileStr),
	}
	resolved, err := ResolveMediaUrn(MediaString, mediaSpecs)
	require.NoError(t, err)
	assert.Nil(t, resolved.Metadata)
}

func TestMetadataWithValidation(t *testing.T) {
	// Ensure metadata and validation can coexist
	minVal := 0.0
	maxVal := 100.0
	mediaSpecs := map[string]MediaSpecDef{
		"media:bounded-number;numeric;setting": {
			IsString: false,
			ObjectValue: &MediaSpecDefObject{
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
	mediaSpecs := map[string]MediaSpecDef{}
	_, err := ResolveMediaUrn("media:unknown;type", mediaSpecs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be resolved")
}

// Test extension field propagation from object def to resolved media spec
func TestExtensionPropagationFromObjectDef(t *testing.T) {
	mediaSpecs := map[string]MediaSpecDef{
		"media:pdf;bytes": {
			IsString: false,
			ObjectValue: &MediaSpecDefObject{
				MediaType:   "application/pdf",
				ProfileURI:  "https://capns.org/schema/pdf",
				Title:       "PDF Document",
				Description: "A PDF document",
				Extension:   "pdf",
			},
		},
	}

	resolved, err := ResolveMediaUrn("media:pdf;bytes", mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, "pdf", resolved.Extension)
}

// Test extension is empty string for string form media spec definitions
func TestExtensionEmptyForStringDef(t *testing.T) {
	mediaSpecs := map[string]MediaSpecDef{
		"media:text;textable": NewMediaSpecDefString("text/plain; profile=https://example.com"),
	}

	resolved, err := ResolveMediaUrn("media:text;textable", mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, "", resolved.Extension)
}

// Test extension can coexist with metadata and validation
func TestExtensionWithMetadataAndValidation(t *testing.T) {
	minLen := 1
	maxLen := 1000
	mediaSpecs := map[string]MediaSpecDef{
		"media:custom-output": {
			IsString: false,
			ObjectValue: &MediaSpecDefObject{
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
				Extension: "json",
			},
		},
	}

	resolved, err := ResolveMediaUrn("media:custom-output", mediaSpecs)
	require.NoError(t, err)

	// Verify all fields are present
	require.NotNil(t, resolved.Validation)
	require.NotNil(t, resolved.Metadata)
	assert.Equal(t, "json", resolved.Extension)
}
