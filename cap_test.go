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
		return `cap:in="media:void";out="media:form=map"`
	}
	return `cap:in="media:void";out="media:form=map";` + tags
}

// TEST108: Test creating new cap with URN, title, and command verifies correct initialization
func TestCapCreation(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=transform;format=json;data_processing"))
	require.NoError(t, err)

	cap := NewCap(id, "Transform JSON Data", "test-command")

	// Check that URN string contains the expected tags
	urnStr := cap.UrnString()
	assert.Contains(t, urnStr, "op=transform")
	assert.Contains(t, urnStr, "in=")
	assert.Contains(t, urnStr, "media:void")
	assert.Contains(t, urnStr, "out=")
	assert.Contains(t, urnStr, "form=map")
	assert.Equal(t, "Transform JSON Data", cap.Title)
	assert.NotNil(t, cap.Metadata)
	assert.Empty(t, cap.Metadata)
}

// TEST109: Test creating cap with metadata initializes and retrieves metadata correctly
func TestCapWithMetadata(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=arithmetic;compute;subtype=math"))
	require.NoError(t, err)

	metadata := map[string]string{
		"precision":  "double",
		"operations": "add,subtract,multiply,divide",
	}

	cap := NewCapWithMetadata(id, "Perform Mathematical Operations", "test-command", metadata)

	assert.Equal(t, "Perform Mathematical Operations", cap.Title)

	precision, exists := cap.GetMetadata("precision")
	assert.True(t, exists)
	assert.Equal(t, "double", precision)

	operations, exists := cap.GetMetadata("operations")
	assert.True(t, exists)
	assert.Equal(t, "add,subtract,multiply,divide", operations)

	assert.True(t, cap.HasMetadata("precision"))
	assert.False(t, cap.HasMetadata("nonexistent"))
}

// TEST110: Test cap matching with subset semantics for request fulfillment
func TestCapMatching(t *testing.T) {
	// Use type=data_processing key-value instead of flag for proper matching
	id, err := NewCapUrnFromString(capTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	cap := NewCap(id, "Transform JSON Data", "test-command")

	assert.True(t, cap.MatchesRequest(capTestUrn("op=transform;format=json;type=data_processing")))
	assert.True(t, cap.MatchesRequest(capTestUrn("op=transform;format=*;type=data_processing")))
	assert.True(t, cap.MatchesRequest(capTestUrn("type=data_processing")))
	assert.False(t, cap.MatchesRequest(capTestUrn("type=compute")))
}

// TEST111: Test getting and setting cap title updates correctly
func TestCapTitle(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap := NewCap(id, "Extract Document Metadata", "extract-metadata")

	assert.Equal(t, "Extract Document Metadata", cap.GetTitle())
	assert.Equal(t, "Extract Document Metadata", cap.Title)

	cap.SetTitle("Extract File Metadata")
	assert.Equal(t, "Extract File Metadata", cap.GetTitle())
	assert.Equal(t, "Extract File Metadata", cap.Title)
}

// TEST112: Test cap equality based on URN and title matching
func TestCapDefinitionEquality(t *testing.T) {
	id1, err := NewCapUrnFromString(capTestUrn("op=transform;format=json"))
	require.NoError(t, err)
	id2, err := NewCapUrnFromString(capTestUrn("op=transform;format=json"))
	require.NoError(t, err)

	cap1 := NewCap(id1, "Transform JSON Data", "transform")
	cap2 := NewCap(id2, "Transform JSON Data", "transform")
	cap3 := NewCap(id2, "Convert JSON Format", "transform")

	assert.True(t, cap1.Equals(cap2))
	assert.False(t, cap1.Equals(cap3))
	assert.False(t, cap2.Equals(cap3))
}

// TEST113: Test cap stdin support via args with stdin source and serialization roundtrip
func TestCapStdin(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=generate;target=embeddings"))
	require.NoError(t, err)

	cap := NewCap(id, "Generate Embeddings", "generate")

	// By default, caps should not accept stdin
	assert.False(t, cap.AcceptsStdin())
	assert.Nil(t, cap.GetStdinMediaUrn())

	// Enable stdin support by adding an arg with a stdin source
	stdinUrn := "media:textable"
	stdinArg := CapArg{
		MediaUrn:       "media:textable",
		Required:       true,
		Sources:        []ArgSource{{Stdin: &stdinUrn}},
		ArgDescription: "Input text",
	}
	cap.AddArg(stdinArg)

	assert.True(t, cap.AcceptsStdin())
	assert.Equal(t, "media:textable", *cap.GetStdinMediaUrn())

	// Test serialization/deserialization preserves the args
	serialized, err := json.Marshal(cap)
	require.NoError(t, err)
	assert.Contains(t, string(serialized), `"args"`)
	assert.Contains(t, string(serialized), `"stdin"`)

	var deserialized Cap
	err = json.Unmarshal(serialized, &deserialized)
	require.NoError(t, err)
	assert.True(t, deserialized.AcceptsStdin())
	assert.Equal(t, "media:textable", *deserialized.GetStdinMediaUrn())
}

// TEST114: Test ArgSource type variants stdin, position, and cli_flag with their accessors
func TestArgSourceTypes(t *testing.T) {
	// Test stdin source
	stdinUrn := "media:text"
	stdinSource := ArgSource{Stdin: &stdinUrn}
	assert.Equal(t, "stdin", stdinSource.GetType())
	assert.NotNil(t, stdinSource.StdinMediaUrn())
	assert.Equal(t, "media:text", *stdinSource.StdinMediaUrn())
	assert.Nil(t, stdinSource.GetPosition())
	assert.Nil(t, stdinSource.GetCliFlag())

	// Test position source
	pos := 0
	positionSource := ArgSource{Position: &pos}
	assert.Equal(t, "position", positionSource.GetType())
	assert.Nil(t, positionSource.StdinMediaUrn())
	assert.NotNil(t, positionSource.GetPosition())
	assert.Equal(t, 0, *positionSource.GetPosition())
	assert.Nil(t, positionSource.GetCliFlag())

	// Test cli_flag source
	flag := "--input"
	cliFlagSource := ArgSource{CliFlag: &flag}
	assert.Equal(t, "cli_flag", cliFlagSource.GetType())
	assert.Nil(t, cliFlagSource.StdinMediaUrn())
	assert.Nil(t, cliFlagSource.GetPosition())
	assert.NotNil(t, cliFlagSource.GetCliFlag())
	assert.Equal(t, "--input", *cliFlagSource.GetCliFlag())
}

// TEST115: Test CapArg serialization and deserialization with multiple sources
func TestCapArgSerialization(t *testing.T) {
	flag := "--name"
	pos := 0
	arg := CapArg{
		MediaUrn:       "media:string",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &flag}, {Position: &pos}},
		ArgDescription: "The name argument",
	}

	serialized, err := json.Marshal(arg)
	require.NoError(t, err)
	jsonStr := string(serialized)

	assert.Contains(t, jsonStr, `"media_urn":"media:string"`)
	assert.Contains(t, jsonStr, `"required":true`)
	assert.Contains(t, jsonStr, `"cli_flag":"--name"`)
	assert.Contains(t, jsonStr, `"position":0`)

	var deserialized CapArg
	err = json.Unmarshal(serialized, &deserialized)
	require.NoError(t, err)
	assert.Equal(t, arg, deserialized)
}

// TEST116: Test CapArg constructor methods basic and with_description create args correctly
func TestCapArgConstructors(t *testing.T) {
	// Test basic constructor
	flag := "--name"
	arg := NewCapArg("media:string", true, []ArgSource{{CliFlag: &flag}})
	assert.Equal(t, "media:string", arg.MediaUrn)
	assert.True(t, arg.Required)
	assert.Len(t, arg.Sources, 1)
	assert.Equal(t, "", arg.ArgDescription)

	// Test with description
	pos := 0
	arg2 := NewCapArgWithDescription(
		"media:integer",
		false,
		[]ArgSource{{Position: &pos}},
		"The count argument",
	)
	assert.Equal(t, "media:integer", arg2.MediaUrn)
	assert.False(t, arg2.Required)
	assert.Equal(t, "The count argument", arg2.ArgDescription)
}

// Additional existing tests below (not part of TEST108-116 sequence)

func TestCapRequestHandling(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=extract;target=metadata"))
	require.NoError(t, err)

	cap1 := NewCap(id, "Extract Metadata", "extract-cmd")
	cap2 := NewCap(id, "Extract Metadata", "extract-cmd")

	assert.True(t, cap1.AcceptsRequest(cap2.Urn))

	otherId, err := NewCapUrnFromString(capTestUrn("op=generate;image"))
	require.NoError(t, err)
	cap3 := NewCap(otherId, "Generate Image", "generate-cmd")

	assert.False(t, cap1.AcceptsRequest(cap3.Urn))
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
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"data": map[string]any{"type": "string"},
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

func TestCapJSONRoundTrip(t *testing.T) {
	id, err := NewCapUrnFromString(capTestUrn("op=test"))
	require.NoError(t, err)

	cap := NewCap(id, "Test Cap", "test-command")
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
