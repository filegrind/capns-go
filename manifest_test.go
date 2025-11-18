package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityManifestCreation(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=extract;target=metadata;type=document")
	require.NoError(t, err)
	
	capability := NewCapability(id, "1.0.0", "extract-metadata")
	
	manifest := NewCapabilityManifest(
		"TestComponent",
		"0.1.0", 
		"A test component for validation",
		[]Capability{*capability},
	)
	
	assert.Equal(t, "TestComponent", manifest.Name)
	assert.Equal(t, "0.1.0", manifest.Version)
	assert.Equal(t, "A test component for validation", manifest.Description)
	assert.Len(t, manifest.Capabilities, 1)
	assert.Nil(t, manifest.Author)
}

func TestCapabilityManifestWithAuthor(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=extract;target=metadata;type=document")
	require.NoError(t, err)
	
	capability := NewCapability(id, "1.0.0", "extract-metadata")
	
	manifest := NewCapabilityManifest(
		"TestComponent",
		"0.1.0",
		"A test component for validation", 
		[]Capability{*capability},
	).WithAuthor("Test Author")
	
	require.NotNil(t, manifest.Author)
	assert.Equal(t, "Test Author", *manifest.Author)
}

func TestCapabilityManifestJSONSerialization(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=extract;target=metadata;type=document")
	require.NoError(t, err)
	
	capability := NewCapability(id, "1.0.0", "extract-metadata")
	capability.AcceptsStdin = true
	
	manifest := NewCapabilityManifest(
		"TestComponent",
		"0.1.0",
		"A test component for validation",
		[]Capability{*capability},
	).WithAuthor("Test Author")
	
	// Test serialization
	jsonData, err := json.Marshal(manifest)
	require.NoError(t, err)
	
	jsonStr := string(jsonData)
	assert.Contains(t, jsonStr, `"name":"TestComponent"`)
	assert.Contains(t, jsonStr, `"version":"0.1.0"`)
	assert.Contains(t, jsonStr, `"author":"Test Author"`)
	assert.Contains(t, jsonStr, `"accepts_stdin":true`)
	
	// Test deserialization
	var deserialized CapabilityManifest
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)
	
	assert.Equal(t, manifest.Name, deserialized.Name)
	assert.Equal(t, manifest.Version, deserialized.Version)
	assert.Equal(t, manifest.Description, deserialized.Description)
	assert.Equal(t, manifest.Author, deserialized.Author)
	assert.Len(t, deserialized.Capabilities, len(manifest.Capabilities))
	assert.Equal(t, manifest.Capabilities[0].AcceptsStdin, deserialized.Capabilities[0].AcceptsStdin)
}

func TestCapabilityManifestRequiredFields(t *testing.T) {
	// Test that deserialization succeeds even with missing optional fields
	// (Go JSON unmarshaling uses zero values for missing fields)
	partialJSON := `{"name": "TestComponent", "version": "1.0.0", "description": "Test", "capabilities": []}`
	var result CapabilityManifest
	err := json.Unmarshal([]byte(partialJSON), &result)
	assert.NoError(t, err)
	assert.Equal(t, "TestComponent", result.Name)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "Test", result.Description)
	assert.Len(t, result.Capabilities, 0)
	assert.Nil(t, result.Author)
	
	// Test that invalid JSON fails
	invalidJSON := `{"name": "TestComponent", invalid`
	var result2 CapabilityManifest
	err2 := json.Unmarshal([]byte(invalidJSON), &result2)
	assert.Error(t, err2)
}

func TestCapabilityManifestWithMultipleCapabilities(t *testing.T) {
	id1, err := NewCapabilityKeyFromString("action=extract;target=metadata;type=document")
	require.NoError(t, err)
	capability1 := NewCapability(id1, "1.0.0", "extract-metadata")
	
	id2, err := NewCapabilityKeyFromString("action=extract;target=outline;type=document")
	require.NoError(t, err)
	metadata := map[string]string{"supports_toc": "true"}
	capability2 := NewCapabilityWithMetadata(id2, "1.0.0", "extract-outline", metadata)
	
	manifest := NewCapabilityManifest(
		"MultiCapComponent",
		"1.0.0",
		"Component with multiple capabilities",
		[]Capability{*capability1, *capability2},
	)
	
	assert.Len(t, manifest.Capabilities, 2)
	assert.Equal(t, "action=extract;target=metadata;type=document", manifest.Capabilities[0].IdString())
	assert.Equal(t, "action=extract;target=outline;type=document", manifest.Capabilities[1].IdString())
	assert.True(t, manifest.Capabilities[1].HasMetadata("supports_toc"))
}

func TestCapabilityManifestEmptyCapabilities(t *testing.T) {
	manifest := NewCapabilityManifest(
		"EmptyComponent",
		"1.0.0",
		"Component with no capabilities",
		[]Capability{},
	)
	
	assert.Len(t, manifest.Capabilities, 0)
	
	// Should still serialize/deserialize correctly
	jsonData, err := json.Marshal(manifest)
	require.NoError(t, err)
	
	var deserialized CapabilityManifest
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)
	assert.Len(t, deserialized.Capabilities, 0)
}

func TestCapabilityManifestOptionalAuthorField(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=validate;type=file")
	require.NoError(t, err)
	capability := NewCapability(id, "1.0.0", "validate")
	
	manifestWithoutAuthor := NewCapabilityManifest(
		"ValidatorComponent",
		"1.0.0",
		"File validation component",
		[]Capability{*capability},
	)
	
	// Serialize manifest without author
	jsonData, err := json.Marshal(manifestWithoutAuthor)
	require.NoError(t, err)
	
	jsonStr := string(jsonData)
	assert.NotContains(t, jsonStr, `"author"`)
	
	// Should deserialize correctly
	var deserialized CapabilityManifest
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)
	assert.Nil(t, deserialized.Author)
}

// Test component that implements ComponentMetadata interface
type testComponent struct {
	name         string
	capabilities []Capability
}

// Implement the ComponentMetadata interface
func (tc *testComponent) ComponentManifest() *CapabilityManifest {
	return NewCapabilityManifest(
		tc.name,
		"1.0.0",
		"Test component implementation",
		tc.capabilities,
	)
}

func (tc *testComponent) Capabilities() []Capability {
	return tc.ComponentManifest().Capabilities
}

func TestComponentMetadataInterface(t *testing.T) {
	
	id, err := NewCapabilityKeyFromString("action=test;type=component")
	require.NoError(t, err)
	capability := NewCapability(id, "1.0.0", "test")
	
	component := &testComponent{
		name:         "TestImpl",
		capabilities: []Capability{*capability},
	}
	
	manifest := component.ComponentManifest()
	assert.Equal(t, "TestImpl", manifest.Name)
	
	capabilities := component.Capabilities()
	assert.Len(t, capabilities, 1)
	assert.Equal(t, "action=test;type=component", capabilities[0].IdString())
}

func TestCapabilityManifestValidation(t *testing.T) {
	// Test that manifest with valid capabilities works
	id, err := NewCapabilityKeyFromString("action=extract;target=metadata;type=document") 
	require.NoError(t, err)
	
	capability := NewCapability(id, "1.0.0", "extract-metadata")
	capability.AcceptsStdin = true
	
	manifest := NewCapabilityManifest(
		"ValidComponent",
		"1.0.0",
		"Valid component for testing",
		[]Capability{*capability},
	)
	
	// Validate that all required fields are present
	assert.NotEmpty(t, manifest.Name)
	assert.NotEmpty(t, manifest.Version)
	assert.NotEmpty(t, manifest.Description)
	assert.NotNil(t, manifest.Capabilities)
	
	// Validate capability integrity
	assert.Len(t, manifest.Capabilities, 1)
	cap := manifest.Capabilities[0]
	assert.Equal(t, "1.0.0", cap.Version)
	assert.Equal(t, "extract-metadata", cap.Command)
	assert.True(t, cap.AcceptsStdin)
}

func TestCapabilityManifestCompatibility(t *testing.T) {
	// Test that manifest format is compatible between different types
	id, err := NewCapabilityKeyFromString("action=process;type=document")
	require.NoError(t, err)
	capability := NewCapability(id, "1.0.0", "process")
	
	// Create manifest similar to what a plugin would have
	pluginStyleManifest := NewCapabilityManifest(
		"PluginComponent", 
		"0.1.0",
		"Plugin-style component",
		[]Capability{*capability},
	)
	
	// Create manifest similar to what a provider would have
	providerStyleManifest := NewCapabilityManifest(
		"ProviderComponent",
		"0.1.0", 
		"Provider-style component",
		[]Capability{*capability},
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
	assert.Contains(t, pluginMap, "capabilities")
	
	// Same field types
	assert.IsType(t, providerMap["name"], pluginMap["name"])
	assert.IsType(t, providerMap["version"], pluginMap["version"])
	assert.IsType(t, providerMap["description"], pluginMap["description"])
	assert.IsType(t, providerMap["capabilities"], pluginMap["capabilities"])
}