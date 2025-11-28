package capns

import (
	"encoding/json"
)

// ArgumentType represents the type of a cap argument
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

// ArgumentValidation represents validation rules for cap arguments
type ArgumentValidation struct {
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	MinLength    *int      `json:"min_length,omitempty"`
	MaxLength    *int      `json:"max_length,omitempty"`
	Pattern      *string   `json:"pattern,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

// CapArgument represents a single argument definition for a cap
type CapArgument struct {
	Name        string               `json:"name"`
	Type        ArgumentType         `json:"type"`
	Description string               `json:"description"`
	CliFlag     string               `json:"cli_flag"`
	Position    *int                 `json:"position,omitempty"`
	Validation  *ArgumentValidation  `json:"validation,omitempty"`
	Default     interface{}          `json:"default,omitempty"`
}

// CapArguments represents the collection of arguments for a cap
type CapArguments struct {
	Required []CapArgument `json:"required,omitempty"`
	Optional []CapArgument `json:"optional,omitempty"`
}


// OutputType represents the type of output a cap returns
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

// CapOutput represents the output definition for a cap
type CapOutput struct {
	Type        OutputType           `json:"type"`
	SchemaRef   *string              `json:"schema_ref,omitempty"`
	ContentType *string              `json:"content_type,omitempty"`
	Validation  *ArgumentValidation  `json:"validation,omitempty"`
	Description string               `json:"description"`
}

// Cap represents a formal cap definition
//
// This defines the structure for formal cap definitions that include
// the cap identifier, versioning, metadata, and arguments. Caps are general-purpose
// and do not assume any specific domain like files or documents.
type Cap struct {
	// Id is the formal cap identifier with hierarchical naming
	Id *CapUrn `json:"id"`

	// Version is the cap version
	Version string `json:"version"`

	// Description is an optional description
	Description *string `json:"description,omitempty"`

	// Metadata contains optional metadata as key-value pairs
	Metadata map[string]string `json:"metadata,omitempty"`

	// Command defines the command string for this cap
	Command string `json:"command"`

	// Arguments defines the arguments for this cap
	Arguments *CapArguments `json:"arguments,omitempty"`

	// Output defines the output format for this cap
	Output *CapOutput `json:"output,omitempty"`

	// AcceptsStdin indicates whether this cap accepts input via stdin
	AcceptsStdin bool `json:"accepts_stdin,omitempty"`
}

// NewCapArgument creates a new cap argument
func NewCapArgument(name string, argType ArgumentType, description string, cliFlag string) CapArgument {
	return CapArgument{
		Name:        name,
		Type:        argType,
		Description: description,
		CliFlag:     cliFlag,
	}
}

// NewCapArgumentWithCliFlag creates an argument with CLI flag (deprecated - use NewCapArgument)
func NewCapArgumentWithCliFlag(name string, argType ArgumentType, description string, cliFlag string) CapArgument {
	return NewCapArgument(name, argType, description, cliFlag)
}

// NewCapArgumentWithPosition creates an argument with position
func NewCapArgumentWithPosition(name string, argType ArgumentType, description string, cliFlag string, position int) CapArgument {
	return CapArgument{
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

// NewCapOutput creates a new output definition
func NewCapOutput(outputType OutputType, description string) *CapOutput {
	return &CapOutput{
		Type:        outputType,
		Description: description,
	}
}

// NewCapOutputWithContentType creates output with content type
func NewCapOutputWithContentType(outputType OutputType, description string, contentType string) *CapOutput {
	return &CapOutput{
		Type:        outputType,
		Description: description,
		ContentType: &contentType,
	}
}

// NewCapOutputWithSchema creates output with schema reference
func NewCapOutputWithSchema(outputType OutputType, description string, schemaRef string) *CapOutput {
	return &CapOutput{
		Type:        outputType,
		Description: description,
		SchemaRef:   &schemaRef,
	}
}

// NewCapArguments creates a new cap arguments collection
func NewCapArguments() *CapArguments {
	return &CapArguments{
		Required: []CapArgument{},
		Optional: []CapArgument{},
	}
}

// IsEmpty checks if the cap arguments collection is empty
func (ca *CapArguments) IsEmpty() bool {
	return len(ca.Required) == 0 && len(ca.Optional) == 0
}

// AddRequired adds a required argument
func (ca *CapArguments) AddRequired(arg CapArgument) {
	ca.Required = append(ca.Required, arg)
}

// AddOptional adds an optional argument
func (ca *CapArguments) AddOptional(arg CapArgument) {
	ca.Optional = append(ca.Optional, arg)
}

// FindArgument finds an argument by name
func (ca *CapArguments) FindArgument(name string) *CapArgument {
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
func (ca *CapArguments) GetPositionalArgs() []CapArgument {
	var args []CapArgument
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
func (ca *CapArguments) GetFlagArgs() []CapArgument {
	var args []CapArgument
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

// NewCap creates a new cap
func NewCap(id *CapUrn, version string, command string) *Cap {
	return &Cap{
		Id:       id,
		Version:  version,
		Command:  command,
		Metadata: make(map[string]string),
		Arguments: NewCapArguments(),
	}
}

// NewCapWithDescription creates a new cap with description
func NewCapWithDescription(id *CapUrn, version string, command string, description string) *Cap {
	return &Cap{
		Id:          id,
		Version:     version,
		Command:     command,
		Description: &description,
		Metadata:    make(map[string]string),
		Arguments:   NewCapArguments(),
	}
}

// NewCapWithMetadata creates a new cap with metadata
func NewCapWithMetadata(id *CapUrn, version string, command string, metadata map[string]string) *Cap {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Cap{
		Id:        id,
		Version:   version,
		Command:   command,
		Metadata:  metadata,
		Arguments: NewCapArguments(),
	}
}

// NewCapWithDescriptionAndMetadata creates a new cap with description and metadata
func NewCapWithDescriptionAndMetadata(id *CapUrn, version string, description string, metadata map[string]string) *Cap {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Cap{
		Id:          id,
		Version:     version,
		Description: &description,
		Metadata:    metadata,
		Arguments:   NewCapArguments(),
	}
}

// MatchesRequest checks if this cap matches a request string
func (c *Cap) MatchesRequest(request string) bool {
	requestId, err := NewCapUrnFromString(request)
	if err != nil {
		return false
	}
	return c.Id.CanHandle(requestId)
}

// CanHandleRequest checks if this cap can handle a request
func (c *Cap) CanHandleRequest(request *CapUrn) bool {
	return c.Id.CanHandle(request)
}

// IsMoreSpecificThan checks if this cap is more specific than another
func (c *Cap) IsMoreSpecificThan(other *Cap) bool {
	if other == nil {
		return true
	}
	return c.Id.IsMoreSpecificThan(other.Id)
}

// GetMetadata gets a metadata value by key
func (c *Cap) GetMetadata(key string) (string, bool) {
	if c.Metadata == nil {
		return "", false
	}
	value, exists := c.Metadata[key]
	return value, exists
}

// SetMetadata sets a metadata value
func (c *Cap) SetMetadata(key, value string) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	c.Metadata[key] = value
}

// RemoveMetadata removes a metadata value
func (c *Cap) RemoveMetadata(key string) bool {
	if c.Metadata == nil {
		return false
	}
	_, exists := c.Metadata[key]
	if exists {
		delete(c.Metadata, key)
	}
	return exists
}

// HasMetadata checks if this cap has specific metadata
func (c *Cap) HasMetadata(key string) bool {
	if c.Metadata == nil {
		return false
	}
	_, exists := c.Metadata[key]
	return exists
}

// GetCommand gets the command
func (c *Cap) GetCommand() string {
	return c.Command
}

// SetCommand sets the command
func (c *Cap) SetCommand(command string) {
	c.Command = command
}

// GetArguments gets the arguments
func (c *Cap) GetArguments() *CapArguments {
	return c.Arguments
}

// SetArguments sets the arguments
func (c *Cap) SetArguments(arguments *CapArguments) {
	c.Arguments = arguments
}

// AddRequiredArgument adds a required argument
func (c *Cap) AddRequiredArgument(arg CapArgument) {
	if c.Arguments == nil {
		c.Arguments = NewCapArguments()
	}
	c.Arguments.AddRequired(arg)
}

// AddOptionalArgument adds an optional argument
func (c *Cap) AddOptionalArgument(arg CapArgument) {
	if c.Arguments == nil {
		c.Arguments = NewCapArguments()
	}
	c.Arguments.AddOptional(arg)
}

// GetOutput gets the output definition if defined
func (c *Cap) GetOutput() *CapOutput {
	return c.Output
}

// SetOutput sets the output definition
func (c *Cap) SetOutput(output *CapOutput) {
	c.Output = output
}

// IdString gets the cap identifier as a string
func (c *Cap) IdString() string {
	return c.Id.ToString()
}

// Equals checks if this cap is equal to another
func (c *Cap) Equals(other *Cap) bool {
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
func (c *Cap) MarshalJSON() ([]byte, error) {
	type CapAlias Cap
	return json.Marshal((*CapAlias)(c))
}

// UnmarshalJSON implements custom JSON unmarshaling
func (c *Cap) UnmarshalJSON(data []byte) error {
	type CapAlias Cap
	aux := (*CapAlias)(c)
	return json.Unmarshal(data, aux)
}