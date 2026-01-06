// Package capns provides cap-based execution with strict input validation
package capns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CapCaller executes caps via host service with strict validation
type CapCaller struct {
	cap           string
	capHost       CapHost
	capDefinition *Cap
}

// CapHost defines the interface for cap host communication
type CapHost interface {
	ExecuteCap(
		ctx context.Context,
		capUrn string,
		positionalArgs []string,
		namedArgs map[string]string,
		stdinData []byte,
	) (*HostResult, error)
}

// HostResult represents the result from cap execution
type HostResult struct {
	BinaryOutput []byte
	TextOutput   string
}

// NewCapCaller creates a new cap caller with validation
func NewCapCaller(cap string, capHost CapHost, capDefinition *Cap) *CapCaller {
	return &CapCaller{
		cap:           cap,
		capHost:       capHost,
		capDefinition: capDefinition,
	}
}

// Call executes the cap with structured arguments and optional stdin data
// Validates inputs against cap definition before execution
func (cc *CapCaller) Call(
	ctx context.Context,
	positionalArgs []interface{},
	namedArgs []interface{},
	stdinData []byte,
) (*ResponseWrapper, error) {
	// Validate inputs against cap definition
	if err := cc.validateInputs(positionalArgs, namedArgs); err != nil {
		return nil, fmt.Errorf("input validation failed for %s: %w", cc.cap, err)
	}

	// Convert JSON positional args to strings
	stringPositionalArgs := make([]string, len(positionalArgs))
	for i, arg := range positionalArgs {
		stringPositionalArgs[i] = cc.convertToString(arg)
	}

	// Convert JSON named args to map[string]string
	stringNamedArgs := make(map[string]string)
	for _, arg := range namedArgs {
		if argMap, ok := arg.(map[string]interface{}); ok {
			if name, nameOk := argMap["name"].(string); nameOk {
				if value, valueOk := argMap["value"]; valueOk {
					stringNamedArgs[name] = cc.convertToString(value)
				}
			}
		}
	}

	// Execute via cap host method with stdin support
	result, err := cc.capHost.ExecuteCap(
		ctx,
		cc.cap,
		stringPositionalArgs,
		stringNamedArgs,
		stdinData,
	)
	if err != nil {
		return nil, fmt.Errorf("cap execution failed: %w", err)
	}

	// Determine response type based on what was returned
	var response *ResponseWrapper
	if len(result.BinaryOutput) > 0 {
		response = NewResponseWrapperFromBinary(result.BinaryOutput)
	} else if result.TextOutput != "" {
		if cc.isJsonCap() {
			response = NewResponseWrapperFromJSON([]byte(result.TextOutput))
		} else {
			response = NewResponseWrapperFromText([]byte(result.TextOutput))
		}
	} else {
		return nil, fmt.Errorf("cap returned no output")
	}

	// Validate output against cap definition
	if err := cc.validateOutput(response); err != nil {
		return nil, fmt.Errorf("output validation failed for %s: %w", cc.cap, err)
	}

	return response, nil
}

// convertToString converts various types to string
func (cc *CapCaller) convertToString(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return ""
	default:
		// For complex types, marshal to JSON
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", v)
	}
}

// capToCommand converts cap name to command
func (cc *CapCaller) capToCommand(cap string) string {
	// Extract operation part (everything before the last colon)
	if colonPos := strings.LastIndex(cap, ":"); colonPos != -1 {
		cap = cap[:colonPos]
	}

	// Convert underscores to hyphens for command name
	return strings.ReplaceAll(cap, "_", "-")
}

// isBinaryCap checks if this cap produces binary output based on media_spec
func (cc *CapCaller) isBinaryCap() bool {
	capUrn, err := NewCapUrnFromString(cc.cap)
	if err != nil {
		return false
	}

	mediaSpec, err := GetMediaSpecFromCapUrn(capUrn)
	if err != nil {
		return false
	}
	return mediaSpec.IsBinary()
}

// isJsonCap checks if this cap should produce JSON output based on media_spec
func (cc *CapCaller) isJsonCap() bool {
	capUrn, err := NewCapUrnFromString(cc.cap)
	if err != nil {
		return false
	}

	mediaSpec, err := GetMediaSpecFromCapUrn(capUrn)
	if err != nil {
		// Default to text/plain (not JSON) if no media_spec is specified
		return false
	}
	return mediaSpec.IsJSON()
}

// validateInputs validates input arguments against cap definition
func (cc *CapCaller) validateInputs(positionalArgs, namedArgs []interface{}) error {
	// Create enhanced input validator with schema support
	inputValidator := NewInputValidator()
	
	// Convert named args to map for validation
	namedArgsMap := make(map[string]interface{})
	for _, arg := range namedArgs {
		if argMap, ok := arg.(map[string]interface{}); ok {
			if name, nameOk := argMap["name"].(string); nameOk {
				if value, valueOk := argMap["value"]; valueOk {
					namedArgsMap[name] = value
				}
			}
		}
	}
	
	// Use schema validator for comprehensive validation
	return inputValidator.schemaValidator.ValidateArguments(cc.capDefinition, positionalArgs, namedArgsMap)
}

// validateOutput validates output against cap definition
func (cc *CapCaller) validateOutput(response *ResponseWrapper) error {
	// For binary outputs, check type compatibility
	if response.IsBinary() {
		// For binary outputs, validate that the cap expects binary output
		if output := cc.capDefinition.GetOutput(); output != nil {
			if output.OutputType != OutputTypeBinary {
				return fmt.Errorf(
					"cap %s expects %s output but received binary data",
					cc.cap,
					output.OutputType,
				)
			}
		}
		return nil
	}

	// For text/JSON outputs, parse and validate
	text, err := response.AsString()
	if err != nil {
		return fmt.Errorf("failed to convert output to string: %w", err)
	}

	var outputValue interface{}
	if cc.isJsonCap() {
		if err := json.Unmarshal([]byte(text), &outputValue); err != nil {
			return fmt.Errorf("output is not valid JSON for cap %s: %w", cc.cap, err)
		}
	} else {
		outputValue = text
	}

	outputValidator := NewOutputValidator()
	return outputValidator.ValidateOutput(cc.capDefinition, outputValue)
}