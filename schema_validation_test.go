package capns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a cap with media specs for testing
func createCapWithSchema(t *testing.T, argSchema interface{}) *Cap {
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:form=map;textable"`)
	require.NoError(t, err)

	cap := NewCap(urn, "Test Cap", "test-command")

	// Add a custom media spec with the provided schema
	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:test-obj;textable;form=map",
		"application/json",
		"https://test.example.com/schema",
		argSchema,
	))

	return cap
}

// TEST051: Test input validation succeeds with valid positional argument
func TestSchemaValidator_ValidateArgumentWithSchema_Success(t *testing.T) {
	validator := NewSchemaValidator()

	// Define a JSON schema for user data
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
			},
		},
		"required": []interface{}{"name"},
	}

	// Create an argument using new architecture
	cliFlag := "--user"
	pos := 0
	arg := CapArg{
		MediaUrn:       "media:test-obj;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "User data",
	}

	// Test valid data
	validData := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
	}

	err := validator.ValidateArgumentWithSchema(&arg, schema, validData)
	assert.NoError(t, err)
}

// TEST052: Test input validation fails with MissingRequiredArgument when required arg missing
func TestSchemaValidator_ValidateArgumentWithSchema_Failure(t *testing.T) {
	validator := NewSchemaValidator()

	// Define a JSON schema requiring name field
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
			},
		},
		"required": []interface{}{"name"},
	}

	// Create an argument using new architecture
	cliFlag := "--user"
	pos := 0
	arg := CapArg{
		MediaUrn:       "media:test-obj;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "User data",
	}

	// Test invalid data (missing required field)
	invalidData := map[string]interface{}{
		"age": 30,
	}

	err := validator.ValidateArgumentWithSchema(&arg, schema, invalidData)
	assert.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "MediaValidation", schemaErr.Type)
	assert.Equal(t, "media:test-obj;textable;form=map", schemaErr.Argument)
	assert.Contains(t, schemaErr.Details, "name")
}

// TEST053: Test input validation fails with InvalidArgumentType when wrong type provided
func TestSchemaValidator_ValidateArgumentWithSchema_NilSchema(t *testing.T) {
	validator := NewSchemaValidator()

	// Create argument using new architecture
	cliFlag := "--string"
	pos := 0
	arg := CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Simple string",
	}

	// Nil schema should not validate
	err := validator.ValidateArgumentWithSchema(&arg, nil, "any string value")
	assert.NoError(t, err)
}

func TestSchemaValidator_ValidateOutputWithSchema_Success(t *testing.T) {
	validator := NewSchemaValidator()

	// Define a JSON schema for result data
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{
				"type": "string",
			},
			"timestamp": map[string]interface{}{
				"type":   "string",
				"format": "date-time",
			},
		},
		"required": []interface{}{"result"},
	}

	// Create output
	output := NewCapOutput("media:test-result;textable;form=map", "Query result")

	// Test valid output data
	validData := map[string]interface{}{
		"result":    "success",
		"timestamp": "2023-01-01T00:00:00Z",
	}

	err := validator.ValidateOutputWithSchema(output, schema, validData)
	assert.NoError(t, err)
}

func TestSchemaValidator_ValidateOutputWithSchema_Failure(t *testing.T) {
	validator := NewSchemaValidator()

	// Define a JSON schema requiring result field
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []interface{}{"result"},
	}

	// Create output
	output := NewCapOutput("media:test-result;textable;form=map", "Query result")

	// Test invalid output data (missing required field)
	invalidData := map[string]interface{}{
		"status": "ok",
	}

	err := validator.ValidateOutputWithSchema(output, schema, invalidData)
	assert.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "OutputValidation", schemaErr.Type)
	assert.Contains(t, schemaErr.Details, "result")
}

func TestSchemaValidator_ValidateArguments_Integration(t *testing.T) {
	registry := testRegistry(t)
	validator := NewSchemaValidator()

	// Create a capability with schema-enabled arguments
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=query;out="media:form=map;textable";target=structured`)
	require.NoError(t, err)

	cap := NewCap(urn, "Query Processor", "test-command")

	// Add a custom media spec with schema
	userSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer", "minimum": 0},
		},
		"required": []interface{}{"name"},
	}

	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:user;textable;form=map",
		"application/json",
		"https://example.com/schema/user",
		userSchema,
	))

	// Add argument referencing the custom spec using new architecture
	cliFlag := "--user"
	pos := 0
	cap.AddArg(CapArg{
		MediaUrn:       "media:user;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "User data",
	})

	// Test valid arguments
	validUser := map[string]interface{}{
		"name": "Alice",
		"age":  25,
	}

	namedArgs := map[string]interface{}{
		"media:user;textable;form=map": validUser,
	}

	err = validator.ValidateArguments(cap, []interface{}{}, namedArgs, registry)
	assert.NoError(t, err)

	// Test invalid arguments
	invalidUser := map[string]interface{}{
		"age": 25, // Missing required "name"
	}

	namedArgs = map[string]interface{}{
		"media:user;textable;form=map": invalidUser,
	}

	err = validator.ValidateArguments(cap, []interface{}{}, namedArgs, registry)
	assert.Error(t, err)
}

func TestSchemaValidator_ArraySchemaValidation(t *testing.T) {
	validator := NewSchemaValidator()

	// Define a JSON schema for an array of items
	schema := map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":   map[string]interface{}{"type": "integer"},
				"name": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"id", "name"},
		},
		"minItems": 1,
	}

	// Create an argument using new architecture
	cliFlag := "--items"
	pos := 0
	arg := CapArg{
		MediaUrn:       "media:items;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "List of items",
	}

	// Test valid array data
	validData := []interface{}{
		map[string]interface{}{"id": 1, "name": "Item 1"},
		map[string]interface{}{"id": 2, "name": "Item 2"},
	}

	err := validator.ValidateArgumentWithSchema(&arg, schema, validData)
	assert.NoError(t, err)

	// Test invalid array data (missing required field)
	invalidData := []interface{}{
		map[string]interface{}{"id": 1}, // Missing "name"
	}

	err = validator.ValidateArgumentWithSchema(&arg, schema, invalidData)
	assert.Error(t, err)

	// Test empty array (violates minItems)
	emptyData := []interface{}{}

	err = validator.ValidateArgumentWithSchema(&arg, schema, emptyData)
	assert.Error(t, err)
}

func TestInputValidator_WithSchemaValidation(t *testing.T) {
	registry := testRegistry(t)
	validator := NewInputValidator()

	// Create a capability with schema-enabled arguments
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:form=map;textable"`)
	require.NoError(t, err)

	cap := NewCap(urn, "Config Validator", "test-command")

	// Add a custom media spec with schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"value": map[string]interface{}{"type": "string", "minLength": 3},
		},
		"required": []interface{}{"value"},
	}

	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:config;textable;form=map",
		"application/json",
		"https://example.com/schema/config",
		schema,
	))

	cliFlag := "--config"
	pos := 0
	cap.AddArg(CapArg{
		MediaUrn:       "media:config;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Configuration",
	})

	// Test valid input
	validConfig := map[string]interface{}{
		"value": "valid string",
	}

	err = validator.ValidateArguments(cap, []interface{}{validConfig}, registry)
	assert.NoError(t, err)

	// Test invalid input (violates minLength)
	invalidConfig := map[string]interface{}{
		"value": "ab", // Too short
	}

	err = validator.ValidateArguments(cap, []interface{}{invalidConfig}, registry)
	assert.Error(t, err)

	// Should get a ValidationError with schema validation type
	validationErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Equal(t, "SchemaValidationFailed", validationErr.Type)
}

func TestOutputValidator_WithSchemaValidation(t *testing.T) {
	registry := testRegistry(t)
	validator := NewOutputValidator()

	// Create a capability with schema-enabled output
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:form=map;textable"`)
	require.NoError(t, err)

	cap := NewCap(urn, "Output Validator", "test-command")

	// Add a custom media spec with schema for output
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"success", "error"},
			},
			"data": map[string]interface{}{"type": "object"},
		},
		"required": []interface{}{"status"},
	}

	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:result;textable;form=map",
		"application/json",
		"https://example.com/schema/result",
		schema,
	))

	output := NewCapOutput("media:result;textable;form=map", "Command result")
	cap.SetOutput(output)

	// Test valid output
	validOutput := map[string]interface{}{
		"status": "success",
		"data":   map[string]interface{}{"result": "ok"},
	}

	err = validator.ValidateOutput(cap, validOutput, registry)
	assert.NoError(t, err)

	// Test invalid output (invalid enum value)
	invalidOutput := map[string]interface{}{
		"status": "unknown", // Not in enum
		"data":   map[string]interface{}{"result": "ok"},
	}

	err = validator.ValidateOutput(cap, invalidOutput, registry)
	assert.Error(t, err)

	// Should get a ValidationError
	validationErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Equal(t, "OutputValidationFailed", validationErr.Type)
}

func TestCapValidationCoordinator_EndToEnd(t *testing.T) {
	registry := testRegistry(t)
	coordinator := NewCapValidationCoordinator()

	// Create a capability with full schema validation
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=query;out="media:form=map;textable";target=structured`)
	require.NoError(t, err)

	cap := NewCap(urn, "Structured Query", "query-command")

	// Add input argument with schema
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{"type": "string", "minLength": 1},
			"limit": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 100},
		},
		"required": []interface{}{"query"},
	}

	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:query-params;textable;form=map",
		"application/json",
		"https://example.com/schema/query-params",
		inputSchema,
	))

	cliFlag := "--query"
	pos := 0
	cap.AddArg(CapArg{
		MediaUrn:       "media:query-params;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Query parameters",
	})

	// Add output with schema
	outputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"results": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":    map[string]interface{}{"type": "integer"},
						"title": map[string]interface{}{"type": "string"},
					},
				},
			},
			"total": map[string]interface{}{"type": "integer"},
		},
		"required": []interface{}{"results", "total"},
	}

	cap.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:query-results;textable;form=map",
		"application/json",
		"https://example.com/schema/query-results",
		outputSchema,
	))

	output := NewCapOutput("media:query-results;textable;form=map", "Query results")
	cap.SetOutput(output)

	// Register the capability
	coordinator.RegisterCap(cap)

	// Test valid input validation
	validInput := []interface{}{
		map[string]interface{}{
			"query": "search term",
			"limit": 10,
		},
	}

	err = coordinator.ValidateInputs(cap.UrnString(), validInput, registry)
	assert.NoError(t, err)

	// Test invalid input validation
	invalidInput := []interface{}{
		map[string]interface{}{
			"query": "", // Empty string violates minLength
			"limit": 0,  // Zero violates minimum
		},
	}

	err = coordinator.ValidateInputs(cap.UrnString(), invalidInput, registry)
	assert.Error(t, err)

	// Test valid output validation
	validOutput := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": 1, "title": "Result 1"},
			map[string]interface{}{"id": 2, "title": "Result 2"},
		},
		"total": 2,
	}

	err = coordinator.ValidateOutput(cap.UrnString(), validOutput, registry)
	assert.NoError(t, err)

	// Test invalid output validation
	invalidOutput := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": "not_integer", "title": "Result 1"}, // Invalid type
		},
		// Missing required "total" field
	}

	err = coordinator.ValidateOutput(cap.UrnString(), invalidOutput, registry)
	assert.Error(t, err)
}

func TestFileSchemaResolver_ErrorHandling(t *testing.T) {
	resolver := NewFileSchemaResolver("/nonexistent/path")

	_, err := resolver.ResolveSchema("test.schema.json")
	assert.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "SchemaRefNotResolved", schemaErr.Type)
}

func TestComplexNestedSchemaValidation(t *testing.T) {
	validator := NewSchemaValidator()

	// Define a complex nested schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"user": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"profile": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{"type": "string"},
							"settings": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"theme":         map[string]interface{}{"type": "string"},
									"notifications": map[string]interface{}{"type": "boolean"},
								},
							},
						},
						"required": []interface{}{"name"},
					},
					"permissions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
							"enum": []interface{}{"read", "write", "admin"},
						},
					},
				},
				"required": []interface{}{"profile", "permissions"},
			},
		},
		"required": []interface{}{"user"},
	}

	cliFlag := "--user-data"
	pos := 0
	arg := CapArg{
		MediaUrn:       "media:user-data;textable;form=map",
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Complex user data",
	}

	// Test valid complex data
	validData := map[string]interface{}{
		"user": map[string]interface{}{
			"profile": map[string]interface{}{
				"name": "John Doe",
				"settings": map[string]interface{}{
					"theme":         "dark",
					"notifications": true,
				},
			},
			"permissions": []interface{}{"read", "write"},
		},
	}

	err := validator.ValidateArgumentWithSchema(&arg, schema, validData)
	assert.NoError(t, err)

	// Test invalid complex data (invalid permission)
	invalidData := map[string]interface{}{
		"user": map[string]interface{}{
			"profile": map[string]interface{}{
				"name": "John Doe",
			},
			"permissions": []interface{}{"read", "invalid_permission"}, // Invalid enum value
		},
	}

	err = validator.ValidateArgumentWithSchema(&arg, schema, invalidData)
	assert.Error(t, err)
}

func TestMediaUrnResolutionWithMediaSpecs(t *testing.T) {
	registry := testRegistry(t)

	// Media URN resolution requires a mediaSpecs array - no built-in fallback
	// Test resolution with provided mediaSpecs
	mediaSpecs := []MediaSpecDef{
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
		{Urn: MediaInteger, MediaType: "text/plain", ProfileURI: ProfileInt},
		{Urn: MediaObject, MediaType: "application/json", ProfileURI: ProfileObj},
		{Urn: MediaBinary, MediaType: "application/octet-stream"},
	}

	resolved, err := ResolveMediaUrn(MediaString, mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)

	resolved, err = ResolveMediaUrn(MediaInteger, mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileInt, resolved.ProfileURI)

	resolved, err = ResolveMediaUrn(MediaObject, mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.Equal(t, ProfileObj, resolved.ProfileURI)

	resolved, err = ResolveMediaUrn(MediaBinary, mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", resolved.MediaType)
	assert.True(t, resolved.IsBinary())

	// Resolution succeeds from registry when mediaSpecs is nil (fallback to registry)
	resolved, err = ResolveMediaUrn(MediaString, nil, registry)
	require.NoError(t, err, "Resolution should succeed from registry")
	assert.Equal(t, "text/plain", resolved.MediaType)
}

func TestCustomMediaUrnResolution(t *testing.T) {
	registry := testRegistry(t)

	mediaSpecs := []MediaSpecDef{
		{Urn: "media:custom;textable", MediaType: "text/html", ProfileURI: "https://example.com/schema/html"},
		NewMediaSpecDefWithSchema(
			"media:complex;textable;form=map",
			"application/json",
			"https://example.com/schema/complex",
			map[string]interface{}{"type": "object"},
		),
	}

	// Resolution
	resolved, err := ResolveMediaUrn("media:custom;textable", mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "text/html", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/html", resolved.ProfileURI)

	// Object form resolution with schema
	resolved, err = ResolveMediaUrn("media:complex;textable;form=map", mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.NotNil(t, resolved.Schema)

	// Unknown media URN should fail
	_, err = ResolveMediaUrn("media:unknown", mediaSpecs, registry)
	assert.Error(t, err)
}

// ============================================================================
// XV5 VALIDATION TESTS
// TEST054-056: Validate that inline media_specs don't redefine registry specs
// ============================================================================

// TEST054: XV5 - Test inline media spec redefinition of existing registry spec is detected and rejected
func TestXV5InlineSpecRedefinitionDetected(t *testing.T) {
	// Try to redefine a media URN that exists in the registry
	mediaSpecs := map[string]any{
		MediaString: map[string]any{
			"media_type": "text/plain",
			"title":      "My Custom String",
		},
	}

	// Mock registry lookup that returns true for MediaString (it exists in registry)
	mockRegistryLookup := func(mediaUrn string) bool {
		return mediaUrn == MediaString
	}

	result := ValidateNoInlineMediaSpecRedefinition(mediaSpecs, mockRegistryLookup)

	assert.False(t, result.Valid, "Should fail validation when redefining registry spec")
	assert.Contains(t, result.Error, "XV5", "Error should mention XV5")
	assert.Contains(t, result.Redefines, MediaString, "Should identify MediaString as redefined")
}

// TEST055: XV5 - Test new inline media spec (not in registry) is allowed
func TestXV5NewInlineSpecAllowed(t *testing.T) {
	// Define a completely new media spec that doesn't exist in registry
	mediaSpecs := map[string]any{
		"media:my-unique-custom-type-xyz123": map[string]any{
			"media_type": "application/json",
			"title":      "My Custom Output",
		},
	}

	// Mock registry lookup that returns false (spec not in registry)
	mockRegistryLookup := func(mediaUrn string) bool {
		return false
	}

	result := ValidateNoInlineMediaSpecRedefinition(mediaSpecs, mockRegistryLookup)

	assert.True(t, result.Valid, "Should pass validation for new spec not in registry")
	assert.Empty(t, result.Error, "Should not have error message")
}

// TEST056: XV5 - Test empty media_specs (no inline specs) passes XV5 validation
func TestXV5EmptyMediaSpecsAllowed(t *testing.T) {
	// Empty media_specs should pass (with or without registry lookup)
	result := ValidateNoInlineMediaSpecRedefinition(map[string]any{}, nil)
	assert.True(t, result.Valid, "Empty map should pass validation")

	// Nil media_specs should pass
	result = ValidateNoInlineMediaSpecRedefinition(nil, nil)
	assert.True(t, result.Valid, "Nil should pass validation")

	// Graceful degradation: nil lookup function should allow
	mediaSpecs := map[string]any{
		MediaString: map[string]any{
			"media_type": "text/plain",
		},
	}
	result = ValidateNoInlineMediaSpecRedefinition(mediaSpecs, nil)
	assert.True(t, result.Valid, "Should pass when registry lookup not available (graceful degradation)")
}
