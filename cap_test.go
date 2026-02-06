package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create URNs with required in/out specs
func capTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:void";out="media:object"`
	}
	return `cap:in="media:void";out="media:object";` + tags
}

func TestCapCreation(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=transform;format=json;data_processing"))
	require.NoError(t, err)

	cap := NewCap(id, "Transform JSON Data", "test-command")

	// Canonical form includes in/out in alphabetical order
	// Values without semicolons (like media:void, media:object) don't need quotes
	assert.Equal(t, `cap:data_processing;format=json;in=media:void;op=transform;out=media:object`, cap.UrnString())
	assert.Equal(t, "Transform JSON Data", cap.Title)
	assert.Equal(t, "test-command", cap.Command)
	assert.Nil(t, cap.CapDescription)
	assert.NotNil(t, cap.Metadata)
	assert.Empty(t, cap.Metadata)
}

func TestCapWithMetadata(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=arithmetic;subtype=math;compute"))
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
	// Use key=value pairs instead of flags for proper matching tests
	id, err := NewCapUrnFromString(capTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	cap := NewCap(id, "Transform JSON Data", "test-command")

	assert.True(t, cap.MatchesRequest(capTestUrn("op=transform;format=json;type=data_processing")))
	assert.True(t, cap.MatchesRequest(capTestUrn("op=transform;format=*;type=data_processing"))) // Request wants any format
	assert.True(t, cap.MatchesRequest(capTestUrn("type=data_processing")))
	assert.False(t, cap.MatchesRequest(capTestUrn("type=compute")))
}

func TestCapRequestHandling(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap1 := NewCap(id, "Extract Metadata", "extract-cmd")
	cap2 := NewCap(id, "Extract Metadata", "extract-cmd")

	assert.True(t, cap1.CanHandleRequest(cap2.Urn))

	otherId, err := NewCapUrnFromString(capTestUrn("op=generate;image"))
	require.NoError(t, err)
	cap3 := NewCap(otherId, "Generate Image", "generate-cmd")

	assert.False(t, cap1.CanHandleRequest(cap3.Urn))
}

func TestCapEquality(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=transform;format=json;data_processing"))
	require.NoError(t, err)

	cap1 := NewCap(id, "Transform JSON Data", "test-command")
	cap2 := NewCap(id, "Transform JSON Data", "test-command")

	assert.True(t, cap1.Equals(cap2))
}

func TestCapDescription(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=parse;format=json;data"))
	require.NoError(t, err)

	cap1 := NewCapWithDescription(id, "Parse JSON Data", "parse-cmd", "Parse JSON data")
	cap2 := NewCapWithDescription(id, "Parse JSON Data", "parse-cmd", "Parse JSON data v2")
	cap3 := NewCapWithDescription(id, "Parse JSON Data", "parse-cmd", "Parse JSON data")

	assert.False(t, cap1.Equals(cap2)) // Different descriptions
	assert.True(t, cap1.Equals(cap3))  // Same everything
}

func TestCapStdin(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=generate;target=embeddings"))
	require.NoError(t, err)

	cap := NewCap(id, "Generate Embeddings", "generate")

	// By default, caps should not accept stdin
	assert.False(t, cap.AcceptsStdin())
	assert.Nil(t, cap.GetStdinMediaUrn())

	// Enable stdin support by adding an arg with stdin source
	stdinUrn := "media:textable"
	cap.AddArg(CapArg{
		MediaUrn: MediaString,
		Required: true,
		Sources:  []ArgSource{{Stdin: &stdinUrn}},
	})
	assert.True(t, cap.AcceptsStdin())
	assert.Equal(t, stdinUrn, *cap.GetStdinMediaUrn())

	// Test JSON serialization/deserialization preserves the field
	jsonData, err := json.Marshal(cap)
	require.NoError(t, err)

	// Verify JSON contains args with stdin source
	assert.Contains(t, string(jsonData), `"stdin":"media:textable"`)

	var deserialized Cap
	err = json.Unmarshal(jsonData, &deserialized)
	require.NoError(t, err)

	assert.True(t, deserialized.AcceptsStdin())
	assert.Equal(t, *cap.GetStdinMediaUrn(), *deserialized.GetStdinMediaUrn())
}

func TestCapWithMediaSpecs(t *testing.T) {
	// Use proper in/out in the URN - custom media URN in out
	id, err := NewCapUrnFromString(`cap:in="media:string";op=query;out="media:result";target=structured`)
	require.NoError(t, err)

	cap := NewCap(id, "Query Structured Data", "query-cmd")

	// Add media spec for MediaString (required for resolution)
	cap.AddMediaSpec(NewMediaSpecDef(MediaString, "text/plain", ProfileStr))

	// Add a custom media spec for the result type
	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:result",
		"application/json",
		"https://example.com/schema/result",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{"type": "string"},
			},
		},
	))

	// Add an argument using the media URN with new architecture
	cliFlag := "--query"
	pos := 0
	cap.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "The query string",
	})

	// Add output
	cap.SetOutput(NewCapOutput("media:result", "Query result"))

	// Get test registry
	registry := testRegistry(t)

	// Resolve the argument spec
	args := cap.GetArgs()
	require.Len(t, args, 1)
	arg := args[0]
	resolved, err := arg.Resolve(cap.GetMediaSpecs(), registry)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)

	// Resolve the output spec
	outResolved, err := cap.Output.Resolve(cap.GetMediaSpecs(), registry)
	require.NoError(t, err)
	assert.Equal(t, "application/json", outResolved.MediaType)
	assert.NotNil(t, outResolved.Schema)
}

// Helper to create a test registry (matches Rust test_registry() helper)
func testRegistry(t *testing.T) *MediaUrnRegistry {
	t.Helper()
	registry, err := NewMediaUrnRegistry()
	require.NoError(t, err, "Failed to create test registry")
	return registry
}

func TestCapJSONRoundTrip(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=test"))
	require.NoError(t, err)

	cap := NewCap(id, "Test Cap", "test-cmd")
	cliFlag := "--input"
	pos := 0
	cap.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Input text",
	})
	cap.SetOutput(NewCapOutput(MediaObject, "Output object"))

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
	assert.Equal(t, len(cap.GetArgs()), len(deserialized.GetArgs()))
	assert.Equal(t, cap.GetArgs()[0].MediaUrn, deserialized.GetArgs()[0].MediaUrn)
	assert.Equal(t, cap.Output.MediaUrn, deserialized.Output.MediaUrn)
}
