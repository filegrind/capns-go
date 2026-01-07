package capns

import (
	"encoding/json"
	"fmt"
)

// ArgumentValidation represents validation rules for cap arguments
type ArgumentValidation struct {
	Min           *float64 `json:"min,omitempty"`
	Max           *float64 `json:"max,omitempty"`
	MinLength     *int     `json:"min_length,omitempty"`
	MaxLength     *int     `json:"max_length,omitempty"`
	Pattern       *string  `json:"pattern,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
}

// CapArgument represents a single argument definition for a cap
type CapArgument struct {
	Name           string              `json:"name"`
	MediaSpec      string              `json:"media_spec"` // Spec ID, e.g., "capns:ms:str.v1"
	ArgDescription string              `json:"arg_description"`
	CliFlag        string              `json:"cli_flag"`
	Position       *int                `json:"position,omitempty"`
	Validation     *ArgumentValidation `json:"validation,omitempty"`
	DefaultValue   interface{}         `json:"default_value,omitempty"`
	Metadata       interface{}         `json:"metadata,omitempty"`
}

// Resolve resolves the argument's media spec to a ResolvedMediaSpec
func (ca *CapArgument) Resolve(mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	return ResolveSpecID(ca.MediaSpec, mediaSpecs)
}

// IsBinary checks if this argument expects binary data
func (ca *CapArgument) IsBinary(mediaSpecs map[string]MediaSpecDef) bool {
	resolved, err := ca.Resolve(mediaSpecs)
	if err != nil {
		return false
	}
	return resolved.IsBinary()
}

// IsJSON checks if this argument expects JSON data
func (ca *CapArgument) IsJSON(mediaSpecs map[string]MediaSpecDef) bool {
	resolved, err := ca.Resolve(mediaSpecs)
	if err != nil {
		return false
	}
	return resolved.IsJSON()
}

// GetMediaType returns the resolved media type for this argument
func (ca *CapArgument) GetMediaType(mediaSpecs map[string]MediaSpecDef) string {
	resolved, err := ca.Resolve(mediaSpecs)
	if err != nil {
		return ""
	}
	return resolved.MediaType
}

// GetProfileURI returns the resolved profile URI for this argument
func (ca *CapArgument) GetProfileURI(mediaSpecs map[string]MediaSpecDef) string {
	resolved, err := ca.Resolve(mediaSpecs)
	if err != nil {
		return ""
	}
	return resolved.ProfileURI
}

// GetSchema returns the resolved schema for this argument
func (ca *CapArgument) GetSchema(mediaSpecs map[string]MediaSpecDef) interface{} {
	resolved, err := ca.Resolve(mediaSpecs)
	if err != nil {
		return nil
	}
	return resolved.Schema
}

// CapArguments represents the collection of arguments for a cap
type CapArguments struct {
	Required []CapArgument `json:"required,omitempty"`
	Optional []CapArgument `json:"optional,omitempty"`
}

// CapOutput represents the output definition for a cap
type CapOutput struct {
	MediaSpec         string              `json:"media_spec"` // Spec ID, e.g., "capns:ms:obj.v1"
	OutputDescription string              `json:"output_description"`
	Validation        *ArgumentValidation `json:"validation,omitempty"`
	Metadata          interface{}         `json:"metadata,omitempty"`
}

// Resolve resolves the output's media spec to a ResolvedMediaSpec
func (co *CapOutput) Resolve(mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	return ResolveSpecID(co.MediaSpec, mediaSpecs)
}

// IsBinary checks if this output produces binary data
func (co *CapOutput) IsBinary(mediaSpecs map[string]MediaSpecDef) bool {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return false
	}
	return resolved.IsBinary()
}

// IsJSON checks if this output produces JSON data
func (co *CapOutput) IsJSON(mediaSpecs map[string]MediaSpecDef) bool {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return false
	}
	return resolved.IsJSON()
}

// GetMediaType returns the resolved media type for this output
func (co *CapOutput) GetMediaType(mediaSpecs map[string]MediaSpecDef) string {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return ""
	}
	return resolved.MediaType
}

// GetProfileURI returns the resolved profile URI for this output
func (co *CapOutput) GetProfileURI(mediaSpecs map[string]MediaSpecDef) string {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return ""
	}
	return resolved.ProfileURI
}

// GetSchema returns the resolved schema for this output
func (co *CapOutput) GetSchema(mediaSpecs map[string]MediaSpecDef) interface{} {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return nil
	}
	return resolved.Schema
}

// Cap represents a formal cap definition
//
// This defines the structure for formal cap definitions that include
// the cap URN, metadata, and arguments. Caps are general-purpose
// and do not assume any specific domain like files or documents.
type Cap struct {
	// Urn is the formal cap URN with hierarchical naming
	Urn *CapUrn `json:"urn"`

	// Title is the human-readable title of the capability (required)
	Title string `json:"title"`

	// CapDescription is an optional description
	CapDescription *string `json:"cap_description,omitempty"`

	// Metadata contains optional metadata as key-value pairs
	Metadata map[string]string `json:"metadata,omitempty"`

	// Command defines the command string for this cap
	Command string `json:"command"`

	// MediaSpecs is the spec ID resolution table
	// Maps spec IDs to their definitions (string or object form)
	// Arguments and output media_spec fields reference entries here
	MediaSpecs map[string]MediaSpecDef `json:"media_specs,omitempty"`

	// Arguments defines the arguments for this cap
	Arguments *CapArguments `json:"arguments,omitempty"`

	// Output defines the output format for this cap
	Output *CapOutput `json:"output,omitempty"`

	// AcceptsStdin indicates whether this cap accepts input via stdin
	AcceptsStdin bool `json:"accepts_stdin,omitempty"`

	// MetadataJSON contains arbitrary metadata as JSON object
	MetadataJSON interface{} `json:"metadata_json,omitempty"`
}

// NewCapArgument creates a new cap argument with a spec ID
func NewCapArgument(name string, mediaSpec string, description string, cliFlag string) CapArgument {
	return CapArgument{
		Name:           name,
		MediaSpec:      mediaSpec,
		ArgDescription: description,
		CliFlag:        cliFlag,
	}
}

// NewCapArgumentWithPosition creates an argument with position
func NewCapArgumentWithPosition(name string, mediaSpec string, description string, cliFlag string, position int) CapArgument {
	return CapArgument{
		Name:           name,
		MediaSpec:      mediaSpec,
		ArgDescription: description,
		CliFlag:        cliFlag,
		Position:       &position,
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

// NewCapOutput creates a new output definition with a spec ID
func NewCapOutput(mediaSpec string, description string) *CapOutput {
	return &CapOutput{
		MediaSpec:         mediaSpec,
		OutputDescription: description,
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
func NewCap(urn *CapUrn, title string, command string) *Cap {
	return &Cap{
		Urn:        urn,
		Title:      title,
		Command:    command,
		Metadata:   make(map[string]string),
		MediaSpecs: make(map[string]MediaSpecDef),
		Arguments:  NewCapArguments(),
	}
}

// NewCapWithDescription creates a new cap with description
func NewCapWithDescription(urn *CapUrn, title string, command string, description string) *Cap {
	return &Cap{
		Urn:            urn,
		Title:          title,
		Command:        command,
		CapDescription: &description,
		Metadata:       make(map[string]string),
		MediaSpecs:     make(map[string]MediaSpecDef),
		Arguments:      NewCapArguments(),
	}
}

// NewCapWithMetadata creates a new cap with metadata
func NewCapWithMetadata(urn *CapUrn, title string, command string, metadata map[string]string) *Cap {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Cap{
		Urn:        urn,
		Title:      title,
		Command:    command,
		Metadata:   metadata,
		MediaSpecs: make(map[string]MediaSpecDef),
		Arguments:  NewCapArguments(),
	}
}

// NewCapWithDescriptionAndMetadata creates a new cap with description and metadata
func NewCapWithDescriptionAndMetadata(urn *CapUrn, title string, command string, description string, metadata map[string]string) *Cap {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Cap{
		Urn:            urn,
		Title:          title,
		Command:        command,
		CapDescription: &description,
		Metadata:       metadata,
		MediaSpecs:     make(map[string]MediaSpecDef),
		Arguments:      NewCapArguments(),
	}
}

// GetMediaSpecs returns the media specs table
func (c *Cap) GetMediaSpecs() map[string]MediaSpecDef {
	if c.MediaSpecs == nil {
		c.MediaSpecs = make(map[string]MediaSpecDef)
	}
	return c.MediaSpecs
}

// SetMediaSpecs sets the media specs table
func (c *Cap) SetMediaSpecs(mediaSpecs map[string]MediaSpecDef) {
	c.MediaSpecs = mediaSpecs
}

// AddMediaSpec adds a media spec to the table
func (c *Cap) AddMediaSpec(specID string, def MediaSpecDef) {
	if c.MediaSpecs == nil {
		c.MediaSpecs = make(map[string]MediaSpecDef)
	}
	c.MediaSpecs[specID] = def
}

// ResolveSpecID resolves a spec ID using this cap's media_specs table
func (c *Cap) ResolveSpecID(specID string) (*ResolvedMediaSpec, error) {
	return ResolveSpecID(specID, c.GetMediaSpecs())
}

// MatchesRequest checks if this cap matches a request string
func (c *Cap) MatchesRequest(request string) bool {
	requestId, err := NewCapUrnFromString(request)
	if err != nil {
		return false
	}
	return c.Urn.CanHandle(requestId)
}

// CanHandleRequest checks if this cap can handle a request
func (c *Cap) CanHandleRequest(request *CapUrn) bool {
	return c.Urn.CanHandle(request)
}

// IsMoreSpecificThan checks if this cap is more specific than another
func (c *Cap) IsMoreSpecificThan(other *Cap) bool {
	if other == nil {
		return true
	}
	return c.Urn.IsMoreSpecificThan(other.Urn)
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

// GetTitle gets the title
func (c *Cap) GetTitle() string {
	return c.Title
}

// SetTitle sets the title
func (c *Cap) SetTitle(title string) {
	c.Title = title
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

// GetMetadataJSON gets the metadata JSON
func (c *Cap) GetMetadataJSON() interface{} {
	return c.MetadataJSON
}

// SetMetadataJSON sets the metadata JSON
func (c *Cap) SetMetadataJSON(metadata interface{}) {
	c.MetadataJSON = metadata
}

// ClearMetadataJSON clears the metadata JSON
func (c *Cap) ClearMetadataJSON() {
	c.MetadataJSON = nil
}

// GetMetadata gets the metadata JSON for CapArgument
func (ca *CapArgument) GetMetadata() interface{} {
	return ca.Metadata
}

// SetMetadata sets the metadata JSON for CapArgument
func (ca *CapArgument) SetMetadata(metadata interface{}) {
	ca.Metadata = metadata
}

// ClearMetadata clears the metadata JSON for CapArgument
func (ca *CapArgument) ClearMetadata() {
	ca.Metadata = nil
}

// GetMetadata gets the metadata JSON for CapOutput
func (co *CapOutput) GetMetadata() interface{} {
	return co.Metadata
}

// SetMetadata sets the metadata JSON for CapOutput
func (co *CapOutput) SetMetadata(metadata interface{}) {
	co.Metadata = metadata
}

// ClearMetadata clears the metadata JSON for CapOutput
func (co *CapOutput) ClearMetadata() {
	co.Metadata = nil
}

// UrnString gets the cap URN as a string
func (c *Cap) UrnString() string {
	return c.Urn.ToString()
}

// Equals checks if this cap is equal to another
func (c *Cap) Equals(other *Cap) bool {
	if other == nil {
		return false
	}

	if !c.Urn.Equals(other.Urn) {
		return false
	}

	if c.Title != other.Title {
		return false
	}

	if c.Command != other.Command {
		return false
	}

	if (c.CapDescription == nil) != (other.CapDescription == nil) {
		return false
	}

	if c.CapDescription != nil && *c.CapDescription != *other.CapDescription {
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
	capData := map[string]interface{}{
		"urn": map[string]interface{}{
			"tags": c.Urn.tags,
		},
		"title":   c.Title,
		"command": c.Command,
	}

	if c.CapDescription != nil {
		capData["cap_description"] = *c.CapDescription
	}

	if len(c.Metadata) > 0 {
		capData["metadata"] = c.Metadata
	}

	if len(c.MediaSpecs) > 0 {
		capData["media_specs"] = c.MediaSpecs
	}

	if c.Arguments != nil && !c.Arguments.IsEmpty() {
		capData["arguments"] = c.Arguments
	}

	if c.Output != nil {
		capData["output"] = c.Output
	}

	if c.AcceptsStdin {
		capData["accepts_stdin"] = c.AcceptsStdin
	}

	if c.MetadataJSON != nil {
		capData["metadata_json"] = c.MetadataJSON
	}

	return json.Marshal(capData)
}

// UnmarshalJSON implements custom JSON unmarshaling
func (c *Cap) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Handle urn field - can be string or object with tags
	urnField, ok := raw["urn"]
	if !ok {
		return fmt.Errorf("missing required field 'urn'")
	}

	var urn *CapUrn
	var err error
	switch v := urnField.(type) {
	case string:
		urn, err = NewCapUrnFromString(v)
		if err != nil {
			return fmt.Errorf("failed to parse URN string: %v", err)
		}
	case map[string]interface{}:
		tagsField, ok := v["tags"]
		if !ok {
			return fmt.Errorf("URN object missing 'tags' field")
		}
		tagsMap, ok := tagsField.(map[string]interface{})
		if !ok {
			return fmt.Errorf("URN tags field must be an object")
		}

		tags := make(map[string]string)
		for k, v := range tagsMap {
			if s, ok := v.(string); ok {
				tags[k] = s
			}
		}
		urn = NewCapUrnFromTags(tags)
	default:
		return fmt.Errorf("URN field must be string or object")
	}

	c.Urn = urn

	// Handle required fields
	if title, ok := raw["title"].(string); ok {
		c.Title = title
	} else {
		return fmt.Errorf("missing required field 'title'")
	}

	if command, ok := raw["command"].(string); ok {
		c.Command = command
	} else {
		return fmt.Errorf("missing required field 'command'")
	}

	if desc, ok := raw["cap_description"].(string); ok {
		c.CapDescription = &desc
	}

	if metadata, ok := raw["metadata"].(map[string]interface{}); ok {
		c.Metadata = make(map[string]string)
		for k, v := range metadata {
			if s, ok := v.(string); ok {
				c.Metadata[k] = s
			}
		}
	}

	// Handle media_specs
	if mediaSpecsRaw, ok := raw["media_specs"]; ok {
		mediaSpecsBytes, _ := json.Marshal(mediaSpecsRaw)
		var mediaSpecs map[string]MediaSpecDef
		if err := json.Unmarshal(mediaSpecsBytes, &mediaSpecs); err == nil {
			c.MediaSpecs = mediaSpecs
		}
	}

	if acceptsStdin, ok := raw["accepts_stdin"].(bool); ok {
		c.AcceptsStdin = acceptsStdin
	}

	// Handle arguments and output if present
	if args, ok := raw["arguments"]; ok {
		argsBytes, _ := json.Marshal(args)
		var arguments CapArguments
		if err := json.Unmarshal(argsBytes, &arguments); err == nil {
			c.Arguments = &arguments
		}
	}

	if output, ok := raw["output"]; ok {
		outputBytes, _ := json.Marshal(output)
		var capOutput CapOutput
		if err := json.Unmarshal(outputBytes, &capOutput); err == nil {
			c.Output = &capOutput
		}
	}

	if metadataJSON, ok := raw["metadata_json"]; ok {
		c.MetadataJSON = metadataJSON
	}

	return nil
}
