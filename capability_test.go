package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityCreation(t *testing.T) {
	id, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	cap := NewCapability(id, "1.0.0")
	
	assert.Equal(t, "data_processing:transform:json", cap.IdString())
	assert.Equal(t, "1.0.0", cap.Version)
	assert.Nil(t, cap.Description)
	assert.NotNil(t, cap.Metadata)
	assert.Empty(t, cap.Metadata)
}

func TestCapabilityWithMetadata(t *testing.T) {
	id, err := NewCapabilityIdFromString("compute:math:arithmetic")
	require.NoError(t, err)
	
	metadata := map[string]string{
		"precision":  "double",
		"operations": "add,subtract,multiply,divide",
	}
	
	cap := NewCapabilityWithMetadata(id, "2.1.0", metadata)
	
	value, exists := cap.GetMetadata("precision")
	assert.True(t, exists)
	assert.Equal(t, "double", value)
	
	value, exists = cap.GetMetadata("operations")
	assert.True(t, exists)
	assert.Equal(t, "add,subtract,multiply,divide", value)
	
	assert.True(t, cap.HasMetadata("precision"))
	assert.False(t, cap.HasMetadata("nonexistent"))
}

func TestCapabilityRequestMatching(t *testing.T) {
	id, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	cap := NewCapability(id, "1.0.0")
	
	assert.True(t, cap.MatchesRequest("data_processing:transform:json"))
	assert.True(t, cap.MatchesRequest("data_processing:transform"))
	assert.True(t, cap.MatchesRequest("data_processing"))
	assert.False(t, cap.MatchesRequest("compute:math"))
}

func TestCapabilitySpecificity(t *testing.T) {
	id1, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	cap1 := NewCapability(id1, "1.0.0")
	
	id2, err := NewCapabilityIdFromString("data_processing:*")
	require.NoError(t, err)
	cap2 := NewCapability(id2, "1.0.0")
	
	assert.True(t, cap1.IsMoreSpecificThan(cap2))
	assert.False(t, cap2.IsMoreSpecificThan(cap1))
}

func TestCapabilityMetadataOperations(t *testing.T) {
	id, err := NewCapabilityIdFromString("test:capability")
	require.NoError(t, err)
	
	cap := NewCapability(id, "1.0.0")
	
	// Test setting metadata
	cap.SetMetadata("key1", "value1")
	assert.True(t, cap.HasMetadata("key1"))
	
	value, exists := cap.GetMetadata("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", value)
	
	// Test removing metadata
	removed := cap.RemoveMetadata("key1")
	assert.True(t, removed)
	assert.False(t, cap.HasMetadata("key1"))
	
	// Test removing non-existent metadata
	removed = cap.RemoveMetadata("nonexistent")
	assert.False(t, removed)
}

func TestCapabilityEquality(t *testing.T) {
	id, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	cap1 := NewCapabilityWithDescription(id, "1.0.0", "Test capability")
	cap2 := NewCapabilityWithDescription(id, "1.0.0", "Test capability")
	cap3 := NewCapabilityWithDescription(id, "2.0.0", "Test capability")
	
	assert.True(t, cap1.Equals(cap2))
	assert.False(t, cap1.Equals(cap3))
}

func TestCapabilityJSONSerialization(t *testing.T) {
	id, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	metadata := map[string]string{
		"format": "json",
		"version": "2.0",
	}
	
	original := NewCapabilityWithDescriptionAndMetadata(id, "1.0.0", "Test capability", metadata)
	
	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	
	var decoded Capability
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}