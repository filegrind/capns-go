package capdef

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// ValidationError represents validation errors with descriptive failure information
type ValidationError struct {
	Type         string
	CapabilityID string
	ArgumentName string
	ExpectedType string
	ActualType   string
	ActualValue  interface{}
	Rule         string
	Message      string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewUnknownCapabilityError creates an error for unknown capabilities
func NewUnknownCapabilityError(capabilityID string) *ValidationError {
	return &ValidationError{
		Type:         "UnknownCapability",
		CapabilityID: capabilityID,
		Message:      fmt.Sprintf("Unknown capability '%s' - capability not registered or advertised", capabilityID),
	}
}

// NewMissingRequiredArgumentError creates an error for missing required arguments
func NewMissingRequiredArgumentError(capabilityID, argumentName string) *ValidationError {
	return &ValidationError{
		Type:         "MissingRequiredArgument",
		CapabilityID: capabilityID,
		ArgumentName: argumentName,
		Message:      fmt.Sprintf("Capability '%s' requires argument '%s' but it was not provided", capabilityID, argumentName),
	}
}

// NewInvalidArgumentTypeError creates an error for invalid argument types
func NewInvalidArgumentTypeError(capabilityID, argumentName string, expectedType ArgumentType, actualType string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "InvalidArgumentType",
		CapabilityID: capabilityID,
		ArgumentName: argumentName,
		ExpectedType: string(expectedType),
		ActualType:   actualType,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Capability '%s' argument '%s' expects type '%s' but received '%s' with value: %v", capabilityID, argumentName, expectedType, actualType, actualValue),
	}
}

// NewArgumentValidationFailedError creates an error for argument validation failures
func NewArgumentValidationFailedError(capabilityID, argumentName, rule string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "ArgumentValidationFailed",
		CapabilityID: capabilityID,
		ArgumentName: argumentName,
		Rule:         rule,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Capability '%s' argument '%s' failed validation rule '%s' with value: %v", capabilityID, argumentName, rule, actualValue),
	}
}

// NewInvalidOutputTypeError creates an error for invalid output types
func NewInvalidOutputTypeError(capabilityID string, expectedType OutputType, actualType string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "InvalidOutputType",
		CapabilityID: capabilityID,
		ExpectedType: string(expectedType),
		ActualType:   actualType,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Capability '%s' output expects type '%s' but received '%s' with value: %v", capabilityID, expectedType, actualType, actualValue),
	}
}

// NewOutputValidationFailedError creates an error for output validation failures
func NewOutputValidationFailedError(capabilityID, rule string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "OutputValidationFailed",
		CapabilityID: capabilityID,
		Rule:         rule,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Capability '%s' output failed validation rule '%s' with value: %v", capabilityID, rule, actualValue),
	}
}

// InputValidator validates arguments against capability input schemas
type InputValidator struct{}

// ValidateArguments validates arguments against a capability's input schema
func (iv *InputValidator) ValidateArguments(capability *Capability, arguments []interface{}) error {
	capabilityID := capability.IdString()
	args := capability.Arguments

	if args == nil {
		args = NewCapabilityArguments()
	}

	// Check if too many arguments provided
	maxArgs := len(args.Required) + len(args.Optional)
	if len(arguments) > maxArgs {
		return &ValidationError{
			Type:         "TooManyArguments",
			CapabilityID: capabilityID,
			Message:      fmt.Sprintf("Capability '%s' expects at most %d arguments but received %d", capabilityID, maxArgs, len(arguments)),
		}
	}

	// Validate required arguments
	for index, reqArg := range args.Required {
		if index >= len(arguments) {
			return NewMissingRequiredArgumentError(capabilityID, reqArg.Name)
		}

		if err := iv.validateSingleArgument(capability, &reqArg, arguments[index]); err != nil {
			return err
		}
	}

	// Validate optional arguments if provided
	requiredCount := len(args.Required)
	for index, optArg := range args.Optional {
		argIndex := requiredCount + index
		if argIndex < len(arguments) {
			if err := iv.validateSingleArgument(capability, &optArg, arguments[argIndex]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (iv *InputValidator) validateSingleArgument(capability *Capability, argDef *CapabilityArgument, value interface{}) error {
	// Type validation
	if err := iv.validateArgumentType(capability, argDef, value); err != nil {
		return err
	}

	// Validation rules
	if err := iv.validateArgumentRules(capability, argDef, value); err != nil {
		return err
	}

	return nil
}

func (iv *InputValidator) validateArgumentType(capability *Capability, argDef *CapabilityArgument, value interface{}) error {
	capabilityID := capability.IdString()
	actualType := getValueTypeName(value)

	typeMatches := false
	switch argDef.Type {
	case ArgumentTypeString:
		_, typeMatches = value.(string)
	case ArgumentTypeInteger:
		if num, ok := value.(float64); ok {
			typeMatches = num == float64(int64(num))
		} else if _, ok := value.(int); ok {
			typeMatches = true
		} else if _, ok := value.(int64); ok {
			typeMatches = true
		}
	case ArgumentTypeNumber:
		_, ok1 := value.(float64)
		_, ok2 := value.(int)
		_, ok3 := value.(int64)
		typeMatches = ok1 || ok2 || ok3
	case ArgumentTypeBoolean:
		_, typeMatches = value.(bool)
	case ArgumentTypeArray:
		_, typeMatches = value.([]interface{})
	case ArgumentTypeObject:
		_, typeMatches = value.(map[string]interface{})
	case ArgumentTypeBinary:
		_, typeMatches = value.(string) // Binary as base64 string
	}

	if !typeMatches {
		return NewInvalidArgumentTypeError(capabilityID, argDef.Name, argDef.Type, actualType, value)
	}

	return nil
}

func (iv *InputValidator) validateArgumentRules(capability *Capability, argDef *CapabilityArgument, value interface{}) error {
	capabilityID := capability.IdString()
	validation := argDef.Validation

	if validation == nil {
		return nil
	}

	// Numeric validation
	if validation.Min != nil {
		if num, ok := getNumericValue(value); ok {
			if num < *validation.Min {
				return NewArgumentValidationFailedError(capabilityID, argDef.Name, fmt.Sprintf("minimum value %v", *validation.Min), value)
			}
		}
	}

	if validation.Max != nil {
		if num, ok := getNumericValue(value); ok {
			if num > *validation.Max {
				return NewArgumentValidationFailedError(capabilityID, argDef.Name, fmt.Sprintf("maximum value %v", *validation.Max), value)
			}
		}
	}

	// String length validation
	if validation.MinLength != nil {
		if s, ok := value.(string); ok {
			if len(s) < *validation.MinLength {
				return NewArgumentValidationFailedError(capabilityID, argDef.Name, fmt.Sprintf("minimum length %d", *validation.MinLength), value)
			}
		}
	}

	if validation.MaxLength != nil {
		if s, ok := value.(string); ok {
			if len(s) > *validation.MaxLength {
				return NewArgumentValidationFailedError(capabilityID, argDef.Name, fmt.Sprintf("maximum length %d", *validation.MaxLength), value)
			}
		}
	}

	// Pattern validation
	if validation.Pattern != nil {
		if s, ok := value.(string); ok {
			if regex, err := regexp.Compile(*validation.Pattern); err == nil {
				if !regex.MatchString(s) {
					return NewArgumentValidationFailedError(capabilityID, argDef.Name, fmt.Sprintf("pattern '%s'", *validation.Pattern), value)
				}
			}
		}
	}

	// Allowed values validation
	if len(validation.AllowedValues) > 0 {
		if s, ok := value.(string); ok {
			allowed := false
			for _, allowedValue := range validation.AllowedValues {
				if s == allowedValue {
					allowed = true
					break
				}
			}
			if !allowed {
				return NewArgumentValidationFailedError(capabilityID, argDef.Name, fmt.Sprintf("allowed values: %v", validation.AllowedValues), value)
			}
		}
	}

	return nil
}

// OutputValidator validates output against capability output schemas
type OutputValidator struct{}

// ValidateOutput validates output against a capability's output schema
func (ov *OutputValidator) ValidateOutput(capability *Capability, output interface{}) error {
	capabilityID := capability.IdString()

	outputDef := capability.GetOutput()
	if outputDef == nil {
		return &ValidationError{
			Type:         "InvalidCapabilitySchema",
			CapabilityID: capabilityID,
			Message:      fmt.Sprintf("Capability '%s' has no output definition specified", capabilityID),
		}
	}

	// Type validation
	if err := ov.validateOutputType(capability, outputDef, output); err != nil {
		return err
	}

	// Validation rules
	if err := ov.validateOutputRules(capability, outputDef, output); err != nil {
		return err
	}

	return nil
}

func (ov *OutputValidator) validateOutputType(capability *Capability, outputDef *CapabilityOutput, value interface{}) error {
	capabilityID := capability.IdString()
	actualType := getValueTypeName(value)

	typeMatches := false
	switch outputDef.Type {
	case OutputTypeString:
		_, typeMatches = value.(string)
	case OutputTypeInteger:
		if num, ok := value.(float64); ok {
			typeMatches = num == float64(int64(num))
		} else if _, ok := value.(int); ok {
			typeMatches = true
		} else if _, ok := value.(int64); ok {
			typeMatches = true
		}
	case OutputTypeNumber:
		_, ok1 := value.(float64)
		_, ok2 := value.(int)
		_, ok3 := value.(int64)
		typeMatches = ok1 || ok2 || ok3
	case OutputTypeBoolean:
		_, typeMatches = value.(bool)
	case OutputTypeArray:
		_, typeMatches = value.([]interface{})
	case OutputTypeObject:
		_, typeMatches = value.(map[string]interface{})
	case OutputTypeBinary:
		_, typeMatches = value.(string) // Binary as base64 string
	}

	if !typeMatches {
		return NewInvalidOutputTypeError(capabilityID, outputDef.Type, actualType, value)
	}

	return nil
}

func (ov *OutputValidator) validateOutputRules(capability *Capability, outputDef *CapabilityOutput, value interface{}) error {
	capabilityID := capability.IdString()
	validation := outputDef.Validation

	if validation == nil {
		return nil
	}

	// Apply same validation rules as arguments
	if validation.Min != nil {
		if num, ok := getNumericValue(value); ok {
			if num < *validation.Min {
				return NewOutputValidationFailedError(capabilityID, fmt.Sprintf("minimum value %v", *validation.Min), value)
			}
		}
	}

	if validation.Max != nil {
		if num, ok := getNumericValue(value); ok {
			if num > *validation.Max {
				return NewOutputValidationFailedError(capabilityID, fmt.Sprintf("maximum value %v", *validation.Max), value)
			}
		}
	}

	if validation.MinLength != nil {
		if s, ok := value.(string); ok {
			if len(s) < *validation.MinLength {
				return NewOutputValidationFailedError(capabilityID, fmt.Sprintf("minimum length %d", *validation.MinLength), value)
			}
		}
	}

	if validation.MaxLength != nil {
		if s, ok := value.(string); ok {
			if len(s) > *validation.MaxLength {
				return NewOutputValidationFailedError(capabilityID, fmt.Sprintf("maximum length %d", *validation.MaxLength), value)
			}
		}
	}

	if validation.Pattern != nil {
		if s, ok := value.(string); ok {
			if regex, err := regexp.Compile(*validation.Pattern); err == nil {
				if !regex.MatchString(s) {
					return NewOutputValidationFailedError(capabilityID, fmt.Sprintf("pattern '%s'", *validation.Pattern), value)
				}
			}
		}
	}

	if len(validation.AllowedValues) > 0 {
		if s, ok := value.(string); ok {
			allowed := false
			for _, allowedValue := range validation.AllowedValues {
				if s == allowedValue {
					allowed = true
					break
				}
			}
			if !allowed {
				return NewOutputValidationFailedError(capabilityID, fmt.Sprintf("allowed values: %v", validation.AllowedValues), value)
			}
		}
	}

	return nil
}

// SchemaValidator provides centralized validation coordination
type SchemaValidator struct {
	capabilities   map[string]*Capability
	inputValidator *InputValidator
	outputValidator *OutputValidator
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{
		capabilities:    make(map[string]*Capability),
		inputValidator:  &InputValidator{},
		outputValidator: &OutputValidator{},
	}
}

// RegisterCapability registers a capability schema for validation
func (sv *SchemaValidator) RegisterCapability(capability *Capability) {
	sv.capabilities[capability.IdString()] = capability
}

// GetCapability gets a capability by ID
func (sv *SchemaValidator) GetCapability(capabilityID string) *Capability {
	return sv.capabilities[capabilityID]
}

// ValidateInputs validates arguments against a capability's input schema
func (sv *SchemaValidator) ValidateInputs(capabilityID string, arguments []interface{}) error {
	capability := sv.GetCapability(capabilityID)
	if capability == nil {
		return NewUnknownCapabilityError(capabilityID)
	}

	return sv.inputValidator.ValidateArguments(capability, arguments)
}

// ValidateOutput validates output against a capability's output schema
func (sv *SchemaValidator) ValidateOutput(capabilityID string, output interface{}) error {
	capability := sv.GetCapability(capabilityID)
	if capability == nil {
		return NewUnknownCapabilityError(capabilityID)
	}

	return sv.outputValidator.ValidateOutput(capability, output)
}

// ValidateCapabilitySchema validates a capability definition itself
func (sv *SchemaValidator) ValidateCapabilitySchema(capability *Capability) error {
	capabilityID := capability.IdString()

	if capability.Arguments == nil {
		return nil
	}

	// Validate that required arguments don't have default values
	for _, arg := range capability.Arguments.Required {
		if arg.Default != nil {
			return &ValidationError{
				Type:         "InvalidCapabilitySchema",
				CapabilityID: capabilityID,
				Message:      fmt.Sprintf("Capability '%s' required argument '%s' cannot have a default value", capabilityID, arg.Name),
			}
		}
	}

	// Validate argument position uniqueness
	positions := make(map[int]string)
	for _, arg := range capability.Arguments.Required {
		if arg.Position != nil {
			if existing, exists := positions[*arg.Position]; exists {
				return &ValidationError{
					Type:         "InvalidCapabilitySchema",
					CapabilityID: capabilityID,
					Message:      fmt.Sprintf("Capability '%s' duplicate argument position %d for arguments '%s' and '%s'", capabilityID, *arg.Position, existing, arg.Name),
				}
			}
			positions[*arg.Position] = arg.Name
		}
	}
	for _, arg := range capability.Arguments.Optional {
		if arg.Position != nil {
			if existing, exists := positions[*arg.Position]; exists {
				return &ValidationError{
					Type:         "InvalidCapabilitySchema",
					CapabilityID: capabilityID,
					Message:      fmt.Sprintf("Capability '%s' duplicate argument position %d for arguments '%s' and '%s'", capabilityID, *arg.Position, existing, arg.Name),
				}
			}
			positions[*arg.Position] = arg.Name
		}
	}

	// Validate CLI flag uniqueness
	cliFlags := make(map[string]string)
	for _, arg := range capability.Arguments.Required {
		if arg.CliFlag != nil {
			if existing, exists := cliFlags[*arg.CliFlag]; exists {
				return &ValidationError{
					Type:         "InvalidCapabilitySchema",
					CapabilityID: capabilityID,
					Message:      fmt.Sprintf("Capability '%s' duplicate CLI flag '%s' for arguments '%s' and '%s'", capabilityID, *arg.CliFlag, existing, arg.Name),
				}
			}
			cliFlags[*arg.CliFlag] = arg.Name
		}
	}
	for _, arg := range capability.Arguments.Optional {
		if arg.CliFlag != nil {
			if existing, exists := cliFlags[*arg.CliFlag]; exists {
				return &ValidationError{
					Type:         "InvalidCapabilitySchema",
					CapabilityID: capabilityID,
					Message:      fmt.Sprintf("Capability '%s' duplicate CLI flag '%s' for arguments '%s' and '%s'", capabilityID, *arg.CliFlag, existing, arg.Name),
				}
			}
			cliFlags[*arg.CliFlag] = arg.Name
		}
	}

	return nil
}

// Utility functions

func getValueTypeName(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64:
		return "integer"
	case float32, float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case json.Number:
		if _, err := v.Int64(); err == nil {
			return "integer"
		}
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func getNumericValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}