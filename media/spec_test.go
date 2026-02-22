package media

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -------------------------------------------------------------------------
// Media URN resolution tests
// -------------------------------------------------------------------------

// Helper to create a test registry (matches Rust test_registry() helper)
func testRegistry(t *testing.T) *MediaUrnRegistry {
	t.Helper()
	registry, err := NewMediaUrnRegistry()
	require.NoError(t, err, "Failed to create test registry")
	return registry
}

// TEST088: Test resolving string media URN from registry returns correct media type and profile
func Test088_resolve_from_registry_str(t *testing.T) {
	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:textable;form=scalar", nil, registry)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, "https://capns.org/schema/string", resolved.ProfileURI)
}

// TEST089: Test resolving object media URN from registry returns JSON media type
func Test089_resolve_from_registry_obj(t *testing.T) {
	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:form=map;textable", nil, registry)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
}

// TEST090: Test resolving binary media URN from registry returns octet-stream and IsBinary true
func Test090_resolve_from_registry_binary(t *testing.T) {
	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:", nil, registry)
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", resolved.MediaType)
	assert.True(t, resolved.IsBinary())
}

// TEST091: Test resolving custom media URN from local media_specs takes precedence over registry
func Test091_resolve_custom_media_spec(t *testing.T) {
	registry := testRegistry(t)
	customSpecs := []MediaSpecDef{
		{
			Urn:         "media:custom-spec;json",
			MediaType:   "application/json",
			Title:       "Custom Spec",
			ProfileURI:  "https://example.com/schema",
			Schema:      nil,
			Description: "",
			Validation:  nil,
			Metadata:    nil,
			Extensions:  []string{},
		},
	}

	// Local media_specs takes precedence over registry
	resolved, err := ResolveMediaUrn("media:custom-spec;json", customSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "media:custom-spec;json", resolved.SpecID)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema", resolved.ProfileURI)
	assert.Nil(t, resolved.Schema)
}

// TEST092: Test resolving custom object form media spec with schema from local media_specs
func Test092_resolve_custom_with_schema(t *testing.T) {
	registry := testRegistry(t)
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	customSpecs := []MediaSpecDef{
		{
			Urn:         "media:output-spec;json;form=map",
			MediaType:   "application/json",
			Title:       "Output Spec",
			ProfileURI:  "https://example.com/schema/output",
			Schema:      schema,
			Description: "",
			Validation:  nil,
			Metadata:    nil,
			Extensions:  []string{},
		},
	}

	resolved, err := ResolveMediaUrn("media:output-spec;json;form=map", customSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "media:output-spec;json;form=map", resolved.SpecID)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/output", resolved.ProfileURI)
	assert.Equal(t, schema, resolved.Schema)
}

// TEST093: Test resolving unknown media URN fails with UnresolvableMediaUrn error
func Test093_resolve_unresolvable_fails_hard(t *testing.T) {
	registry := testRegistry(t)
	// URN not in local media_specs and not in registry - FAIL HARD
	_, err := ResolveMediaUrn("media:completely-unknown-urn-not-in-registry", nil, registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "media:completely-unknown-urn-not-in-registry")
	assert.Contains(t, err.Error(), "cannot resolve")
}

// TEST094: Test local media_specs definition overrides registry definition for same URN
func Test094_local_overrides_registry(t *testing.T) {
	registry := testRegistry(t)

	// Custom definition in media_specs takes precedence over registry
	customOverride := []MediaSpecDef{
		{
			Urn:         "media:textable;form=scalar",
			MediaType:   "application/json", // Override: normally text/plain
			Title:       "Custom String",
			ProfileURI:  "https://custom.example.com/str",
			Schema:      nil,
			Description: "",
			Validation:  nil,
			Metadata:    nil,
			Extensions:  []string{},
		},
	}

	resolved, err := ResolveMediaUrn("media:textable;form=scalar", customOverride, registry)
	require.NoError(t, err)
	// Custom definition used, not registry
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.Equal(t, "https://custom.example.com/str", resolved.ProfileURI)
}

// -------------------------------------------------------------------------
// MediaSpecDef serialization tests
// -------------------------------------------------------------------------

// TEST095: Test MediaSpecDef serializes with required fields and skips None fields
func Test095_media_spec_def_serialize(t *testing.T) {
	def := MediaSpecDef{
		Urn:         "media:test;json",
		MediaType:   "application/json",
		Title:       "Test Media",
		ProfileURI:  "https://example.com/profile",
		Schema:      nil,
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	jsonBytes, err := json.Marshal(def)
	require.NoError(t, err)
	jsonStr := string(jsonBytes)

	assert.Contains(t, jsonStr, `"urn":"media:test;json"`)
	assert.Contains(t, jsonStr, `"media_type":"application/json"`)
	assert.Contains(t, jsonStr, `"profile_uri":"https://example.com/profile"`)
	assert.Contains(t, jsonStr, `"title":"Test Media"`)
	// Empty/nil fields use omitempty - check they're omitted or empty
	// Schema is nil - omitempty skips it
	// Description is empty string - may or may not be omitted depending on tag
}

// TEST096: Test deserializing MediaSpecDef from JSON object
func Test096_media_spec_def_deserialize(t *testing.T) {
	jsonStr := `{"urn":"media:test;json","media_type":"application/json","title":"Test"}`
	var def MediaSpecDef
	err := json.Unmarshal([]byte(jsonStr), &def)
	require.NoError(t, err)
	assert.Equal(t, "media:test;json", def.Urn)
	assert.Equal(t, "application/json", def.MediaType)
	assert.Equal(t, "Test", def.Title)
	assert.Equal(t, "", def.ProfileURI)
}

// -------------------------------------------------------------------------
// Duplicate URN validation tests
// -------------------------------------------------------------------------

// TEST097: Test duplicate URN validation catches duplicates
func Test097_validate_no_duplicate_urns_catches_duplicates(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		NewMediaSpecDefWithTitle("media:dup;json", "application/json", "", "First"),
		NewMediaSpecDefWithTitle("media:dup;json", "application/json", "", "Second"), // duplicate
	}
	err := ValidateNoMediaSpecDuplicates(mediaSpecs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "media:dup;json")
	assert.Contains(t, err.Error(), "duplicate")
}

// TEST098: Test duplicate URN validation passes for unique URNs
func Test098_validate_no_duplicate_urns_passes_for_unique(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		NewMediaSpecDefWithTitle("media:first;json", "application/json", "", "First"),
		NewMediaSpecDefWithTitle("media:second;json", "application/json", "", "Second"),
	}
	err := ValidateNoMediaSpecDuplicates(mediaSpecs)
	require.NoError(t, err)
}

// -------------------------------------------------------------------------
// ResolvedMediaSpec tests
// -------------------------------------------------------------------------

// TEST099: Test ResolvedMediaSpec IsBinary returns true when textable tag is absent
func Test099_resolved_is_binary(t *testing.T) {
	resolved := &ResolvedMediaSpec{
		SpecID:      "media:",
		MediaType:   "application/octet-stream",
		ProfileURI:  "",
		Schema:      nil,
		Title:       "",
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	assert.True(t, resolved.IsBinary())
	assert.False(t, resolved.IsMap())
	assert.False(t, resolved.IsJSON())
}

// TEST100: Test ResolvedMediaSpec IsMap returns true for form=map media URN
func Test100_resolved_is_map(t *testing.T) {
	resolved := &ResolvedMediaSpec{
		SpecID:      "media:textable;form=map",
		MediaType:   "application/json",
		ProfileURI:  "",
		Schema:      nil,
		Title:       "",
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	assert.True(t, resolved.IsMap())
	assert.False(t, resolved.IsBinary())
	assert.False(t, resolved.IsScalar())
	assert.False(t, resolved.IsList())
}

// TEST101: Test ResolvedMediaSpec IsScalar returns true for form=scalar media URN
func Test101_resolved_is_scalar(t *testing.T) {
	resolved := &ResolvedMediaSpec{
		SpecID:      "media:textable;form=scalar",
		MediaType:   "text/plain",
		ProfileURI:  "",
		Schema:      nil,
		Title:       "",
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	assert.True(t, resolved.IsScalar())
	assert.False(t, resolved.IsMap())
	assert.False(t, resolved.IsList())
}

// TEST102: Test ResolvedMediaSpec IsList returns true for form=list media URN
func Test102_resolved_is_list(t *testing.T) {
	resolved := &ResolvedMediaSpec{
		SpecID:      "media:textable;form=list",
		MediaType:   "application/json",
		ProfileURI:  "",
		Schema:      nil,
		Title:       "",
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	assert.True(t, resolved.IsList())
	assert.False(t, resolved.IsMap())
	assert.False(t, resolved.IsScalar())
}

// TEST103: Test ResolvedMediaSpec IsJSON returns true when json tag is present
func Test103_resolved_is_json(t *testing.T) {
	resolved := &ResolvedMediaSpec{
		SpecID:      "media:json;textable;form=map",
		MediaType:   "application/json",
		ProfileURI:  "",
		Schema:      nil,
		Title:       "",
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	assert.True(t, resolved.IsJSON())
	assert.True(t, resolved.IsMap())
	assert.False(t, resolved.IsBinary())
}

// TEST104: Test ResolvedMediaSpec IsText returns true when textable tag is present
func Test104_resolved_is_text(t *testing.T) {
	resolved := &ResolvedMediaSpec{
		SpecID:      "media:textable",
		MediaType:   "text/plain",
		ProfileURI:  "",
		Schema:      nil,
		Title:       "",
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{},
	}
	assert.True(t, resolved.IsText())
	assert.False(t, resolved.IsBinary())
	assert.False(t, resolved.IsJSON())
}

// -------------------------------------------------------------------------
// Metadata propagation tests
// -------------------------------------------------------------------------

// TEST105: Test metadata propagates from media spec def to resolved media spec
func Test105_metadata_propagation(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:custom-setting;setting",
			MediaType:   "text/plain",
			Title:       "Custom Setting",
			ProfileURI:  "https://example.com/schema",
			Schema:      nil,
			Description: "A custom setting",
			Validation:  nil,
			Metadata: map[string]any{
				"category_key": "interface",
				"ui_type":      "SETTING_UI_TYPE_CHECKBOX",
			},
			Extensions: []string{},
		},
	}

	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:custom-setting;setting", mediaSpecs, registry)
	require.NoError(t, err)
	require.NotNil(t, resolved.Metadata)
	assert.Equal(t, "interface", resolved.Metadata["category_key"])
	assert.Equal(t, "SETTING_UI_TYPE_CHECKBOX", resolved.Metadata["ui_type"])
}

// TEST106: Test metadata and validation can coexist in media spec definition
func Test106_metadata_with_validation(t *testing.T) {
	minVal := 0.0
	maxVal := 100.0
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:bounded-number;numeric;setting",
			MediaType:   "text/plain",
			Title:       "Bounded Number",
			ProfileURI:  "https://example.com/schema",
			Schema:      nil,
			Description: "",
			Validation: &MediaValidation{
				Min: &minVal,
				Max: &maxVal,
			},
			Metadata: map[string]any{
				"category_key": "inference",
				"ui_type":      "SETTING_UI_TYPE_SLIDER",
			},
			Extensions: []string{},
		},
	}

	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:bounded-number;numeric;setting", mediaSpecs, registry)
	require.NoError(t, err)

	// Verify validation
	require.NotNil(t, resolved.Validation)
	assert.Equal(t, 0.0, *resolved.Validation.Min)
	assert.Equal(t, 100.0, *resolved.Validation.Max)

	// Verify metadata
	require.NotNil(t, resolved.Metadata)
	assert.Equal(t, "inference", resolved.Metadata["category_key"])
}

// -------------------------------------------------------------------------
// Extension field tests
// -------------------------------------------------------------------------

// TEST107: Test extensions field propagates from media spec def to resolved
func Test107_extensions_propagation(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:custom-pdf",
			MediaType:   "application/pdf",
			Title:       "PDF Document",
			ProfileURI:  "https://capns.org/schema/pdf",
			Schema:      nil,
			Description: "A PDF document",
			Validation:  nil,
			Metadata:    nil,
			Extensions:  []string{"pdf"},
		},
	}

	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:custom-pdf", mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, []string{"pdf"}, resolved.Extensions)
}

// TEST108: Test extensions serializes/deserializes correctly in MediaSpecDef
func Test108_extensions_serialization(t *testing.T) {
	def := MediaSpecDef{
		Urn:         "media:json-data",
		MediaType:   "application/json",
		Title:       "JSON Data",
		ProfileURI:  "https://example.com/profile",
		Schema:      nil,
		Description: "",
		Validation:  nil,
		Metadata:    nil,
		Extensions:  []string{"json"},
	}
	jsonBytes, err := json.Marshal(def)
	require.NoError(t, err)
	jsonStr := string(jsonBytes)
	assert.Contains(t, jsonStr, `"extensions":["json"]`)

	// Deserialize and verify
	var parsed MediaSpecDef
	err = json.Unmarshal(jsonBytes, &parsed)
	require.NoError(t, err)
	assert.Equal(t, []string{"json"}, parsed.Extensions)
}

// TEST109: Test extensions can coexist with metadata and validation
func Test109_extensions_with_metadata_and_validation(t *testing.T) {
	minLen := 1
	maxLen := 1000
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:custom-output;json",
			MediaType:   "application/json",
			Title:       "Custom Output",
			ProfileURI:  "https://example.com/schema",
			Schema:      nil,
			Description: "",
			Validation: &MediaValidation{
				MinLength: &minLen,
				MaxLength: &maxLen,
			},
			Metadata: map[string]any{
				"category": "output",
			},
			Extensions: []string{"json"},
		},
	}

	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:custom-output;json", mediaSpecs, registry)
	require.NoError(t, err)

	// Verify all fields are present
	require.NotNil(t, resolved.Validation)
	require.NotNil(t, resolved.Metadata)
	assert.Equal(t, []string{"json"}, resolved.Extensions)
}

// TEST110: Test multiple extensions in a media spec
func Test110_multiple_extensions(t *testing.T) {
	mediaSpecs := []MediaSpecDef{
		{
			Urn:         "media:image;jpeg",
			MediaType:   "image/jpeg",
			Title:       "JPEG Image",
			ProfileURI:  "https://capns.org/schema/jpeg",
			Schema:      nil,
			Description: "JPEG image data",
			Validation:  nil,
			Metadata:    nil,
			Extensions:  []string{"jpg", "jpeg"},
		},
	}

	registry := testRegistry(t)
	resolved, err := ResolveMediaUrn("media:image;jpeg", mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, []string{"jpg", "jpeg"}, resolved.Extensions)
	assert.Len(t, resolved.Extensions, 2)
}

// -------------------------------------------------------------------------
// Standard caps tests (from other test file - included for completeness)
// -------------------------------------------------------------------------

// TEST304: Test MediaAvailabilityOutput constant parses as valid media URN with correct tags
func Test304_media_availability_output_constant(t *testing.T) {
	assert.True(t, HasMediaUrnTag(MediaAvailabilityOutput, "textable"),
		"model-availability must be textable")
	assert.True(t, HasMediaUrnTagValue(MediaAvailabilityOutput, "form", "map"),
		"model-availability must be form=map")
	assert.True(t, HasMediaUrnTag(MediaAvailabilityOutput, "textable"),
		"model-availability must be textable (not binary)")
}

// TEST305: Test MediaPathOutput constant parses as valid media URN with correct tags
func Test305_media_path_output_constant(t *testing.T) {
	assert.True(t, HasMediaUrnTag(MediaPathOutput, "textable"),
		"model-path must be textable")
	assert.True(t, HasMediaUrnTagValue(MediaPathOutput, "form", "map"),
		"model-path must be form=map")
	assert.True(t, HasMediaUrnTag(MediaPathOutput, "textable"),
		"model-path must be textable (not binary)")
}

// TEST306: Test MediaAvailabilityOutput and MediaPathOutput are distinct URNs
func Test306_availability_and_path_output_distinct(t *testing.T) {
	assert.NotEqual(t, MediaAvailabilityOutput, MediaPathOutput,
		"availability and path output must be distinct media URNs")
	// They must NOT be the same type (different model-availability vs model-path marker tags)
	assert.True(t, HasMediaUrnTag(MediaAvailabilityOutput, "model-availability"),
		"availability must have model-availability tag")
	assert.True(t, HasMediaUrnTag(MediaPathOutput, "model-path"),
		"path must have model-path tag")
}
