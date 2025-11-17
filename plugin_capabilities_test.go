package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginCapabilities(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0", "transform-cmd")
	
	id2, err := NewCapabilityKeyFromString("action=validate;format=*;type=data_processing")
	require.NoError(t, err)
	metadata := map[string]string{
		"formats": "json,xml,yaml",
	}
	cap2 := NewCapabilityWithMetadata(id2, "1.0.0", "validate-cmd", metadata)
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	assert.True(t, capabilities.CanHandleCapability("action=transform;format=json;type=data_processing"))
	assert.True(t, capabilities.CanHandleCapability("action=validate;format=xml;type=data_processing"))
	assert.False(t, capabilities.CanHandleCapability("type=compute"))
	
	metadataCaps := capabilities.CapabilitiesWithMetadata("formats", nil)
	assert.Len(t, metadataCaps, 1)
	
	versionCaps := capabilities.CapabilitiesWithVersion("1.0.0")
	assert.Len(t, versionCaps, 2)
}

func TestPluginCapabilitiesIdentifiers(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0", "transform-cmd")
	
	id2, err := NewCapabilityKeyFromString("action=arithmetic;subtype=math;type=compute")
	require.NoError(t, err)
	cap2 := NewCapability(id2, "1.0.0", "arithmetic-cmd")
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	identifiers := capabilities.CapabilityKeys()
	assert.Len(t, identifiers, 2)
	assert.Contains(t, identifiers, "action=transform;format=json;type=data_processing")
	assert.Contains(t, identifiers, "action=arithmetic;subtype=math;type=compute")
}

func TestFindBestCapability(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	// Add a specific capability
	id1, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0", "transform-cmd")
	
	// Add a wildcard capability
	id2, err := NewCapabilityKeyFromString("type=data_processing;action=*")
	require.NoError(t, err)
	cap2 := NewCapability(id2, "1.0.0", "general-cmd")
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	// Should find the more specific capability
	best := capabilities.FindBestCapabilityForRequest("action=transform;format=json;type=data_processing")
	require.NotNil(t, best)
	assert.Equal(t, "action=transform;format=json;type=data_processing", best.IdString())
	
	// Should find the wildcard capability for a different format
	best = capabilities.FindBestCapabilityForRequest("action=transform;format=xml;type=data_processing")
	require.NotNil(t, best)
	assert.Equal(t, "action=*;type=data_processing", best.IdString())
}

func TestCapabilitiesWithMetadata(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	metadata1 := map[string]string{
		"format": "json",
		"version": "2.0",
	}
	cap1 := NewCapabilityWithMetadata(id1, "1.0.0", "transform-cmd", metadata1)
	
	id2, err := NewCapabilityKeyFromString("action=transform;format=xml;type=data_processing")
	require.NoError(t, err)
	metadata2 := map[string]string{
		"format": "xml",
		"version": "1.0",
	}
	cap2 := NewCapabilityWithMetadata(id2, "1.0.0", "transform-cmd", metadata2)
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	// Find capabilities with specific format
	jsonValue := "json"
	jsonCaps := capabilities.CapabilitiesWithMetadata("format", &jsonValue)
	assert.Len(t, jsonCaps, 1)
	assert.Equal(t, "action=transform;format=json;type=data_processing", jsonCaps[0].IdString())
	
	// Find all capabilities with format metadata
	formatCaps := capabilities.CapabilitiesWithMetadata("format", nil)
	assert.Len(t, formatCaps, 2)
}

func TestAllMetadataKeys(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	metadata1 := map[string]string{
		"format": "json",
		"version": "2.0",
	}
	cap1 := NewCapabilityWithMetadata(id1, "1.0.0", "transform-cmd", metadata1)
	
	id2, err := NewCapabilityKeyFromString("action=transform;format=xml;type=data_processing")
	require.NoError(t, err)
	metadata2 := map[string]string{
		"format": "xml",
		"encoding": "utf-8",
	}
	cap2 := NewCapabilityWithMetadata(id2, "1.0.0", "transform-cmd", metadata2)
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	keys := capabilities.AllMetadataKeys()
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "format")
	assert.Contains(t, keys, "version")
	assert.Contains(t, keys, "encoding")
}

func TestRemoveCapability(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	cap := NewCapability(id, "1.0.0", "transform-cmd")
	
	capabilities.AddCapability(cap)
	assert.Equal(t, 1, capabilities.Count())
	assert.False(t, capabilities.IsEmpty())
	
	removed := capabilities.RemoveCapability(cap)
	assert.True(t, removed)
	assert.Equal(t, 0, capabilities.Count())
	assert.True(t, capabilities.IsEmpty())
	
	// Try to remove again
	removed = capabilities.RemoveCapability(cap)
	assert.False(t, removed)
}

func TestPluginCapabilitiesJSONSerialization(t *testing.T) {
	original := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0", "transform-cmd")
	
	id2, err := NewCapabilityKeyFromString("action=arithmetic;subtype=math;type=compute")
	require.NoError(t, err)
	cap2 := NewCapabilityWithDescription(id2, "2.0.0", "math-cmd", "Math operations")
	
	original.AddCapability(cap1)
	original.AddCapability(cap2)
	
	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	
	var decoded PluginCapabilities
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}