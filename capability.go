package capdef

import (
	"encoding/json"
)

// ArgumentType represents the type of a capability argument
type ArgumentType string

const (
	ArgumentTypeString  ArgumentType = "string"
	ArgumentTypeInteger ArgumentType = "integer"
	ArgumentTypeNumber  ArgumentType = "number"
	ArgumentTypeBoolean ArgumentType = "boolean"
	ArgumentTypeArray   ArgumentType = "array"
	ArgumentTypeObject  ArgumentType = "object"
	ArgumentTypeBinary  ArgumentType = "binary"
)

// ArgumentValidation represents validation rules for capability arguments
type ArgumentValidation struct {
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	MinLength    *int      `json:"min_length,omitempty"`
	MaxLength    *int      `json:"max_length,omitempty"`
	Pattern      *string   `json:"pattern,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

// CapabilityArgument represents a single argument definition for a capability
type CapabilityArgument struct {
	Name        string               `json:"name"`
	Type        ArgumentType         `json:"type"`
	Description string               `json:"description"`
	CliFlag     string               `json:"cli_flag"`
	Position    *int                 `json:"position,omitempty"`
	Validation  *ArgumentValidation  `json:"validation,omitempty"`
	Default     interface{}          `json:"default,omitempty"`
}

// CapabilityArguments represents the collection of arguments for a capability
type CapabilityArguments struct {
	Required []CapabilityArgument `json:"required,omitempty"`
	Optional []CapabilityArgument `json:"optional,omitempty"`
}


// OutputType represents the type of output a capability returns
type OutputType string

const (
	OutputTypeString  OutputType = "string"
	OutputTypeInteger OutputType = "integer"
	OutputTypeNumber  OutputType = "number"
	OutputTypeBoolean OutputType = "boolean"
	OutputTypeArray   OutputType = "array"
	OutputTypeObject  OutputType = "object"
	OutputTypeBinary  OutputType = "binary"
)

// CapabilityOutput represents the output definition for a capability
type CapabilityOutput struct {
	Type        OutputType           `json:"type"`
	SchemaRef   *string              `json:"schema_ref,omitempty"`
	ContentType *string              `json:"content_type,omitempty"`
	Validation  *ArgumentValidation  `json:"validation,omitempty"`
	Description string               `json:"description"`
}

// Capability represents a formal capability definition
//
// This defines the structure for formal capability definitions that include
// the capability identifier, versioning, metadata, and arguments. Capabilities are general-purpose
// and do not assume any specific domain like files or documents.
type Capability struct {
	// Id is the formal capability identifier with hierarchical naming
	Id *CapabilityKey `json:"id"`

	// Version is the capability version
	Version string `json:"version"`

	// Description is an optional description
	Description *string `json:"description,omitempty"`

	// Metadata contains optional metadata as key-value pairs
	Metadata map[string]string `json:"metadata,omitempty"`

	// Command defines the command string for this capability
	Command string `json:"command"`

	// Arguments defines the arguments for this capability
	Arguments *CapabilityArguments `json:"arguments,omitempty"`

	// Output defines the output format for this capability
	Output *CapabilityOutput `json:"output,omitempty"`
}

// NewCapabilityArgument creates a new capability argument
func NewCapabilityArgument(name string, argType ArgumentType, description string, cliFlag string) CapabilityArgument {
	return CapabilityArgument{
		Name:        name,
		Type:        argType,
		Description: description,
		CliFlag:     cliFlag,
	}
}

// NewCapabilityArgumentWithCliFlag creates an argument with CLI flag (deprecated - use NewCapabilityArgument)
func NewCapabilityArgumentWithCliFlag(name string, argType ArgumentType, description string, cliFlag string) CapabilityArgument {
	return NewCapabilityArgument(name, argType, description, cliFlag)
}

// NewCapabilityArgumentWithPosition creates an argument with position
func NewCapabilityArgumentWithPosition(name string, argType ArgumentType, description string, cliFlag string, position int) CapabilityArgument {
	return CapabilityArgument{
		Name:        name,
		Type:        argType,
		Description: description,
		CliFlag:     cliFlag,
		Position:    &position,
	}
}

// NewArgumentValidationNumericRange creates validation with numeric constraints
func NewArgumentValidationNumericRange(min, max *float64) *ArgumentValidation {
	return &ArgumentValidation{
		Min: min,
		Max: max,
	}
}

// NewArgumentValidationStringLength creates validation with string length constraints
func NewArgumentValidationStringLength(minLength, maxLength *int) *ArgumentValidation {
	return &ArgumentValidation{
		MinLength: minLength,
		MaxLength: maxLength,
	}
}

// NewArgumentValidationPattern creates validation with pattern
func NewArgumentValidationPattern(pattern string) *ArgumentValidation {
	return &ArgumentValidation{
		Pattern: &pattern,
	}
}

// NewArgumentValidationAllowedValues creates validation with allowed values
func NewArgumentValidationAllowedValues(values []string) *ArgumentValidation {
	return &ArgumentValidation{
		AllowedValues: values,
	}
}

// NewCapabilityOutput creates a new output definition
func NewCapabilityOutput(outputType OutputType, description string) *CapabilityOutput {
	return &CapabilityOutput{
		Type:        outputType,
		Description: description,
	}
}

// NewCapabilityOutputWithContentType creates output with content type
func NewCapabilityOutputWithContentType(outputType OutputType, description string, contentType string) *CapabilityOutput {
	return &CapabilityOutput{
		Type:        outputType,
		Description: description,
		ContentType: &contentType,
	}
}

// NewCapabilityOutputWithSchema creates output with schema reference
func NewCapabilityOutputWithSchema(outputType OutputType, description string, schemaRef string) *CapabilityOutput {
	return &CapabilityOutput{
		Type:        outputType,
		Description: description,
		SchemaRef:   &schemaRef,
	}
}

// NewCapabilityArguments creates a new capability arguments collection
func NewCapabilityArguments() *CapabilityArguments {
	return &CapabilityArguments{
		Required: []CapabilityArgument{},
		Optional: []CapabilityArgument{},
	}
}

// IsEmpty checks if the capability arguments collection is empty
func (ca *CapabilityArguments) IsEmpty() bool {
	return len(ca.Required) == 0 && len(ca.Optional) == 0
}

// AddRequired adds a required argument
func (ca *CapabilityArguments) AddRequired(arg CapabilityArgument) {
	ca.Required = append(ca.Required, arg)
}

// AddOptional adds an optional argument
func (ca *CapabilityArguments) AddOptional(arg CapabilityArgument) {
	ca.Optional = append(ca.Optional, arg)
}

// FindArgument finds an argument by name
func (ca *CapabilityArguments) FindArgument(name string) *CapabilityArgument {
	for i := range ca.Required {
		if ca.Required[i].Name == name {
			return &ca.Required[i]
		}
	}
	for i := range ca.Optional {
		if ca.Optional[i].Name == name {
			return &ca.Optional[i]
		}
	}
	return nil
}

// GetPositionalArgs returns arguments sorted by position
func (ca *CapabilityArguments) GetPositionalArgs() []CapabilityArgument {
	var args []CapabilityArgument
	for _, arg := range ca.Required {
		if arg.Position != nil {
			args = append(args, arg)
		}
	}
	for _, arg := range ca.Optional {
		if arg.Position != nil {
			args = append(args, arg)
		}
	}
	// Sort by position
	for i := 0; i < len(args)-1; i++ {
		for j := i + 1; j < len(args); j++ {
			if *args[i].Position > *args[j].Position {
				args[i], args[j] = args[j], args[i]
			}
		}
	}
	return args
}

// GetFlagArgs returns arguments that have CLI flags
func (ca *CapabilityArguments) GetFlagArgs() []CapabilityArgument {
	var args []CapabilityArgument
	for _, arg := range ca.Required {
		if arg.CliFlag != "" {
			args = append(args, arg)
		}
	}
	for _, arg := range ca.Optional {
		if arg.CliFlag != "" {
			args = append(args, arg)
		}
	}
	return args
}

// NewCapability creates a new capability
func NewCapability(id *CapabilityKey, version string, command string) *Capability {
	return &Capability{
		Id:       id,
		Version:  version,
		Command:  command,
		Metadata: make(map[string]string),
		Arguments: NewCapabilityArguments(),
	}
}

// NewCapabilityWithDescription creates a new capability with description
func NewCapabilityWithDescription(id *CapabilityKey, version string, command string, description string) *Capability {
	return &Capability{
		Id:          id,
		Version:     version,
		Command:     command,
		Description: &description,
		Metadata:    make(map[string]string),
		Arguments:   NewCapabilityArguments(),
	}
}

// NewCapabilityWithMetadata creates a new capability with metadata
func NewCapabilityWithMetadata(id *CapabilityKey, version string, command string, metadata map[string]string) *Capability {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Capability{
		Id:        id,
		Version:   version,
		Command:   command,
		Metadata:  metadata,
		Arguments: NewCapabilityArguments(),
	}
}

// NewCapabilityWithDescriptionAndMetadata creates a new capability with description and metadata
func NewCapabilityWithDescriptionAndMetadata(id *CapabilityKey, version string, description string, metadata map[string]string) *Capability {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Capability{
		Id:          id,
		Version:     version,
		Description: &description,
		Metadata:    metadata,
		Arguments:   NewCapabilityArguments(),
	}
}

// MatchesRequest checks if this capability matches a request string
func (c *Capability) MatchesRequest(request string) bool {
	requestId, err := NewCapabilityKeyFromString(request)
	if err != nil {
		return false
	}
	return c.Id.CanHandle(requestId)
}

// CanHandleRequest checks if this capability can handle a request
func (c *Capability) CanHandleRequest(request *CapabilityKey) bool {
	return c.Id.CanHandle(request)
}

// IsMoreSpecificThan checks if this capability is more specific than another
func (c *Capability) IsMoreSpecificThan(other *Capability) bool {
	if other == nil {
		return true
	}
	return c.Id.IsMoreSpecificThan(other.Id)
}

// GetMetadata gets a metadata value by key
func (c *Capability) GetMetadata(key string) (string, bool) {
	if c.Metadata == nil {
		return "", false
	}
	value, exists := c.Metadata[key]
	return value, exists
}

// SetMetadata sets a metadata value
func (c *Capability) SetMetadata(key, value string) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	c.Metadata[key] = value
}

// RemoveMetadata removes a metadata value
func (c *Capability) RemoveMetadata(key string) bool {
	if c.Metadata == nil {
		return false
	}
	_, exists := c.Metadata[key]
	if exists {
		delete(c.Metadata, key)
	}
	return exists
}

// HasMetadata checks if this capability has specific metadata
func (c *Capability) HasMetadata(key string) bool {
	if c.Metadata == nil {
		return false
	}
	_, exists := c.Metadata[key]
	return exists
}

// GetCommand gets the command
func (c *Capability) GetCommand() string {
	return c.Command
}

// SetCommand sets the command
func (c *Capability) SetCommand(command string) {
	c.Command = command
}

// GetArguments gets the arguments
func (c *Capability) GetArguments() *CapabilityArguments {
	return c.Arguments
}

// SetArguments sets the arguments
func (c *Capability) SetArguments(arguments *CapabilityArguments) {
	c.Arguments = arguments
}

// AddRequiredArgument adds a required argument
func (c *Capability) AddRequiredArgument(arg CapabilityArgument) {
	if c.Arguments == nil {
		c.Arguments = NewCapabilityArguments()
	}
	c.Arguments.AddRequired(arg)
}

// AddOptionalArgument adds an optional argument
func (c *Capability) AddOptionalArgument(arg CapabilityArgument) {
	if c.Arguments == nil {
		c.Arguments = NewCapabilityArguments()
	}
	c.Arguments.AddOptional(arg)
}

// GetOutput gets the output definition if defined
func (c *Capability) GetOutput() *CapabilityOutput {
	return c.Output
}

// SetOutput sets the output definition
func (c *Capability) SetOutput(output *CapabilityOutput) {
	c.Output = output
}

// IdString gets the capability identifier as a string
func (c *Capability) IdString() string {
	return c.Id.ToString()
}

// Equals checks if this capability is equal to another
func (c *Capability) Equals(other *Capability) bool {
	if other == nil {
		return false
	}

	if !c.Id.Equals(other.Id) {
		return false
	}

	if c.Version != other.Version {
		return false
	}

	if (c.Description == nil) != (other.Description == nil) {
		return false
	}

	if c.Description != nil && *c.Description != *other.Description {
		return false
	}

	if len(c.Metadata) != len(other.Metadata) {
		return false
	}

	for key, value := range c.Metadata {
		if otherValue, exists := other.Metadata[key]; !exists || value != otherValue {
			return false
		}
	}

	return true
}

// MarshalJSON implements custom JSON marshaling
func (c *Capability) MarshalJSON() ([]byte, error) {
	type CapabilityAlias Capability
	return json.Marshal((*CapabilityAlias)(c))
}

// UnmarshalJSON implements custom JSON unmarshaling
func (c *Capability) UnmarshalJSON(data []byte) error {
	type CapabilityAlias Capability
	aux := (*CapabilityAlias)(c)
	return json.Unmarshal(data, aux)
}