package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginCapabilities(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0")
	
	id2, err := NewCapabilityKeyFromString("data_processing:validate:*")
	require.NoError(t, err)
	metadata := map[string]string{
		"formats": "json,xml,yaml",
	}
	cap2 := NewCapabilityWithMetadata(id2, "1.0.0", metadata)
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	assert.True(t, capabilities.CanHandleCapability("data_processing:transform:json"))
	assert.True(t, capabilities.CanHandleCapability("data_processing:validate:xml"))
	assert.False(t, capabilities.CanHandleCapability("compute:math"))
	
	metadataCaps := capabilities.CapabilitiesWithMetadata("formats", nil)
	assert.Len(t, metadataCaps, 1)
	
	versionCaps := capabilities.CapabilitiesWithVersion("1.0.0")
	assert.Len(t, versionCaps, 2)
}

func TestPluginCapabilitiesIdentifiers(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0")
	
	id2, err := NewCapabilityKeyFromString("compute:math:arithmetic")
	require.NoError(t, err)
	cap2 := NewCapability(id2, "1.0.0")
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	identifiers := capabilities.CapabilityKeyentifiers()
	assert.Len(t, identifiers, 2)
	assert.Contains(t, identifiers, "data_processing:transform:json")
	assert.Contains(t, identifiers, "compute:math:arithmetic")
}

func TestFindBestCapability(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	// Add a specific capability
	id1, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0")
	
	// Add a wildcard capability
	id2, err := NewCapabilityKeyFromString("data_processing:*")
	require.NoError(t, err)
	cap2 := NewCapability(id2, "1.0.0")
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	// Should find the more specific capability
	best := capabilities.FindBestCapabilityForRequest("data_processing:transform:json")
	require.NotNil(t, best)
	assert.Equal(t, "data_processing:transform:json", best.IdString())
	
	// Should find the wildcard capability for a different format
	best = capabilities.FindBestCapabilityForRequest("data_processing:transform:xml")
	require.NotNil(t, best)
	assert.Equal(t, "data_processing:*", best.IdString())
}

func TestCapabilitiesWithMetadata(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	metadata1 := map[string]string{
		"format": "json",
		"version": "2.0",
	}
	cap1 := NewCapabilityWithMetadata(id1, "1.0.0", metadata1)
	
	id2, err := NewCapabilityKeyFromString("data_processing:transform:xml")
	require.NoError(t, err)
	metadata2 := map[string]string{
		"format": "xml",
		"version": "1.0",
	}
	cap2 := NewCapabilityWithMetadata(id2, "1.0.0", metadata2)
	
	capabilities.AddCapability(cap1)
	capabilities.AddCapability(cap2)
	
	// Find capabilities with specific format
	jsonValue := "json"
	jsonCaps := capabilities.CapabilitiesWithMetadata("format", &jsonValue)
	assert.Len(t, jsonCaps, 1)
	assert.Equal(t, "data_processing:transform:json", jsonCaps[0].IdString())
	
	// Find all capabilities with format metadata
	formatCaps := capabilities.CapabilitiesWithMetadata("format", nil)
	assert.Len(t, formatCaps, 2)
}

func TestAllMetadataKeys(t *testing.T) {
	capabilities := NewPluginCapabilities()
	
	id1, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	metadata1 := map[string]string{
		"format": "json",
		"version": "2.0",
	}
	cap1 := NewCapabilityWithMetadata(id1, "1.0.0", metadata1)
	
	id2, err := NewCapabilityKeyFromString("data_processing:transform:xml")
	require.NoError(t, err)
	metadata2 := map[string]string{
		"format": "xml",
		"encoding": "utf-8",
	}
	cap2 := NewCapabilityWithMetadata(id2, "1.0.0", metadata2)
	
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
	
	id, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	cap := NewCapability(id, "1.0.0")
	
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
	
	id1, err := NewCapabilityKeyFromString("data_processing:transform:json")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0")
	
	id2, err := NewCapabilityKeyFromString("compute:math:arithmetic")
	require.NoError(t, err)
	cap2 := NewCapabilityWithDescription(id2, "2.0.0", "Math operations")
	
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