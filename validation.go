package capns

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// ValidationError represents validation errors with descriptive failure information
type ValidationError struct {
	Type         string
	CapUrn string
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

// NewUnknownCapError creates an error for unknown caps
func NewUnknownCapError(capUrn string) *ValidationError {
	return &ValidationError{
		Type:         "UnknownCap",
		CapUrn: capUrn,
		Message:      fmt.Sprintf("Unknown cap '%s' - cap not registered or advertised", capUrn),
	}
}

// NewMissingRequiredArgumentError creates an error for missing required arguments
func NewMissingRequiredArgumentError(capUrn, argumentName string) *ValidationError {
	return &ValidationError{
		Type:         "MissingRequiredArgument",
		CapUrn: capUrn,
		ArgumentName: argumentName,
		Message:      fmt.Sprintf("Cap '%s' requires argument '%s' but it was not provided", capUrn, argumentName),
	}
}

// NewInvalidArgumentTypeError creates an error for invalid argument types
func NewInvalidArgumentTypeError(capUrn, argumentName string, expectedType ArgumentType, actualType string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "InvalidArgumentType",
		CapUrn: capUrn,
		ArgumentName: argumentName,
		ExpectedType: string(expectedType),
		ActualType:   actualType,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Cap '%s' argument '%s' expects type '%s' but received '%s' with value: %v", capUrn, argumentName, expectedType, actualType, actualValue),
	}
}

// NewArgumentValidationFailedError creates an error for argument validation failures
func NewArgumentValidationFailedError(capUrn, argumentName, rule string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "ArgumentValidationFailed",
		CapUrn: capUrn,
		ArgumentName: argumentName,
		Rule:         rule,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Cap '%s' argument '%s' failed validation rule '%s' with value: %v", capUrn, argumentName, rule, actualValue),
	}
}

// NewInvalidOutputTypeError creates an error for invalid output types
func NewInvalidOutputTypeError(capUrn string, expectedType OutputType, actualType string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "InvalidOutputType",
		CapUrn: capUrn,
		ExpectedType: string(expectedType),
		ActualType:   actualType,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Cap '%s' output expects type '%s' but received '%s' with value: %v", capUrn, expectedType, actualType, actualValue),
	}
}

// NewOutputValidationFailedError creates an error for output validation failures
func NewOutputValidationFailedError(capUrn, rule string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "OutputValidationFailed",
		CapUrn: capUrn,
		Rule:         rule,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Cap '%s' output failed validation rule '%s' with value: %v", capUrn, rule, actualValue),
	}
}

// NewSchemaValidationFailedError creates an error for schema validation failures
func NewSchemaValidationFailedError(capUrn, argumentName, details string, actualValue interface{}) *ValidationError {
	return &ValidationError{
		Type:         "SchemaValidationFailed",
		CapUrn:       capUrn,
		ArgumentName: argumentName,
		ActualValue:  actualValue,
		Message:      fmt.Sprintf("Cap '%s' argument '%s' failed schema validation: %s", capUrn, argumentName, details),
	}
}

// InputValidator validates arguments against cap input schemas
type InputValidator struct{
	schemaValidator *SchemaValidator
}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	return &InputValidator{
		schemaValidator: NewSchemaValidator(),
	}
}

// NewInputValidatorWithSchemaResolver creates a new input validator with schema resolver
func NewInputValidatorWithSchemaResolver(resolver SchemaResolver) *InputValidator {
	return &InputValidator{
		schemaValidator: NewSchemaValidatorWithResolver(resolver),
	}
}

// ValidateArguments validates arguments against a cap's input schema
func (iv *InputValidator) ValidateArguments(cap *Cap, arguments []interface{}) error {
	capUrn := cap.UrnString()
	args := cap.Arguments

	if args == nil {
		args = NewCapArguments()
	}

	// Check if too many arguments provided
	maxArgs := len(args.Required) + len(args.Optional)
	if len(arguments) > maxArgs {
		return &ValidationError{
			Type:         "TooManyArguments",
			CapUrn: capUrn,
			Message:      fmt.Sprintf("Cap '%s' expects at most %d arguments but received %d", capUrn, maxArgs, len(arguments)),
		}
	}

	// Validate required arguments
	for index, reqArg := range args.Required {
		if index >= len(arguments) {
			return NewMissingRequiredArgumentError(capUrn, reqArg.Name)
		}

		if err := iv.validateSingleArgument(cap, &reqArg, arguments[index]); err != nil {
			return err
		}
	}

	// Validate optional arguments if provided
	requiredCount := len(args.Required)
	for index, optArg := range args.Optional {
		argIndex := requiredCount + index
		if argIndex < len(arguments) {
			if err := iv.validateSingleArgument(cap, &optArg, arguments[argIndex]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (iv *InputValidator) validateSingleArgument(cap *Cap, argDef *CapArgument, value interface{}) error {
	// Type validation
	if err := iv.validateArgumentType(cap, argDef, value); err != nil {
		return err
	}

	// Validation rules
	if err := iv.validateArgumentRules(cap, argDef, value); err != nil {
		return err
	}

	// Schema validation for object/array types
	if err := iv.validateArgumentSchema(cap, argDef, value); err != nil {
		return err
	}

	return nil
}

// validateArgumentSchema validates argument against JSON schema
func (iv *InputValidator) validateArgumentSchema(cap *Cap, argDef *CapArgument, value interface{}) error {
	// Only validate structured types that have schemas
	if argDef.ArgType != ArgumentTypeObject && argDef.ArgType != ArgumentTypeArray {
		return nil
	}

	if err := iv.schemaValidator.ValidateArgument(argDef, value); err != nil {
		if schemaErr, ok := err.(*SchemaValidationError); ok {
			return NewSchemaValidationFailedError(cap.UrnString(), argDef.Name, schemaErr.Details, value)
		}
		return err
	}

	return nil
}

func (iv *InputValidator) validateArgumentType(cap *Cap, argDef *CapArgument, value interface{}) error {
	capUrn := cap.UrnString()
	actualType := getValueTypeName(value)

	typeMatches := false
	switch argDef.ArgType {
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
		return NewInvalidArgumentTypeError(capUrn, argDef.Name, argDef.ArgType, actualType, value)
	}

	return nil
}

func (iv *InputValidator) validateArgumentRules(cap *Cap, argDef *CapArgument, value interface{}) error {
	capUrn := cap.UrnString()
	validation := argDef.Validation

	if validation == nil {
		return nil
	}

	// Numeric validation
	if validation.Min != nil {
		if num, ok := getNumericValue(value); ok {
			if num < *validation.Min {
				return NewArgumentValidationFailedError(capUrn, argDef.Name, fmt.Sprintf("minimum value %v", *validation.Min), value)
			}
		}
	}

	if validation.Max != nil {
		if num, ok := getNumericValue(value); ok {
			if num > *validation.Max {
				return NewArgumentValidationFailedError(capUrn, argDef.Name, fmt.Sprintf("maximum value %v", *validation.Max), value)
			}
		}
	}

	// String length validation
	if validation.MinLength != nil {
		if s, ok := value.(string); ok {
			if len(s) < *validation.MinLength {
				return NewArgumentValidationFailedError(capUrn, argDef.Name, fmt.Sprintf("minimum length %d", *validation.MinLength), value)
			}
		}
	}

	if validation.MaxLength != nil {
		if s, ok := value.(string); ok {
			if len(s) > *validation.MaxLength {
				return NewArgumentValidationFailedError(capUrn, argDef.Name, fmt.Sprintf("maximum length %d", *validation.MaxLength), value)
			}
		}
	}

	// Pattern validation
	if validation.Pattern != nil {
		if s, ok := value.(string); ok {
			if regex, err := regexp.Compile(*validation.Pattern); err == nil {
				if !regex.MatchString(s) {
					return NewArgumentValidationFailedError(capUrn, argDef.Name, fmt.Sprintf("pattern '%s'", *validation.Pattern), value)
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
				return NewArgumentValidationFailedError(capUrn, argDef.Name, fmt.Sprintf("allowed values: %v", validation.AllowedValues), value)
			}
		}
	}

	return nil
}

// OutputValidator validates output against cap output schemas
type OutputValidator struct{
	schemaValidator *SchemaValidator
}

// NewOutputValidator creates a new output validator
func NewOutputValidator() *OutputValidator {
	return &OutputValidator{
		schemaValidator: NewSchemaValidator(),
	}
}

// NewOutputValidatorWithSchemaResolver creates a new output validator with schema resolver
func NewOutputValidatorWithSchemaResolver(resolver SchemaResolver) *OutputValidator {
	return &OutputValidator{
		schemaValidator: NewSchemaValidatorWithResolver(resolver),
	}
}

// ValidateOutput validates output against a cap's output schema
func (ov *OutputValidator) ValidateOutput(cap *Cap, output interface{}) error {
	capUrn := cap.UrnString()

	outputDef := cap.GetOutput()
	if outputDef == nil {
		return &ValidationError{
			Type:         "InvalidCapSchema",
			CapUrn: capUrn,
			Message:      fmt.Sprintf("Cap '%s' has no output definition specified", capUrn),
		}
	}

	// Type validation
	if err := ov.validateOutputType(cap, outputDef, output); err != nil {
		return err
	}

	// Validation rules
	if err := ov.validateOutputRules(cap, outputDef, output); err != nil {
		return err
	}

	// Schema validation for structured outputs
	if err := ov.validateOutputSchema(cap, outputDef, output); err != nil {
		return err
	}

	return nil
}

// validateOutputSchema validates output against JSON schema
func (ov *OutputValidator) validateOutputSchema(cap *Cap, outputDef *CapOutput, value interface{}) error {
	// Only validate structured types that have schemas
	if outputDef.OutputType != OutputTypeObject && outputDef.OutputType != OutputTypeArray {
		return nil
	}

	if err := ov.schemaValidator.ValidateOutput(outputDef, value); err != nil {
		if schemaErr, ok := err.(*SchemaValidationError); ok {
			return NewOutputValidationFailedError(cap.UrnString(), "schema validation: "+schemaErr.Details, value)
		}
		return err
	}

	return nil
}

func (ov *OutputValidator) validateOutputType(cap *Cap, outputDef *CapOutput, value interface{}) error {
	capUrn := cap.UrnString()
	actualType := getValueTypeName(value)

	typeMatches := false
	switch outputDef.OutputType {
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
		return NewInvalidOutputTypeError(capUrn, outputDef.OutputType, actualType, value)
	}

	return nil
}

func (ov *OutputValidator) validateOutputRules(cap *Cap, outputDef *CapOutput, value interface{}) error {
	capUrn := cap.UrnString()
	validation := outputDef.Validation

	if validation == nil {
		return nil
	}

	// Apply same validation rules as arguments
	if validation.Min != nil {
		if num, ok := getNumericValue(value); ok {
			if num < *validation.Min {
				return NewOutputValidationFailedError(capUrn, fmt.Sprintf("minimum value %v", *validation.Min), value)
			}
		}
	}

	if validation.Max != nil {
		if num, ok := getNumericValue(value); ok {
			if num > *validation.Max {
				return NewOutputValidationFailedError(capUrn, fmt.Sprintf("maximum value %v", *validation.Max), value)
			}
		}
	}

	if validation.MinLength != nil {
		if s, ok := value.(string); ok {
			if len(s) < *validation.MinLength {
				return NewOutputValidationFailedError(capUrn, fmt.Sprintf("minimum length %d", *validation.MinLength), value)
			}
		}
	}

	if validation.MaxLength != nil {
		if s, ok := value.(string); ok {
			if len(s) > *validation.MaxLength {
				return NewOutputValidationFailedError(capUrn, fmt.Sprintf("maximum length %d", *validation.MaxLength), value)
			}
		}
	}

	if validation.Pattern != nil {
		if s, ok := value.(string); ok {
			if regex, err := regexp.Compile(*validation.Pattern); err == nil {
				if !regex.MatchString(s) {
					return NewOutputValidationFailedError(capUrn, fmt.Sprintf("pattern '%s'", *validation.Pattern), value)
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
				return NewOutputValidationFailedError(capUrn, fmt.Sprintf("allowed values: %v", validation.AllowedValues), value)
			}
		}
	}

	return nil
}

// CapValidationCoordinator provides centralized validation coordination
type CapValidationCoordinator struct {
	caps            map[string]*Cap
	inputValidator  *InputValidator
	outputValidator *OutputValidator
}

// NewCapValidationCoordinator creates a new validation coordinator
func NewCapValidationCoordinator() *CapValidationCoordinator {
	return &CapValidationCoordinator{
		caps:            make(map[string]*Cap),
		inputValidator:  NewInputValidator(),
		outputValidator: NewOutputValidator(),
	}
}

// NewCapValidationCoordinatorWithSchemaResolver creates a coordinator with schema resolver
func NewCapValidationCoordinatorWithSchemaResolver(resolver SchemaResolver) *CapValidationCoordinator {
	return &CapValidationCoordinator{
		caps:            make(map[string]*Cap),
		inputValidator:  NewInputValidatorWithSchemaResolver(resolver),
		outputValidator: NewOutputValidatorWithSchemaResolver(resolver),
	}
}

// RegisterCap registers a cap schema for validation
func (cvc *CapValidationCoordinator) RegisterCap(cap *Cap) {
	cvc.caps[cap.UrnString()] = cap
}

// GetCap gets a cap by ID
func (cvc *CapValidationCoordinator) GetCap(capUrn string) *Cap {
	return cvc.caps[capUrn]
}

// ValidateInputs validates arguments against a cap's input schema
func (cvc *CapValidationCoordinator) ValidateInputs(capUrn string, arguments []interface{}) error {
	cap := cvc.GetCap(capUrn)
	if cap == nil {
		return NewUnknownCapError(capUrn)
	}

	return cvc.inputValidator.ValidateArguments(cap, arguments)
}

// ValidateOutput validates output against a cap's output schema
func (cvc *CapValidationCoordinator) ValidateOutput(capUrn string, output interface{}) error {
	cap := cvc.GetCap(capUrn)
	if cap == nil {
		return NewUnknownCapError(capUrn)
	}

	return cvc.outputValidator.ValidateOutput(cap, output)
}

// ValidateCapSchema validates a cap definition itself
func (cvc *CapValidationCoordinator) ValidateCapSchema(cap *Cap) error {
	capUrn := cap.UrnString()

	if cap.Arguments == nil {
		return nil
	}

	// Validate that required arguments don't have default values
	for _, arg := range cap.Arguments.Required {
		if arg.DefaultValue != nil {
			return &ValidationError{
				Type:         "InvalidCapSchema",
				CapUrn: capUrn,
				Message:      fmt.Sprintf("Cap '%s' required argument '%s' cannot have a default value", capUrn, arg.Name),
			}
		}
	}

	// Validate argument position uniqueness
	positions := make(map[int]string)
	for _, arg := range cap.Arguments.Required {
		if arg.Position != nil {
			if existing, exists := positions[*arg.Position]; exists {
				return &ValidationError{
					Type:         "InvalidCapSchema",
					CapUrn: capUrn,
					Message:      fmt.Sprintf("Cap '%s' duplicate argument position %d for arguments '%s' and '%s'", capUrn, *arg.Position, existing, arg.Name),
				}
			}
			positions[*arg.Position] = arg.Name
		}
	}
	for _, arg := range cap.Arguments.Optional {
		if arg.Position != nil {
			if existing, exists := positions[*arg.Position]; exists {
				return &ValidationError{
					Type:         "InvalidCapSchema",
					CapUrn: capUrn,
					Message:      fmt.Sprintf("Cap '%s' duplicate argument position %d for arguments '%s' and '%s'", capUrn, *arg.Position, existing, arg.Name),
				}
			}
			positions[*arg.Position] = arg.Name
		}
	}

	// Validate CLI flag uniqueness
	cliFlags := make(map[string]string)
	for _, arg := range cap.Arguments.Required {
		if arg.CliFlag != "" {
			if existing, exists := cliFlags[arg.CliFlag]; exists {
				return &ValidationError{
					Type:         "InvalidCapSchema",
					CapUrn: capUrn,
					Message:      fmt.Sprintf("Cap '%s' duplicate CLI flag '%s' for arguments '%s' and '%s'", capUrn, arg.CliFlag, existing, arg.Name),
				}
			}
			cliFlags[arg.CliFlag] = arg.Name
		}
	}
	for _, arg := range cap.Arguments.Optional {
		if arg.CliFlag != "" {
			if existing, exists := cliFlags[arg.CliFlag]; exists {
				return &ValidationError{
					Type:         "InvalidCapSchema",
					CapUrn: capUrn,
					Message:      fmt.Sprintf("Cap '%s' duplicate CLI flag '%s' for arguments '%s' and '%s'", capUrn, arg.CliFlag, existing, arg.Name),
				}
			}
			cliFlags[arg.CliFlag] = arg.Name
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
	}
	return 0, false
}