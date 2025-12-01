package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapCreation(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	
	cap := NewCap(id, "test-command")
	
	assert.Equal(t, "cap:action=transform;format=json;type=data_processing", cap.UrnString())
	assert.Equal(t, "test-command", cap.Command)
	assert.Nil(t, cap.CapDescription)
	assert.NotNil(t, cap.Metadata)
	assert.Empty(t, cap.Metadata)
}

func TestCapWithMetadata(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=arithmetic;subtype=math;type=compute")
	require.NoError(t, err)
	
	metadata := map[string]string{
		"precision":  "double",
		"operations": "add,subtract,multiply,divide",
	}
	
	cap := NewCapWithMetadata(id, "calc-command", metadata)
	
	precision, exists := cap.GetMetadata("precision")
	assert.True(t, exists)
	assert.Equal(t, "double", precision)
	
	operations, exists := cap.GetMetadata("operations")
	assert.True(t, exists)
	assert.Equal(t, "add,subtract,multiply,divide", operations)
	assert.True(t, cap.HasMetadata("precision"))
	assert.False(t, cap.HasMetadata("nonexistent"))
}

func TestCapMatching(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	
	cap := NewCap(id, "test-command")
	
	assert.True(t, cap.MatchesRequest("cap:action=transform;format=json;type=data_processing"))
	assert.True(t, cap.MatchesRequest("cap:action=transform;format=*;type=data_processing")) // Request wants any format, cap handles json specifically
	assert.True(t, cap.MatchesRequest("cap:type=data_processing"))
	assert.False(t, cap.MatchesRequest("cap:type=compute"))
}

func TestCapRequestHandling(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=extract;target=metadata;")
	require.NoError(t, err)
	
	cap1 := NewCap(id, "extract-cmd")
	cap2 := NewCap(id, "extract-cmd")
	
	assert.True(t, cap1.CanHandleRequest(cap2.Urn))
	
	otherId, err := NewCapUrnFromString("cap:action=generate;type=image")
	require.NoError(t, err)
	cap3 := NewCap(otherId, "generate-cmd")
	
	assert.False(t, cap1.CanHandleRequest(cap3.Urn))
}

func TestCapEquality(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=transform;format=json;type=data_processing")
	require.NoError(t, err)
	
	cap1 := NewCap(id, "test-command")
	cap2 := NewCap(id, "test-command")
	
	assert.True(t, cap1.Equals(cap2))
}

func TestCapDescription(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=parse;format=json;type=data")
	require.NoError(t, err)
	
	cap1 := NewCapWithDescription(id, "parse-cmd", "Parse JSON data")
	cap2 := NewCapWithDescription(id, "parse-cmd", "Parse JSON data v2")
	cap3 := NewCapWithDescription(id, "parse-cmd", "Parse JSON data")
	
	assert.False(t, cap1.Equals(cap2)) // Different descriptions
	assert.True(t, cap1.Equals(cap3))  // Same everything
}

func TestCapAcceptsStdin(t *testing.T) {
	id, err := NewCapUrnFromString("cap:action=generate;target=embeddings")
	require.NoError(t, err)
	
	cap := NewCap(id, "generate")
	
	// By default, caps should not accept stdin
	assert.False(t, cap.AcceptsStdin)
	
	// Enable stdin support
	cap.AcceptsStdin = true
	assert.True(t, cap.AcceptsStdin)
	
	// Test JSON serialization/deserialization preserves the field
	jsonData, err := json.Marshal(cap)
	require.NoError(t, err)
	
	var deserialized Cap
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)
	
	assert.Equal(t, cap.AcceptsStdin, deserialized.AcceptsStdin)
}