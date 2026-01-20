package capns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a cap with media specs for testing
func createCapWithSchema(t *testing.T, argSchema interface{}) *Cap {
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=object;v=1;textable;keyed"`)
	require.NoError(t, err)

	cap := NewCap(urn, "Test Cap", "test-command")

	// Add a custom media spec with the provided schema
	cap.AddMediaSpec("media:type=test-obj;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://test.example.com/schema",
		argSchema,
	))

	return cap
}

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

	// Create an argument
	arg := NewCapArgument("user_data", "media:type=test-obj;v=1;textable;keyed", "User data", "--user")

	// Test valid data
	validData := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
	}

	err := validator.ValidateArgumentWithSchema(&arg, schema, validData)
	assert.NoError(t, err)
}

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

	// Create an argument
	arg := NewCapArgument("user_data", "media:type=test-obj;v=1;textable;keyed", "User data", "--user")

	// Test invalid data (missing required field)
	invalidData := map[string]interface{}{
		"age": 30,
	}

	err := validator.ValidateArgumentWithSchema(&arg, schema, invalidData)
	assert.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "ArgumentValidation", schemaErr.Type)
	assert.Equal(t, "user_data", schemaErr.Argument)
	assert.Contains(t, schemaErr.Details, "name")
}

func TestSchemaValidator_ValidateArgumentWithSchema_NilSchema(t *testing.T) {
	validator := NewSchemaValidator()

	// Create argument
	arg := NewCapArgument("simple_string", MediaString, "Simple string", "--string")

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
	output := NewCapOutput("media:type=test-result;v=1;textable;keyed", "Query result")

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
	output := NewCapOutput("media:type=test-result;v=1;textable;keyed", "Query result")

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
	validator := NewSchemaValidator()

	// Create a capability with schema-enabled arguments
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=query;out="media:type=object;v=1;textable;keyed";target=structured`)
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

	cap.AddMediaSpec("media:type=user;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/user",
		userSchema,
	))

	// Add argument referencing the custom spec
	userArg := NewCapArgument("user", "media:type=user;v=1;textable;keyed", "User data", "--user")
	cap.AddRequiredArgument(userArg)

	// Test valid arguments
	validUser := map[string]interface{}{
		"name": "Alice",
		"age":  25,
	}

	namedArgs := map[string]interface{}{
		"user": validUser,
	}

	err = validator.ValidateArguments(cap, []interface{}{}, namedArgs)
	assert.NoError(t, err)

	// Test invalid arguments
	invalidUser := map[string]interface{}{
		"age": 25, // Missing required "name"
	}

	namedArgs = map[string]interface{}{
		"user": invalidUser,
	}

	err = validator.ValidateArguments(cap, []interface{}{}, namedArgs)
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

	// Create an argument
	arg := NewCapArgument("items", "media:type=items;v=1;textable;keyed", "List of items", "--items")

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
	validator := NewInputValidator()

	// Create a capability with schema-enabled arguments
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=object;v=1;textable;keyed"`)
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

	cap.AddMediaSpec("media:type=config;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/config",
		schema,
	))

	arg := NewCapArgument("config", "media:type=config;v=1;textable;keyed", "Configuration", "--config")
	cap.AddRequiredArgument(arg)

	// Test valid input
	validConfig := map[string]interface{}{
		"value": "valid string",
	}

	err = validator.ValidateArguments(cap, []interface{}{validConfig})
	assert.NoError(t, err)

	// Test invalid input (violates minLength)
	invalidConfig := map[string]interface{}{
		"value": "ab", // Too short
	}

	err = validator.ValidateArguments(cap, []interface{}{invalidConfig})
	assert.Error(t, err)

	// Should get a ValidationError with schema validation type
	validationErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Equal(t, "SchemaValidationFailed", validationErr.Type)
}

func TestOutputValidator_WithSchemaValidation(t *testing.T) {
	validator := NewOutputValidator()

	// Create a capability with schema-enabled output
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=object;v=1;textable;keyed"`)
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

	cap.AddMediaSpec("media:type=result;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/result",
		schema,
	))

	output := NewCapOutput("media:type=result;v=1;textable;keyed", "Command result")
	cap.SetOutput(output)

	// Test valid output
	validOutput := map[string]interface{}{
		"status": "success",
		"data":   map[string]interface{}{"result": "ok"},
	}

	err = validator.ValidateOutput(cap, validOutput)
	assert.NoError(t, err)

	// Test invalid output (invalid enum value)
	invalidOutput := map[string]interface{}{
		"status": "unknown", // Not in enum
		"data":   map[string]interface{}{"result": "ok"},
	}

	err = validator.ValidateOutput(cap, invalidOutput)
	assert.Error(t, err)

	// Should get a ValidationError
	validationErr, ok := err.(*ValidationError)
	require.True(t, ok)
	assert.Equal(t, "OutputValidationFailed", validationErr.Type)
}

func TestCapValidationCoordinator_EndToEnd(t *testing.T) {
	coordinator := NewCapValidationCoordinator()

	// Create a capability with full schema validation
	urn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=query;out="media:type=object;v=1;textable;keyed";target=structured`)
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

	cap.AddMediaSpec("media:type=query-params;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/query-params",
		inputSchema,
	))

	queryArg := NewCapArgument("query_params", "media:type=query-params;v=1;textable;keyed", "Query parameters", "--query")
	cap.AddRequiredArgument(queryArg)

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

	cap.AddMediaSpec("media:type=query-results;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/query-results",
		outputSchema,
	))

	output := NewCapOutput("media:type=query-results;v=1;textable;keyed", "Query results")
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

	err = coordinator.ValidateInputs(cap.UrnString(), validInput)
	assert.NoError(t, err)

	// Test invalid input validation
	invalidInput := []interface{}{
		map[string]interface{}{
			"query": "", // Empty string violates minLength
			"limit": 0,  // Zero violates minimum
		},
	}

	err = coordinator.ValidateInputs(cap.UrnString(), invalidInput)
	assert.Error(t, err)

	// Test valid output validation
	validOutput := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": 1, "title": "Result 1"},
			map[string]interface{}{"id": 2, "title": "Result 2"},
		},
		"total": 2,
	}

	err = coordinator.ValidateOutput(cap.UrnString(), validOutput)
	assert.NoError(t, err)

	// Test invalid output validation
	invalidOutput := map[string]interface{}{
		"results": []interface{}{
			map[string]interface{}{"id": "not_integer", "title": "Result 1"}, // Invalid type
		},
		// Missing required "total" field
	}

	err = coordinator.ValidateOutput(cap.UrnString(), invalidOutput)
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

	arg := NewCapArgument("user_data", "media:type=user-data;v=1;textable;keyed", "Complex user data", "--user-data")

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

func TestBuiltinMediaUrnResolution(t *testing.T) {
	// Test that built-in media URNs can be resolved
	resolved, err := ResolveMediaUrn(MediaString, nil)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)

	resolved, err = ResolveMediaUrn(MediaInteger, nil)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileInt, resolved.ProfileURI)

	resolved, err = ResolveMediaUrn(MediaObject, nil)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.Equal(t, ProfileObj, resolved.ProfileURI)

	resolved, err = ResolveMediaUrn(MediaBinary, nil)
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", resolved.MediaType)
	assert.True(t, resolved.IsBinary())
}

func TestCustomMediaUrnResolution(t *testing.T) {
	mediaSpecs := map[string]MediaSpecDef{
		"media:type=custom;v=1;textable": NewMediaSpecDefString("text/html; profile=https://example.com/schema/html"),
		"media:type=complex;v=1;textable;keyed": NewMediaSpecDefObjectWithSchema(
			"application/json",
			"https://example.com/schema/complex",
			map[string]interface{}{"type": "object"},
		),
	}

	// String form resolution
	resolved, err := ResolveMediaUrn("media:type=custom;v=1;textable", mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, "text/html", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/html", resolved.ProfileURI)

	// Object form resolution with schema
	resolved, err = ResolveMediaUrn("media:type=complex;v=1;textable;keyed", mediaSpecs)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.NotNil(t, resolved.Schema)

	// Unknown media URN should fail
	_, err = ResolveMediaUrn("media:type=unknown;v=1", mediaSpecs)
	assert.Error(t, err)
}
