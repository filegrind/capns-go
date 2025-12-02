package capns

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaValidationError represents errors that occur during JSON schema validation
type SchemaValidationError struct {
	Type      string      `json:"type"`
	CapUrn    string      `json:"cap_urn,omitempty"`
	Argument  string      `json:"argument,omitempty"`
	Details   string      `json:"details"`
	Context   string      `json:"context,omitempty"`
	Value     interface{} `json:"value,omitempty"`
}

func (e *SchemaValidationError) Error() string {
	if e.Argument != "" {
		return fmt.Sprintf("Schema validation failed for argument '%s': %s", e.Argument, e.Details)
	}
	return fmt.Sprintf("Schema validation failed: %s", e.Details)
}

// SchemaResolver interface for resolving external schema references
type SchemaResolver interface {
	ResolveSchema(schemaRef string) (interface{}, error)
}

// FileSchemaResolver implements SchemaResolver for file-based schemas
type FileSchemaResolver struct {
	basePath string
}

// NewFileSchemaResolver creates a new file-based schema resolver
func NewFileSchemaResolver(basePath string) *FileSchemaResolver {
	return &FileSchemaResolver{
		basePath: basePath,
	}
}

// ResolveSchema resolves a schema reference to a JSON schema
func (f *FileSchemaResolver) ResolveSchema(schemaRef string) (interface{}, error) {
	// This is a simple implementation - in production you might want
	// to support HTTP URLs, caching, etc.
	schemaPath := f.basePath + "/" + schemaRef
	
	// For now, return an error indicating that file resolution is not implemented
	// In a full implementation, you would read the file and parse the JSON
	return nil, &SchemaValidationError{
		Type:    "SchemaRefNotResolved",
		Details: fmt.Sprintf("Schema reference '%s' could not be resolved from path '%s'", schemaRef, schemaPath),
	}
}

// SchemaValidator provides JSON Schema Draft-7 validation capabilities
type SchemaValidator struct {
	resolver SchemaResolver
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

// NewSchemaValidatorWithResolver creates a new schema validator with a schema resolver
func NewSchemaValidatorWithResolver(resolver SchemaResolver) *SchemaValidator {
	return &SchemaValidator{
		resolver: resolver,
	}
}

// ValidateArgument validates a single argument value against its schema
func (sv *SchemaValidator) ValidateArgument(arg *CapArgument, value interface{}) error {
	// Only validate object and array types that have schemas defined
	if arg.ArgType != ArgumentTypeObject && arg.ArgType != ArgumentTypeArray {
		return nil
	}

	schema, err := sv.resolveArgumentSchema(arg)
	if err != nil {
		return err
	}

	// No schema specified, skip validation
	if schema == nil {
		return nil
	}

	return sv.validateValueAgainstSchema(arg.Name, value, schema, "argument")
}

// ValidateOutput validates output value against its schema
func (sv *SchemaValidator) ValidateOutput(output *CapOutput, value interface{}) error {
	// Only validate object and array types that have schemas defined
	if output.OutputType != OutputTypeObject && output.OutputType != OutputTypeArray {
		return nil
	}

	schema, err := sv.resolveOutputSchema(output)
	if err != nil {
		return err
	}

	// No schema specified, skip validation
	if schema == nil {
		return nil
	}

	return sv.validateValueAgainstSchema("output", value, schema, "output")
}

// ValidateArguments validates all arguments for a capability
func (sv *SchemaValidator) ValidateArguments(cap *Cap, arguments []interface{}, namedArgs map[string]interface{}) error {
	if cap.Arguments == nil {
		return nil
	}

	// Validate positional required arguments
	for i, argDef := range cap.Arguments.Required {
		var value interface{}
		var found bool

		// Check if this argument has a position
		if argDef.Position != nil {
			if *argDef.Position < len(arguments) {
				value = arguments[*argDef.Position]
				found = true
			}
		} else if i < len(arguments) {
			// Use index-based position
			value = arguments[i]
			found = true
		}

		// Also check named arguments
		if namedArgs != nil {
			if namedValue, exists := namedArgs[argDef.Name]; exists {
				value = namedValue
				found = true
			}
		}

		if found {
			if err := sv.ValidateArgument(&argDef, value); err != nil {
				return err
			}
		}
	}

	// Validate optional arguments if provided
	for _, argDef := range cap.Arguments.Optional {
		var value interface{}
		var found bool

		// Check named arguments first for optional args
		if namedArgs != nil {
			if namedValue, exists := namedArgs[argDef.Name]; exists {
				value = namedValue
				found = true
			}
		}

		// Check positional if not found in named args
		if !found && argDef.Position != nil {
			if *argDef.Position < len(arguments) {
				value = arguments[*argDef.Position]
				found = true
			}
		}

		if found {
			if err := sv.ValidateArgument(&argDef, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveArgumentSchema resolves the schema for an argument
func (sv *SchemaValidator) resolveArgumentSchema(arg *CapArgument) (interface{}, error) {
	// Prefer embedded schema over schema reference
	if arg.Schema != nil {
		return arg.Schema, nil
	}

	if arg.SchemaRef != nil {
		if sv.resolver == nil {
			return nil, &SchemaValidationError{
				Type:     "SchemaRefNotResolved",
				Argument: arg.Name,
				Details:  fmt.Sprintf("Schema reference '%s' specified but no resolver configured", *arg.SchemaRef),
			}
		}
		return sv.resolver.ResolveSchema(*arg.SchemaRef)
	}

	// No schema specified
	return nil, nil
}

// resolveOutputSchema resolves the schema for output
func (sv *SchemaValidator) resolveOutputSchema(output *CapOutput) (interface{}, error) {
	// Prefer embedded schema over schema reference
	if output.Schema != nil {
		return output.Schema, nil
	}

	if output.SchemaRef != nil {
		if sv.resolver == nil {
			return nil, &SchemaValidationError{
				Type:    "SchemaRefNotResolved",
				Details: fmt.Sprintf("Schema reference '%s' specified but no resolver configured", *output.SchemaRef),
			}
		}
		return sv.resolver.ResolveSchema(*output.SchemaRef)
	}

	// No schema specified
	return nil, nil
}

// validateValueAgainstSchema performs the actual JSON schema validation
func (sv *SchemaValidator) validateValueAgainstSchema(name string, value interface{}, schema interface{}, context string) error {
	// Convert schema to JSON string for gojsonschema
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return &SchemaValidationError{
			Type:    "SchemaCompilation",
			Details: fmt.Sprintf("Failed to marshal schema: %v", err),
			Context: context,
		}
	}

	// Convert value to JSON string for validation
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return &SchemaValidationError{
			Type:    "InvalidJson",
			Details: fmt.Sprintf("Failed to marshal value for validation: %v", err),
			Context: context,
			Value:   value,
		}
	}

	// Create schema and document loaders
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
	documentLoader := gojsonschema.NewBytesLoader(valueBytes)

	// Validate
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return &SchemaValidationError{
			Type:    "SchemaCompilation",
			Details: fmt.Sprintf("Failed to validate schema: %v", err),
			Context: context,
		}
	}

	// Check validation results
	if !result.Valid() {
		var errorDetails []string
		for _, desc := range result.Errors() {
			errorDetails = append(errorDetails, fmt.Sprintf("  - %s", desc))
		}

		if context == "argument" {
			return &SchemaValidationError{
				Type:     "ArgumentValidation",
				Argument: name,
				Details:  strings.Join(errorDetails, "\n"),
				Value:    value,
			}
		} else {
			return &SchemaValidationError{
				Type:    "OutputValidation",
				Details: strings.Join(errorDetails, "\n"),
				Value:   value,
			}
		}
	}

	return nil
}

