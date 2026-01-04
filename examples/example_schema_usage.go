// Example demonstrating schema validation usage
// This file shows how to use the comprehensive JSON schema validation system
package main

import (
	"fmt"

	capns "github.com/fgrnd/cap-sdk-go"
)

func main() {
	// Example 1: Create capability with embedded schema
	fmt.Println("=== Example 1: Basic Schema Validation ===")
	
	urn, _ := capns.NewCapUrnFromString("cap:action=query;target=structured;")
	cap := capns.NewCap(urn, "Query Command", "query-command")

	// Define JSON schema for user data
	userSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":      "string",
				"minLength": 2,
			},
			"age": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
				"maximum": 150,
			},
			"email": map[string]interface{}{
				"type":   "string",
				"format": "email",
			},
		},
		"required": []interface{}{"name", "age"},
	}

	// Add argument with schema
	userArg := capns.NewCapArgumentWithSchema("user", capns.ArgumentTypeObject, "User data", "--user", userSchema)
	cap.AddRequiredArgument(userArg)

	// Create validator and test
	validator := capns.NewSchemaValidator()

	// Valid data
	validUser := map[string]interface{}{
		"name":  "John Doe",
		"age":   30,
		"email": "john@example.com",
	}

	err := validator.ValidateArgument(&userArg, validUser)
	if err != nil {
		fmt.Printf("ERR Validation failed: %v\n", err)
	} else {
		fmt.Printf("OK Valid data passed validation\n")
	}

	// Invalid data
	invalidUser := map[string]interface{}{
		"name": "A", // Too short
		"age":  -5,  // Negative age
	}

	err = validator.ValidateArgument(&userArg, invalidUser)
	if err != nil {
		fmt.Printf("OK Invalid data correctly rejected: %v\n", err)
	} else {
		fmt.Printf("ERR Invalid data incorrectly accepted\n")
	}

	// Example 2: Output validation
	fmt.Println("\n=== Example 2: Output Schema Validation ===")

	outputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"success", "error", "pending"},
			},
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
			"total": map[string]interface{}{
				"type":    "integer",
				"minimum": 0,
			},
		},
		"required": []interface{}{"status", "total"},
	}

	output := capns.NewCapOutputWithEmbeddedSchema(capns.OutputTypeObject, "Query results", outputSchema)
	cap.SetOutput(output)

	// Valid output
	validOutput := map[string]interface{}{
		"status": "success",
		"results": []interface{}{
			map[string]interface{}{"id": 1, "title": "Result 1"},
			map[string]interface{}{"id": 2, "title": "Result 2"},
		},
		"total": 2,
	}

	err = validator.ValidateOutput(output, validOutput)
	if err != nil {
		fmt.Printf("ERR Output validation failed: %v\n", err)
	} else {
		fmt.Printf("OK Valid output passed validation\n")
	}

	// Example 3: Integration with CapValidationCoordinator
	fmt.Println("\n=== Example 3: Full Integration ===")

	coordinator := capns.NewCapValidationCoordinator()
	coordinator.RegisterCap(cap)

	// Test input validation through coordinator
	positionalArgs := []interface{}{validUser}
	err = coordinator.ValidateInputs(cap.UrnString(), positionalArgs)
	if err != nil {
		fmt.Printf("ERR Coordinator input validation failed: %v\n", err)
	} else {
		fmt.Printf("OK Coordinator input validation passed\n")
	}

	// Test output validation through coordinator
	err = coordinator.ValidateOutput(cap.UrnString(), validOutput)
	if err != nil {
		fmt.Printf("ERR Coordinator output validation failed: %v\n", err)
	} else {
		fmt.Printf("OK Coordinator output validation passed\n")
	}

	// Example 4: Array schema validation
	fmt.Println("\n=== Example 4: Array Schema Validation ===")

	arraySchema := map[string]interface{}{
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
		"maxItems": 10,
	}

	itemsArg := capns.NewCapArgumentWithSchema("items", capns.ArgumentTypeArray, "List of items", "--items", arraySchema)

	validArray := []interface{}{
		map[string]interface{}{"id": 1, "name": "Item 1"},
		map[string]interface{}{"id": 2, "name": "Item 2"},
	}

	err = validator.ValidateArgument(&itemsArg, validArray)
	if err != nil {
		fmt.Printf("ERR Array validation failed: %v\n", err)
	} else {
		fmt.Printf("OK Valid array passed validation\n")
	}

	fmt.Println("\n=== Schema validation examples completed successfully! ===")
}