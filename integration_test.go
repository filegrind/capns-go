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
		return `cap:in="media:void";out="media:form=map;textable"`
	}
	return `cap:in="media:void";out="media:form=map;textable";` + tags
}

// TestIntegrationVersionlessCapCreation verifies caps can be created without version fields
func TestIntegrationVersionlessCapCreation(t *testing.T) {
	// Test case 1: Create cap without version parameter
	// Use type=data_processing key=value instead of flag
	urn, err := NewCapUrnFromString(intTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	cap := NewCap(urn, "Data Transformer", "transform-command")

	// Verify the cap has direction specs in canonical form
	// Colons don't need quoting, so media:void doesn't need quotes
	// But media:form=map;textable has semicolons, so needs quotes
	assert.Contains(t, cap.UrnString(), `in=media:void`)
	assert.Contains(t, cap.UrnString(), `out="media:form=map;textable"`)
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
	// Both must use key=value (not flags) for proper comparison
	urn1, err := NewCapUrnFromString(intTestUrn("OP=Transform;FORMAT=JSON;Type=Data_Processing"))
	require.NoError(t, err)

	urn2, err := NewCapUrnFromString(intTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	// URNs should be equal (case-insensitive keys and unquoted values)
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
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=extract;out="media:form=map;textable";target=metadata`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Metadata Extractor", "extract-metadata")
	capDef.SetOutput(NewCapOutput(MediaObject, "Extracted metadata"))

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs(map[string]MediaSpecDef{
		MediaObject: NewMediaSpecDefString("application/json; profile=" + ProfileObj),
		MediaString: NewMediaSpecDefString("text/plain; profile=" + ProfileStr),
	})

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
	caller := NewCapCaller(`cap:in="media:void";op=extract;out="media:form=map;textable";target=metadata`, mockHost, capDef)

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
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:bytes";target=thumbnail`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Thumbnail Generator", "generate-thumbnail")
	capDef.SetOutput(NewCapOutput(MediaBinary, "Generated thumbnail"))

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs(map[string]MediaSpecDef{
		MediaBinary: NewMediaSpecDefString("application/octet-stream"),
	})

	// Mock host that returns binary data
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=generate;out="media:bytes";target=thumbnail`, mockHost, capDef)

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
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=format;out="media:textable;form=scalar";target=text`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Text Formatter", "format-text")
	capDef.SetOutput(NewCapOutput(MediaString, "Formatted text"))

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs(map[string]MediaSpecDef{
		MediaString: NewMediaSpecDefString("text/plain; profile=" + ProfileStr),
	})

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

	caller := NewCapCaller(`cap:in="media:void";op=format;out="media:textable;form=scalar";target=text`, mockHost, capDef)

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
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=query;out="media:result;textable;form=map";target=data`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Query", "query-data")

	// Add custom media spec with schema - needs map tag for JSON
	capDef.AddMediaSpec("media:result;textable;form=map", NewMediaSpecDefObjectWithSchema(
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

	capDef.SetOutput(NewCapOutput("media:result;textable;form=map", "Query result"))

	// Mock host
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"items": ["a", "b", "c"], "count": 3}`,
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=query;out="media:result;textable;form=map";target=data`, mockHost, capDef)

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
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=process;out="media:form=map;textable";target=data`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Processor", "process-data")

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs(map[string]MediaSpecDef{
		MediaObject: NewMediaSpecDefString("application/json; profile=" + ProfileObj),
		MediaString: NewMediaSpecDefString("text/plain; profile=" + ProfileStr),
	})

	// Add required string argument using new architecture
	cliFlag1 := "--input"
	pos1 := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag1}, {Position: &pos1}},
		ArgDescription: "Input path",
	})

	// Set output
	capDef.SetOutput(NewCapOutput(MediaObject, "Processing result"))

	// Register cap
	coordinator.RegisterCap(capDef)

	// Test valid inputs - string for MediaString
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{"test.txt"})
	assert.NoError(t, err)

	// Test missing required argument
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{})
	assert.Error(t, err)
}

// TestIntegrationMediaUrnResolution verifies media URN resolution
func TestIntegrationMediaUrnResolution(t *testing.T) {
	// mediaSpecs for resolution - no built-in resolution, must provide specs
	mediaSpecs := map[string]MediaSpecDef{
		MediaString: NewMediaSpecDefString("text/plain; profile=" + ProfileStr),
		MediaObject: NewMediaSpecDefString("application/json; profile=" + ProfileObj),
		MediaBinary: NewMediaSpecDefString("application/octet-stream"),
	}

	// Test string media URN resolution
	resolved, err := ResolveMediaUrn(MediaString, mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)
	assert.False(t, resolved.IsBinary())
	assert.False(t, resolved.IsJSON())
	assert.True(t, resolved.IsText())

	// Test object media URN - note: MediaObject is form=map, not explicit json tag
	resolved, err = ResolveMediaUrn(MediaObject, mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.True(t, resolved.IsMap())        // form=map
	assert.True(t, resolved.IsStructured()) // is_map || is_list
	assert.False(t, resolved.IsJSON())      // no explicit json tag

	// Test binary media URN
	resolved, err = ResolveMediaUrn(MediaBinary, mediaSpecs)
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test custom media URN resolution - use proper tags
	customSpecs := map[string]MediaSpecDef{
		"media:custom;textable": NewMediaSpecDefString("text/html; profile=https://example.com/schema/html"),
	}

	resolved, err = ResolveMediaUrn("media:custom;textable", customSpecs)
	require.NoError(t, err)
	assert.Equal(t, "text/html", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/html", resolved.ProfileURI)

	// Test unknown media URN fails
	_, err = ResolveMediaUrn("media:unknown", nil)
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
