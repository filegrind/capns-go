package capns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaValidator_ValidateArgument_Success(t *testing.T) {
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

	// Create an argument with embedded schema
	arg := NewCapArgumentWithSchema("user_data", ArgumentTypeObject, "User data", "--user", schema)

	// Test valid data
	validData := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
	}

	err := validator.ValidateArgument(&arg, validData)
	assert.NoError(t, err)
}

func TestSchemaValidator_ValidateArgument_Failure(t *testing.T) {
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

	// Create an argument with embedded schema
	arg := NewCapArgumentWithSchema("user_data", ArgumentTypeObject, "User data", "--user", schema)

	// Test invalid data (missing required field)
	invalidData := map[string]interface{}{
		"age": 30,
	}

	err := validator.ValidateArgument(&arg, invalidData)
	assert.Error(t, err)
	
	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "ArgumentValidation", schemaErr.Type)
	assert.Equal(t, "user_data", schemaErr.Argument)
	assert.Contains(t, schemaErr.Details, "name")
}

func TestSchemaValidator_ValidateArgument_SkipNonStructuredTypes(t *testing.T) {
	validator := NewSchemaValidator()

	// String argument should not be schema validated
	arg := NewCapArgument("simple_string", ArgumentTypeString, "Simple string", "--string")

	err := validator.ValidateArgument(&arg, "any string value")
	assert.NoError(t, err)
}

func TestSchemaValidator_ValidateArgument_NoSchemaSkipsValidation(t *testing.T) {
	validator := NewSchemaValidator()

	// Object argument without schema should not be validated
	arg := NewCapArgument("data", ArgumentTypeObject, "Data object", "--data")

	err := validator.ValidateArgument(&arg, map[string]interface{}{"any": "data"})
	assert.NoError(t, err)
}

func TestSchemaValidator_ValidateOutput_Success(t *testing.T) {
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

	// Create output with embedded schema
	output := NewCapOutputWithEmbeddedSchema(OutputTypeObject, "Query result", schema)

	// Test valid output data
	validData := map[string]interface{}{
		"result":    "success",
		"timestamp": "2023-01-01T00:00:00Z",
	}

	err := validator.ValidateOutput(output, validData)
	assert.NoError(t, err)
}

func TestSchemaValidator_ValidateOutput_Failure(t *testing.T) {
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

	// Create output with embedded schema
	output := NewCapOutputWithEmbeddedSchema(OutputTypeObject, "Query result", schema)

	// Test invalid output data (missing required field)
	invalidData := map[string]interface{}{
		"status": "ok",
	}

	err := validator.ValidateOutput(output, invalidData)
	assert.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "OutputValidation", schemaErr.Type)
	assert.Contains(t, schemaErr.Details, "result")
}

func TestSchemaValidator_ValidateArguments_Integration(t *testing.T) {
	validator := NewSchemaValidator()

	// Create a capability with schema-enabled arguments
	urn, err := NewCapUrnFromString("cap:action=query;target=structured;")
	require.NoError(t, err)

	cap := NewCap(urn, "test-command")

	// Add argument with schema
	userSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer", "minimum": 0},
		},
		"required": []interface{}{"name"},
	}

	userArg := NewCapArgumentWithSchema("user", ArgumentTypeObject, "User data", "--user", userSchema)
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

	// Create an argument with array schema
	arg := NewCapArgumentWithSchema("items", ArgumentTypeArray, "List of items", "--items", schema)

	// Test valid array data
	validData := []interface{}{
		map[string]interface{}{"id": 1, "name": "Item 1"},
		map[string]interface{}{"id": 2, "name": "Item 2"},
	}

	err := validator.ValidateArgument(&arg, validData)
	assert.NoError(t, err)

	// Test invalid array data (missing required field)
	invalidData := []interface{}{
		map[string]interface{}{"id": 1}, // Missing "name"
	}

	err = validator.ValidateArgument(&arg, invalidData)
	assert.Error(t, err)

	// Test empty array (violates minItems)
	emptyData := []interface{}{}

	err = validator.ValidateArgument(&arg, emptyData)
	assert.Error(t, err)
}

func TestSchemaValidator_WithSchemaReference(t *testing.T) {
	// Create a mock resolver that fails (to test error handling)
	resolver := &mockSchemaResolver{
		shouldFail: true,
	}

	validator := NewSchemaValidatorWithResolver(resolver)

	// Create an argument with schema reference
	arg := NewCapArgumentWithSchemaRef("user", ArgumentTypeObject, "User data", "--user", "user.schema.json")

	// Test that validation fails when schema cannot be resolved
	data := map[string]interface{}{"name": "John"}

	err := validator.ValidateArgument(&arg, data)
	assert.Error(t, err)

	schemaErr, ok := err.(*SchemaValidationError)
	require.True(t, ok)
	assert.Equal(t, "SchemaRefNotResolved", schemaErr.Type)
}

func TestInputValidator_WithSchemaValidation(t *testing.T) {
	validator := NewInputValidator()

	// Create a capability with schema-enabled arguments
	urn, err := NewCapUrnFromString("cap:action=test;")
	require.NoError(t, err)

	cap := NewCap(urn, "test-command")

	// Add argument with schema
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"value": map[string]interface{}{"type": "string", "minLength": 3},
		},
		"required": []interface{}{"value"},
	}

	arg := NewCapArgumentWithSchema("config", ArgumentTypeObject, "Configuration", "--config", schema)
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
	urn, err := NewCapUrnFromString("cap:action=test;")
	require.NoError(t, err)

	cap := NewCap(urn, "test-command")

	// Add output with schema
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

	output := NewCapOutputWithEmbeddedSchema(OutputTypeObject, "Command result", schema)
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
	urn, err := NewCapUrnFromString("cap:action=query;target=structured;")
	require.NoError(t, err)

	cap := NewCap(urn, "query-command")

	// Add input argument with schema
	inputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{"type": "string", "minLength": 1},
			"limit": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 100},
		},
		"required": []interface{}{"query"},
	}

	queryArg := NewCapArgumentWithSchema("query_params", ArgumentTypeObject, "Query parameters", "--query", inputSchema)
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

	output := NewCapOutputWithEmbeddedSchema(OutputTypeObject, "Query results", outputSchema)
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

// Mock schema resolver for testing
type mockSchemaResolver struct {
	shouldFail bool
	schemas    map[string]interface{}
}

func (m *mockSchemaResolver) ResolveSchema(schemaRef string) (interface{}, error) {
	if m.shouldFail {
		return nil, &SchemaValidationError{
			Type:    "SchemaRefNotResolved",
			Details: "Mock resolver failure",
		}
	}

	if schema, exists := m.schemas[schemaRef]; exists {
		return schema, nil
	}

	return nil, &SchemaValidationError{
		Type:    "SchemaRefNotResolved",
		Details: "Schema not found in mock resolver",
	}
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

	arg := NewCapArgumentWithSchema("user_data", ArgumentTypeObject, "Complex user data", "--user-data", schema)

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

	err := validator.ValidateArgument(&arg, validData)
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

	err = validator.ValidateArgument(&arg, invalidData)
	assert.Error(t, err)
}