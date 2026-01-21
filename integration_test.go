package capns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper for integration tests - use proper media URNs with tags
func intTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:type=void;v=1";out="media:type=object;v=1;textable;keyed"`
	}
	return `cap:in="media:type=void;v=1";out="media:type=object;v=1;textable;keyed";` + tags
}

// TestIntegrationVersionlessCapCreation verifies caps can be created without version fields
func TestIntegrationVersionlessCapCreation(t *testing.T) {
	// Test case 1: Create cap without version parameter
	urn, err := NewCapUrnFromString(intTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	cap := NewCap(urn, "Data Transformer", "transform-command")

	// Verify the cap has direction specs in canonical form
	assert.Contains(t, cap.UrnString(), `in="media:type=void;v=1"`)
	assert.Contains(t, cap.UrnString(), `out="media:type=object;v=1;textable;keyed"`)
	assert.Equal(t, "transform-command", cap.Command)

	// Test case 2: Create cap with description but no version
	cap2 := NewCapWithDescription(urn, "Data Transformer", "transform-command", "Transforms data")
	assert.NotNil(t, cap2.CapDescription)
	assert.Equal(t, "Transforms data", *cap2.CapDescription)

	// Test case 3: Verify caps can be compared without version
	assert.True(t, cap.Equals(cap))

	// Different caps should not be equal
	urn2, _ := NewCapUrnFromString(intTestUrn("op=generate;format=pdf"))
	cap3 := NewCap(urn2, "PDF Generator", "generate-command")
	assert.False(t, cap.Equals(cap3))
}

// TestIntegrationCaseInsensitiveUrns verifies URNs are case-insensitive
func TestIntegrationCaseInsensitiveUrns(t *testing.T) {
	// Test case 1: Different case inputs should produce same URN
	urn1, err := NewCapUrnFromString(intTestUrn("OP=Transform;FORMAT=JSON;Type=Data_Processing"))
	require.NoError(t, err)

	urn2, err := NewCapUrnFromString(intTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	// URNs should be equal
	assert.True(t, urn1.Equals(urn2))
	assert.Equal(t, urn1.ToString(), urn2.ToString())

	// Test case 2: Case-insensitive tag operations
	op, exists := urn1.GetTag("OP")
	assert.True(t, exists)
	assert.Equal(t, "transform", op) // Should be normalized to lowercase

	op2, exists := urn1.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "transform", op2)

	// Test case 3: HasTag - keys case-insensitive, values case-sensitive
	// Unquoted values were normalized to lowercase, so "transform" is stored
	assert.True(t, urn1.HasTag("OP", "transform"))
	assert.True(t, urn1.HasTag("op", "transform"))
	assert.True(t, urn1.HasTag("Op", "transform"))
	// Different case values should NOT match
	assert.False(t, urn1.HasTag("op", "TRANSFORM"))

	// Test case 4: Builder preserves value case
	urn3, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("OP", "Transform"). // value case preserved
		Tag("Format", "JSON").  // value case preserved
		Build()
	require.NoError(t, err)

	// Builder preserves case, so we need exact match
	assert.True(t, urn3.HasTag("op", "Transform"))
	assert.True(t, urn3.HasTag("format", "JSON"))
}

// TestIntegrationCallerAndResponseSystem verifies the caller and response system
func TestIntegrationCallerAndResponseSystem(t *testing.T) {
	// Setup test cap definition with media URNs - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=extract;out="media:type=object;v=1;textable;keyed";target=metadata`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Metadata Extractor", "extract-metadata")
	capDef.SetOutput(NewCapOutput(MediaObject, "Extracted metadata"))

	// Add required argument using new architecture
	cliFlag := "--input"
	pos := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Input file path",
	})

	// Mock host that returns JSON
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"title": "Test Document", "pages": 10}`,
		},
	}

	// Create caller
	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=extract;out="media:type=object;v=1;textable;keyed";target=metadata`, mockHost, capDef)

	// Test call with valid arguments
	ctx := context.Background()
	positionalArgs := []interface{}{"test.pdf"}
	namedArgs := []interface{}{}

	response, err := caller.Call(ctx, positionalArgs, namedArgs, nil)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response properties
	assert.True(t, response.IsJSON())
	assert.False(t, response.IsBinary())
	assert.False(t, response.IsEmpty())

	// Verify response can be parsed as JSON
	var metadata map[string]interface{}
	err = response.AsType(&metadata)
	require.NoError(t, err)

	assert.Equal(t, "Test Document", metadata["title"])
	assert.Equal(t, float64(10), metadata["pages"])

	// Verify response validation against cap
	err = response.ValidateAgainstCap(capDef)
	assert.NoError(t, err)
}

// TestIntegrationBinaryCapHandling verifies binary cap handling
func TestIntegrationBinaryCapHandling(t *testing.T) {
	// Setup binary cap - use raw type with binary tag
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=generate;out="media:type=raw;v=1;binary";target=thumbnail`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Thumbnail Generator", "generate-thumbnail")
	capDef.SetOutput(NewCapOutput(MediaBinary, "Generated thumbnail"))

	// Mock host that returns binary data
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=generate;out="media:type=raw;v=1;binary";target=thumbnail`, mockHost, capDef)

	// Test binary response
	ctx := context.Background()
	response, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response is binary
	assert.True(t, response.IsBinary())
	assert.False(t, response.IsJSON())
	assert.False(t, response.IsText())
	assert.Equal(t, pngHeader, response.AsBytes())

	// Binary to string should fail
	_, err = response.AsString()
	assert.Error(t, err)
}

// TestIntegrationTextCapHandling verifies text cap handling
func TestIntegrationTextCapHandling(t *testing.T) {
	// Setup text cap - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=format;out="media:type=string;v=1;textable;scalar";target=text`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Text Formatter", "format-text")
	capDef.SetOutput(NewCapOutput(MediaString, "Formatted text"))

	// Add required argument using new architecture
	cliFlag := "--input"
	pos := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Input text",
	})

	// Mock host that returns text
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: "Formatted output text",
		},
	}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=format;out="media:type=string;v=1;textable;scalar";target=text`, mockHost, capDef)

	// Test text response
	ctx := context.Background()
	response, err := caller.Call(ctx, []interface{}{"input text"}, []interface{}{}, nil)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response is text
	assert.True(t, response.IsText())
	assert.False(t, response.IsJSON())
	assert.False(t, response.IsBinary())

	text, err := response.AsString()
	require.NoError(t, err)
	assert.Equal(t, "Formatted output text", text)
}

// TestIntegrationCapWithMediaSpecs verifies caps with custom media specs
func TestIntegrationCapWithMediaSpecs(t *testing.T) {
	// Setup cap with custom media spec - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=query;out="media:type=result;v=1;textable;keyed";target=data`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Query", "query-data")

	// Add custom media spec with schema - needs keyed tag for JSON
	capDef.AddMediaSpec("media:type=result;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/result",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
				"count": map[string]interface{}{"type": "integer"},
			},
			"required": []interface{}{"items", "count"},
		},
	))

	capDef.SetOutput(NewCapOutput("media:type=result;v=1;textable;keyed", "Query result"))

	// Mock host
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"items": ["a", "b", "c"], "count": 3}`,
		},
	}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=query;out="media:type=result;v=1;textable;keyed";target=data`, mockHost, capDef)

	// Test call
	ctx := context.Background()
	response, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response
	assert.True(t, response.IsJSON())

	// Validate against cap
	err = response.ValidateAgainstCap(capDef)
	assert.NoError(t, err)
}

// TestIntegrationCapValidation verifies cap schema validation
func TestIntegrationCapValidation(t *testing.T) {
	coordinator := NewCapValidationCoordinator()

	// Create a cap with arguments - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=process;out="media:type=object;v=1;textable;keyed";target=data`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Processor", "process-data")

	// Add required string argument using new architecture
	cliFlag1 := "--input"
	pos1 := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag1}, {Position: &pos1}},
		ArgDescription: "Input path",
	})

	// Add optional integer argument using new architecture
	cliFlag2 := "--count"
	pos2 := 1
	capDef.AddArg(CapArg{
		MediaUrn:       MediaInteger,
		Required:       false,
		Sources:        []ArgSource{{CliFlag: &cliFlag2}, {Position: &pos2}},
		ArgDescription: "Count limit",
	})

	// Set output
	capDef.SetOutput(NewCapOutput(MediaObject, "Processing result"))

	// Register cap
	coordinator.RegisterCap(capDef)

	// Test valid inputs
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{"test.txt"})
	assert.NoError(t, err)

	// Test valid inputs with optional argument
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{"test.txt", float64(10)})
	assert.NoError(t, err)

	// Test missing required argument
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{})
	assert.Error(t, err)

	// Test wrong type
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{123}) // Should be string
	assert.Error(t, err)
}

// TestIntegrationMediaUrnResolution verifies media URN resolution
func TestIntegrationMediaUrnResolution(t *testing.T) {
	// Test built-in media URN resolution
	resolved, err := ResolveMediaUrn(MediaString, nil)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)
	assert.False(t, resolved.IsBinary())
	assert.False(t, resolved.IsJSON())
	assert.True(t, resolved.IsText())

	// Test object media URN
	resolved, err = ResolveMediaUrn(MediaObject, nil)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.True(t, resolved.IsJSON())

	// Test binary media URN
	resolved, err = ResolveMediaUrn(MediaBinary, nil)
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test custom media URN resolution - use proper tags
	customSpecs := map[string]MediaSpecDef{
		"media:type=custom;v=1;textable": NewMediaSpecDefString("text/html; profile=https://example.com/schema/html"),
	}

	resolved, err = ResolveMediaUrn("media:type=custom;v=1;textable", customSpecs)
	require.NoError(t, err)
	assert.Equal(t, "text/html", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/html", resolved.ProfileURI)

	// Test unknown media URN fails
	_, err = ResolveMediaUrn("media:type=unknown;v=1", nil)
	assert.Error(t, err)
}

// TestIntegrationMediaSpecDefJSON verifies MediaSpecDef JSON serialization
func TestIntegrationMediaSpecDefJSON(t *testing.T) {
	// Test string form
	strDef := NewMediaSpecDefString("text/plain; profile=https://capns.org/schema/str")
	assert.True(t, strDef.IsString)
	assert.Equal(t, "text/plain; profile=https://capns.org/schema/str", strDef.StringValue)

	// Test object form
	objDef := NewMediaSpecDefObject("application/json", "https://example.com/schema")
	assert.False(t, objDef.IsString)
	assert.Equal(t, "application/json", objDef.ObjectValue.MediaType)
	assert.Equal(t, "https://example.com/schema", objDef.ObjectValue.ProfileURI)

	// Test object form with schema
	schema := map[string]interface{}{"type": "object"}
	schemaDef := NewMediaSpecDefObjectWithSchema("application/json", "https://example.com/schema", schema)
	assert.NotNil(t, schemaDef.ObjectValue.Schema)
}
