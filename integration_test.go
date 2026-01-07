package capns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationVersionlessCapCreation verifies caps can be created without version fields
func TestIntegrationVersionlessCapCreation(t *testing.T) {
	// Test case 1: Create cap without version parameter
	urn, err := NewCapUrnFromString("cap:op=transform;format=json;type=data_processing")
	require.NoError(t, err)

	cap := NewCap(urn, "Data Transformer", "transform-command")

	// Verify the cap doesn't have version field
	assert.Equal(t, "cap:format=json;op=transform;type=data_processing", cap.UrnString())
	assert.Equal(t, "transform-command", cap.Command)

	// Test case 2: Create cap with description but no version
	cap2 := NewCapWithDescription(urn, "Data Transformer", "transform-command", "Transforms data")
	assert.NotNil(t, cap2.CapDescription)
	assert.Equal(t, "Transforms data", *cap2.CapDescription)

	// Test case 3: Verify caps can be compared without version
	assert.True(t, cap.Equals(cap))

	// Different caps should not be equal
	urn2, _ := NewCapUrnFromString("cap:op=generate;format=pdf")
	cap3 := NewCap(urn2, "PDF Generator", "generate-command")
	assert.False(t, cap.Equals(cap3))
}

// TestIntegrationCaseInsensitiveUrns verifies URNs are case-insensitive
func TestIntegrationCaseInsensitiveUrns(t *testing.T) {
	// Test case 1: Different case inputs should produce same URN
	urn1, err := NewCapUrnFromString("cap:OP=Transform;FORMAT=JSON;Type=Data_Processing")
	require.NoError(t, err)

	urn2, err := NewCapUrnFromString("cap:op=transform;format=json;type=data_processing")
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
	// Setup test cap definition with spec IDs
	urn, err := NewCapUrnFromString("cap:op=extract;target=metadata;out=std:obj.v1")
	require.NoError(t, err)

	capDef := NewCap(urn, "Metadata Extractor", "extract-metadata")
	capDef.SetOutput(NewCapOutput(SpecIDObj, "Extracted metadata"))

	// Add required argument
	arg := NewCapArgument("input", SpecIDStr, "Input file path", "--input")
	capDef.AddRequiredArgument(arg)

	// Mock host that returns JSON
	mockHost := &MockCapHost{
		returnResult: &HostResult{
			TextOutput: `{"title": "Test Document", "pages": 10}`,
		},
	}

	// Create caller
	caller := NewCapCaller("cap:op=extract;target=metadata;out=std:obj.v1", mockHost, capDef)

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
	// Setup binary cap
	urn, err := NewCapUrnFromString("cap:op=generate;target=thumbnail;out=std:binary.v1")
	require.NoError(t, err)

	capDef := NewCap(urn, "Thumbnail Generator", "generate-thumbnail")
	capDef.SetOutput(NewCapOutput(SpecIDBinary, "Generated thumbnail"))

	// Mock host that returns binary data
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapHost{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller("cap:op=generate;target=thumbnail;out=std:binary.v1", mockHost, capDef)

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
	// Setup text cap
	urn, err := NewCapUrnFromString("cap:op=format;target=text;out=std:str.v1")
	require.NoError(t, err)

	capDef := NewCap(urn, "Text Formatter", "format-text")
	capDef.SetOutput(NewCapOutput(SpecIDStr, "Formatted text"))

	// Add required argument
	arg := NewCapArgument("input", SpecIDStr, "Input text", "--input")
	capDef.AddRequiredArgument(arg)

	// Mock host that returns text
	mockHost := &MockCapHost{
		returnResult: &HostResult{
			TextOutput: "Formatted output text",
		},
	}

	caller := NewCapCaller("cap:op=format;target=text;out=std:str.v1", mockHost, capDef)

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
	// Setup cap with custom media spec
	urn, err := NewCapUrnFromString("cap:op=query;target=data;out=my:result.v1")
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Query", "query-data")

	// Add custom media spec with schema
	capDef.AddMediaSpec("my:result.v1", NewMediaSpecDefObjectWithSchema(
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

	capDef.SetOutput(NewCapOutput("my:result.v1", "Query result"))

	// Mock host
	mockHost := &MockCapHost{
		returnResult: &HostResult{
			TextOutput: `{"items": ["a", "b", "c"], "count": 3}`,
		},
	}

	caller := NewCapCaller("cap:op=query;target=data;out=my:result.v1", mockHost, capDef)

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

	// Create a cap with arguments
	urn, err := NewCapUrnFromString("cap:op=process;target=data;out=std:obj.v1")
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Processor", "process-data")

	// Add required string argument
	capDef.AddRequiredArgument(NewCapArgument("input", SpecIDStr, "Input path", "--input"))

	// Add optional integer argument
	optArg := NewCapArgument("count", SpecIDInt, "Count limit", "--count")
	capDef.AddOptionalArgument(optArg)

	// Set output
	capDef.SetOutput(NewCapOutput(SpecIDObj, "Processing result"))

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

// TestIntegrationSpecIDResolution verifies spec ID resolution
func TestIntegrationSpecIDResolution(t *testing.T) {
	// Test built-in spec ID resolution
	resolved, err := ResolveSpecID(SpecIDStr, nil)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)
	assert.False(t, resolved.IsBinary())
	assert.False(t, resolved.IsJSON())
	assert.True(t, resolved.IsText())

	// Test object spec
	resolved, err = ResolveSpecID(SpecIDObj, nil)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.True(t, resolved.IsJSON())

	// Test binary spec
	resolved, err = ResolveSpecID(SpecIDBinary, nil)
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test custom spec resolution
	customSpecs := map[string]MediaSpecDef{
		"my:custom.v1": NewMediaSpecDefString("text/html; profile=https://example.com/schema/html"),
	}

	resolved, err = ResolveSpecID("my:custom.v1", customSpecs)
	require.NoError(t, err)
	assert.Equal(t, "text/html", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/html", resolved.ProfileURI)

	// Test unknown spec ID fails
	_, err = ResolveSpecID("unknown:spec.v1", nil)
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
