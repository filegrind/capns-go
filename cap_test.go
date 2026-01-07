package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapCreation(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=transform;format=json;type=data_processing")
	require.NoError(t, err)

	cap := NewCap(id, "Transform JSON Data", "test-command")

	assert.Equal(t, "cap:format=json;op=transform;type=data_processing", cap.UrnString())
	assert.Equal(t, "Transform JSON Data", cap.Title)
	assert.Equal(t, "test-command", cap.Command)
	assert.Nil(t, cap.CapDescription)
	assert.NotNil(t, cap.Metadata)
	assert.Empty(t, cap.Metadata)
}

func TestCapWithMetadata(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=arithmetic;subtype=math;type=compute")
	require.NoError(t, err)

	metadata := map[string]string{
		"precision":  "double",
		"operations": "add,subtract,multiply,divide",
	}

	cap := NewCapWithMetadata(id, "Math Calculator", "calc-command", metadata)

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
	id, err := NewCapUrnFromString("cap:op=transform;format=json;type=data_processing")
	require.NoError(t, err)

	cap := NewCap(id, "Transform JSON Data", "test-command")

	assert.True(t, cap.MatchesRequest("cap:op=transform;format=json;type=data_processing"))
	assert.True(t, cap.MatchesRequest("cap:op=transform;format=*;type=data_processing")) // Request wants any format, cap handles json specifically
	assert.True(t, cap.MatchesRequest("cap:type=data_processing"))
	assert.False(t, cap.MatchesRequest("cap:type=compute"))
}

func TestCapRequestHandling(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=extract;target=metadata;")
	require.NoError(t, err)

	cap1 := NewCap(id, "Extract Metadata", "extract-cmd")
	cap2 := NewCap(id, "Extract Metadata", "extract-cmd")

	assert.True(t, cap1.CanHandleRequest(cap2.Urn))

	otherId, err := NewCapUrnFromString("cap:op=generate;type=image")
	require.NoError(t, err)
	cap3 := NewCap(otherId, "Generate Image", "generate-cmd")

	assert.False(t, cap1.CanHandleRequest(cap3.Urn))
}

func TestCapEquality(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=transform;format=json;type=data_processing")
	require.NoError(t, err)

	cap1 := NewCap(id, "Transform JSON Data", "test-command")
	cap2 := NewCap(id, "Transform JSON Data", "test-command")

	assert.True(t, cap1.Equals(cap2))
}

func TestCapDescription(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=parse;format=json;type=data")
	require.NoError(t, err)

	cap1 := NewCapWithDescription(id, "Parse JSON Data", "parse-cmd", "Parse JSON data")
	cap2 := NewCapWithDescription(id, "Parse JSON Data", "parse-cmd", "Parse JSON data v2")
	cap3 := NewCapWithDescription(id, "Parse JSON Data", "parse-cmd", "Parse JSON data")

	assert.False(t, cap1.Equals(cap2)) // Different descriptions
	assert.True(t, cap1.Equals(cap3))  // Same everything
}

func TestCapAcceptsStdin(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=generate;target=embeddings")
	require.NoError(t, err)

	cap := NewCap(id, "Generate Embeddings", "generate")

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

func TestCapWithMediaSpecs(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=query;target=structured;in=std:str.v1;out=my:result.v1")
	require.NoError(t, err)

	cap := NewCap(id, "Query Structured Data", "query-cmd")

	// Add a custom media spec
	cap.AddMediaSpec("my:result.v1", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/result",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{"type": "string"},
			},
		},
	))

	// Add an argument using the built-in spec ID
	cap.AddRequiredArgument(NewCapArgument("query", SpecIDStr, "The query string", "--query"))

	// Add output
	cap.SetOutput(NewCapOutput("my:result.v1", "Query result"))

	// Resolve the argument spec
	arg := cap.Arguments.Required[0]
	resolved, err := arg.Resolve(cap.GetMediaSpecs())
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)

	// Resolve the output spec
	outResolved, err := cap.Output.Resolve(cap.GetMediaSpecs())
	require.NoError(t, err)
	assert.Equal(t, "application/json", outResolved.MediaType)
	assert.NotNil(t, outResolved.Schema)
}

func TestCapJSONRoundTrip(t *testing.T) {
	id, err := NewCapUrnFromString("cap:op=test;out=std:obj.v1")
	require.NoError(t, err)

	cap := NewCap(id, "Test Cap", "test-cmd")
	cap.AddRequiredArgument(NewCapArgument("input", SpecIDStr, "Input text", "--input"))
	cap.SetOutput(NewCapOutput(SpecIDObj, "Output object"))

	// Serialize to JSON
	jsonData, err := json.Marshal(cap)
	require.NoError(t, err)

	// Deserialize
	var deserialized Cap
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, cap.Title, deserialized.Title)
	assert.Equal(t, cap.Command, deserialized.Command)
	assert.Equal(t, len(cap.Arguments.Required), len(deserialized.Arguments.Required))
	assert.Equal(t, cap.Arguments.Required[0].MediaSpec, deserialized.Arguments.Required[0].MediaSpec)
	assert.Equal(t, cap.Output.MediaSpec, deserialized.Output.MediaSpec)
}
