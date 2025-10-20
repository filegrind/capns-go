package capdef

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityIdCreation(t *testing.T) {
	capId, err := NewCapabilityIdFromString("data_processing:transform:json")
	
	assert.NoError(t, err)
	assert.NotNil(t, capId)
	assert.Equal(t, "data_processing:transform:json", capId.ToString())
	assert.Len(t, capId.Segments(), 3)
	assert.Equal(t, "data_processing", capId.Segments()[0])
	assert.Equal(t, "transform", capId.Segments()[1])
	assert.Equal(t, "json", capId.Segments()[2])
}

func TestInvalidCapabilityId(t *testing.T) {
	capId, err := NewCapabilityIdFromString("")
	
	assert.Nil(t, capId)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidFormat, err.(*CapabilityIdError).Code)
}

func TestInvalidCharacters(t *testing.T) {
	capId, err := NewCapabilityIdFromString("data@processing:transform")
	
	assert.Nil(t, capId)
	assert.Error(t, err)
	assert.Equal(t, ErrorInvalidCharacter, err.(*CapabilityIdError).Code)
}

func TestCapabilityMatching(t *testing.T) {
	capability, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	request1, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	request2, err := NewCapabilityIdFromString("data_processing:transform")
	require.NoError(t, err)
	
	request3, err := NewCapabilityIdFromString("data_processing")
	require.NoError(t, err)
	
	request4, err := NewCapabilityIdFromString("compute:math")
	require.NoError(t, err)
	
	assert.True(t, capability.CanHandle(request1))
	assert.True(t, capability.CanHandle(request2))
	assert.True(t, capability.CanHandle(request3))
	assert.False(t, capability.CanHandle(request4))
}

func TestWildcardMatching(t *testing.T) {
	wildcard, err := NewCapabilityIdFromString("data_processing:*")
	require.NoError(t, err)
	
	request1, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	request2, err := NewCapabilityIdFromString("data_processing:validate:xml")
	require.NoError(t, err)
	
	request3, err := NewCapabilityIdFromString("compute:math")
	require.NoError(t, err)
	
	assert.True(t, wildcard.CanHandle(request1))
	assert.True(t, wildcard.CanHandle(request2))
	assert.False(t, wildcard.CanHandle(request3))
}

func TestSpecificity(t *testing.T) {
	specific, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	general, err := NewCapabilityIdFromString("data_processing:*")
	require.NoError(t, err)
	
	assert.True(t, specific.IsMoreSpecificThan(general))
	assert.False(t, general.IsMoreSpecificThan(specific))
	assert.Equal(t, 3, specific.SpecificityLevel())
	assert.Equal(t, 1, general.SpecificityLevel())
}

func TestCompatibility(t *testing.T) {
	cap1, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	cap2, err := NewCapabilityIdFromString("data_processing:*")
	require.NoError(t, err)
	
	cap3, err := NewCapabilityIdFromString("compute:math")
	require.NoError(t, err)
	
	assert.True(t, cap1.IsCompatibleWith(cap2))
	assert.True(t, cap2.IsCompatibleWith(cap1))
	assert.False(t, cap1.IsCompatibleWith(cap3))
}

func TestEquality(t *testing.T) {
	cap1, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	cap2, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	cap3, err := NewCapabilityIdFromString("data_processing:transform:xml")
	require.NoError(t, err)
	
	assert.True(t, cap1.Equals(cap2))
	assert.False(t, cap1.Equals(cap3))
}

func TestWildcardAtLevel(t *testing.T) {
	cap, err := NewCapabilityIdFromString("data_processing:*:json")
	require.NoError(t, err)
	
	assert.False(t, cap.IsWildcardAtLevel(0))
	assert.True(t, cap.IsWildcardAtLevel(1))
	assert.False(t, cap.IsWildcardAtLevel(2))
	assert.False(t, cap.IsWildcardAtLevel(3))
}

func TestJSONSerialization(t *testing.T) {
	original, err := NewCapabilityIdFromString("data_processing:transform:json")
	require.NoError(t, err)
	
	data, err := json.Marshal(original)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	
	var decoded CapabilityId
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.True(t, original.Equals(&decoded))
}