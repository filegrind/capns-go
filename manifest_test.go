package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper for manifest tests - use proper media URNs with tags
func manifestTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:void";out="media:object;textable;keyed"`
	}
	return `cap:in="media:void";out="media:object;textable;keyed";` + tags
}

func TestCapManifestCreation(t *testing.T) {
	id, err := NewCapUrnFromString(manifestTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap := NewCap(id, "Metadata Extractor", "extract-metadata")

	manifest := NewCapManifest(
		"TestComponent",
		"0.1.0",
		"A test component for validation",
		[]Cap{*cap},
	)

	assert.Equal(t, "TestComponent", manifest.Name)
	assert.Equal(t, "0.1.0", manifest.Version)
	assert.Equal(t, "A test component for validation", manifest.Description)
	assert.Len(t, manifest.Caps, 1)
	assert.Nil(t, manifest.Author)
}

func TestCapManifestWithAuthor(t *testing.T) {
	id, err := NewCapUrnFromString(manifestTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap := NewCap(id, "Metadata Extractor", "extract-metadata")

	manifest := NewCapManifest(
		"TestComponent",
		"0.1.0",
		"A test component for validation",
		[]Cap{*cap},
	).WithAuthor("Test Author")

	require.NotNil(t, manifest.Author)
	assert.Equal(t, "Test Author", *manifest.Author)
}

func TestCapManifestJSONSerialization(t *testing.T) {
	id, err := NewCapUrnFromString(manifestTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap := NewCap(id, "Metadata Extractor", "extract-metadata")
	// Add an argument with stdin source using new architecture
	stdinUrn := "media:pdf;binary"
	cap.AddArg(CapArg{
		MediaUrn: MediaBinary,
		Required: true,
		Sources:  []ArgSource{{Stdin: &stdinUrn}},
	})

	manifest := NewCapManifest(
		"TestComponent",
		"0.1.0",
		"A test component for validation",
		[]Cap{*cap},
	).WithAuthor("Test Author")

	// Test serialization
	jsonData, err := json.Marshal(manifest)
	require.NoError(t, err)

	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"name":"TestComponent"`)
	assert.Contains(t, jsonStr, `"version":"0.1.0"`)
	assert.Contains(t, jsonStr, `"author":"Test Author"`)
	assert.Contains(t, jsonStr, `"stdin":"media:pdf;binary"`)

	// Test deserialization
	var deserialized CapManifest
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)

	assert.Equal(t, manifest.Name, deserialized.Name)
	assert.Equal(t, manifest.Version, deserialized.Version)
	assert.Equal(t, manifest.Description, deserialized.Description)
	assert.Equal(t, manifest.Author, deserialized.Author)
	assert.Len(t, deserialized.Caps, len(manifest.Caps))
	assert.Equal(t, *manifest.Caps[0].GetStdinMediaUrn(), *deserialized.Caps[0].GetStdinMediaUrn())
}

func TestCapManifestRequiredFields(t *testing.T) {
	// Test that deserialization succeeds even with missing optional fields
	// (Go JSON unmarshaling uses zero values for missing fields)
	partialJSON := `{"name": "TestComponent", "version": "1.0.0", "description": "Test", "caps": []}`
	var result CapManifest
	err := json.Unmarshal([]byte(partialJSON), &result)
	assert.NoError(t, err)
	assert.Equal(t, "TestComponent", result.Name)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "Test", result.Description)
	assert.Len(t, result.Caps, 0)
	assert.Nil(t, result.Author)

	// Test that invalid JSON fails
	invalidJSON := `{"name": "TestComponent", invalid`
	var result2 CapManifest
	err2 := json.Unmarshal([]byte(invalidJSON), &result2)
	assert.Error(t, err2)
}

func TestCapManifestWithMultipleCaps(t *testing.T) {
	id1, err := NewCapUrnFromString(manifestTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)
	cap1 := NewCap(id1, "Metadata Extractor", "extract-metadata")

	id2, err := NewCapUrnFromString(manifestTestUrn("op=extract;target=outline"))
	require.NoError(t, err)
	metadata := map[string]string{"supports_outline": "true"}
	cap2 := NewCapWithMetadata(id2, "Outline Extractor", "extract-outline", metadata)

	manifest := NewCapManifest(
		"MultiCapComponent",
		"1.0.0",
		"Component with multiple caps",
		[]Cap{*cap1, *cap2},
	)

	assert.Len(t, manifest.Caps, 2)
	// Canonical form includes in/out
	assert.Contains(t, manifest.Caps[0].UrnString(), "op=extract")
	assert.Contains(t, manifest.Caps[0].UrnString(), "target=metadata")
	assert.Contains(t, manifest.Caps[1].UrnString(), "op=extract")
	assert.Contains(t, manifest.Caps[1].UrnString(), "target=outline")
	assert.True(t, manifest.Caps[1].HasMetadata("supports_outline"))
}

func TestCapManifestEmptyCaps(t *testing.T) {
	manifest := NewCapManifest(
		"EmptyComponent",
		"1.0.0",
		"Component with no caps",
		[]Cap{},
	)

	assert.Len(t, manifest.Caps, 0)

	// Should still serialize/deserialize correctly
	jsonData, err := json.Marshal(manifest)
	require.NoError(t, err)

	var deserialized CapManifest
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)
	assert.Len(t, deserialized.Caps, 0)
}

func TestCapManifestOptionalAuthorField(t *testing.T) {
	id, err := NewCapUrnFromString(manifestTestUrn("op=validate;file"))
	require.NoError(t, err)
	cap := NewCap(id, "File Validator", "validate")

	manifestWithoutAuthor := NewCapManifest(
		"ValidatorComponent",
		"1.0.0",
		"File validation component",
		[]Cap{*cap},
	)

	// Serialize manifest without author
	jsonData, err := json.Marshal(manifestWithoutAuthor)
	require.NoError(t, err)

	jsonStr := string(jsonData)
	assert.NotContains(t, jsonStr, `"author"`)

	// Should deserialize correctly
	var deserialized CapManifest
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)
	assert.Nil(t, deserialized.Author)
}

// Test component that implements ComponentMetadata interface
type testComponent struct {
	name string
	caps []Cap
}

// Implement the ComponentMetadata interface
func (tc *testComponent) ComponentManifest() *CapManifest {
	return NewCapManifest(
		tc.name,
		"1.0.0",
		"Test component implementation",
		tc.caps,
	)
}

func (tc *testComponent) Caps() []Cap {
	return tc.ComponentManifest().Caps
}

func TestComponentMetadataInterface(t *testing.T) {

	id, err := NewCapUrnFromString(manifestTestUrn("op=test;component"))
	require.NoError(t, err)
	cap := NewCap(id, "Test Component", "test")

	component := &testComponent{
		name: "TestImpl",
		caps: []Cap{*cap},
	}

	manifest := component.ComponentManifest()
	assert.Equal(t, "TestImpl", manifest.Name)

	caps := component.Caps()
	assert.Len(t, caps, 1)
	assert.Contains(t, caps[0].UrnString(), "op=test")
	assert.Contains(t, caps[0].UrnString(), "type=component")
}

func TestCapManifestValidation(t *testing.T) {
	// Test that manifest with valid caps works
	id, err := NewCapUrnFromString(manifestTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap := NewCap(id, "Metadata Extractor", "extract-metadata")
	// Add an argument with stdin source using new architecture
	stdinUrn := "media:pdf;binary"
	cap.AddArg(CapArg{
		MediaUrn: MediaBinary,
		Required: true,
		Sources:  []ArgSource{{Stdin: &stdinUrn}},
	})

	manifest := NewCapManifest(
		"ValidComponent",
		"1.0.0",
		"Valid component for testing",
		[]Cap{*cap},
	)

	// Validate that all required fields are present
	assert.NotEmpty(t, manifest.Name)
	assert.NotEmpty(t, manifest.Version)
	assert.NotEmpty(t, manifest.Description)
	assert.NotNil(t, manifest.Caps)

	// Validate cap integrity
	assert.Len(t, manifest.Caps, 1)
	capInManifest := manifest.Caps[0]
	// Version field removed from Cap struct
	assert.Equal(t, "extract-metadata", capInManifest.Command)
	assert.True(t, capInManifest.AcceptsStdin())
}

func TestCapManifestCompatibility(t *testing.T) {
	// Test that manifest format is compatible between different types
	id, err := NewCapUrnFromString(manifestTestUrn("op=process"))
	require.NoError(t, err)
	cap := NewCap(id, "Data Processor", "process")

	// Create manifest similar to what a plugin would have
	pluginStyleManifest := NewCapManifest(
		"PluginComponent",
		"0.1.0",
		"Plugin-style component",
		[]Cap{*cap},
	)

	// Create manifest similar to what a provider would have
	providerStyleManifest := NewCapManifest(
		"ProviderComponent",
		"0.1.0",
		"Provider-style component",
		[]Cap{*cap},
	)

	// Both should serialize to the same structure
	pluginJSON, err := json.Marshal(pluginStyleManifest)
	require.NoError(t, err)

	providerJSON, err := json.Marshal(providerStyleManifest)
	require.NoError(t, err)

	// Structure should be identical (except for name/description)
	var pluginMap map[string]interface{}
	var providerMap map[string]interface{}

	err = json.Unmarshal(pluginJSON, &pluginMap)
	require.NoError(t, err)

	err = json.Unmarshal(providerJSON, &providerMap)
	require.NoError(t, err)

	// Same structure
	assert.Equal(t, len(pluginMap), len(providerMap))
	assert.Contains(t, pluginMap, "name")
	assert.Contains(t, pluginMap, "version")
	assert.Contains(t, pluginMap, "description")
	assert.Contains(t, pluginMap, "caps")

	// Same field types
	assert.IsType(t, providerMap["name"], pluginMap["name"])
	assert.IsType(t, providerMap["version"], pluginMap["version"])
	assert.IsType(t, providerMap["description"], pluginMap["description"])
	assert.IsType(t, providerMap["caps"], pluginMap["caps"])
}
