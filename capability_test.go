package capdef

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityCreation(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	
	cap := NewCapability(id, "1.0.0", "test-command")
	
	assert.Equal(t, "action=transform;format=json;type=data_processing", cap.IdString())
	assert.Equal(t, "1.0.0", cap.Version)
	assert.Equal(t, "test-command", cap.Command)
	assert.Nil(t, cap.Description)
	assert.NotNil(t, cap.Metadata)
	assert.Empty(t, cap.Metadata)
}

func TestCapabilityWithMetadata(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=arithmetic;subtype=math;type=compute")
	require.NoError(t, err)
	
	metadata := map[string]string{
		"precision":  "double",
		"operations": "add,subtract,multiply,divide",
	}
	
	cap := NewCapabilityWithMetadata(id, "2.1.0", "calc-command", metadata)
	
	precision, exists := cap.GetMetadata("precision")
	assert.True(t, exists)
	assert.Equal(t, "double", precision)
	
	operations, exists := cap.GetMetadata("operations")
	assert.True(t, exists)
	assert.Equal(t, "add,subtract,multiply,divide", operations)
	assert.True(t, cap.HasMetadata("precision"))
	assert.False(t, cap.HasMetadata("nonexistent"))
}

func TestCapabilityMatching(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	
	cap := NewCapability(id, "1.0.0", "test-command")
	
	assert.True(t, cap.MatchesRequest("action=transform;format=json;type=data_processing"))
	assert.True(t, cap.MatchesRequest("action=transform;format=*;type=data_processing")) // Request wants any format, cap handles json specifically
	assert.True(t, cap.MatchesRequest("type=data_processing"))
	assert.False(t, cap.MatchesRequest("type=compute"))
}

func TestCapabilityRequestHandling(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=extract;target=metadata;type=document")
	require.NoError(t, err)
	
	cap1 := NewCapability(id, "1.0.0", "extract-cmd")
	cap2 := NewCapability(id, "1.0.0", "extract-cmd")
	
	assert.True(t, cap1.CanHandleRequest(cap2.Id))
	
	otherId, err := NewCapabilityKeyFromString("action=generate;type=image")
	require.NoError(t, err)
	cap3 := NewCapability(otherId, "1.0.0", "generate-cmd")
	
	assert.False(t, cap1.CanHandleRequest(cap3.Id))
}

func TestCapabilityEquality(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	
	cap1 := NewCapability(id, "1.0.0", "test-command")
	cap2 := NewCapability(id, "1.0.0", "test-command")
	
	assert.True(t, cap1.Equals(cap2))
}

func TestCapabilityDescription(t *testing.T) {
	id, err := NewCapabilityKeyFromString("action=parse;format=json;type=data")
	require.NoError(t, err)
	
	cap1 := NewCapabilityWithDescription(id, "1.0.0", "parse-cmd", "Parse JSON data")
	cap2 := NewCapabilityWithDescription(id, "2.0.0", "parse-cmd", "Parse JSON data v2")
	cap3 := NewCapabilityWithDescription(id, "1.0.0", "parse-cmd", "Parse JSON data")
	
	assert.False(t, cap1.Equals(cap2)) // Different versions
	assert.True(t, cap1.Equals(cap3))  // Same everything
}