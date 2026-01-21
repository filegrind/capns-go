package capns

import (
	"encoding/json"
	"fmt"
	"reflect"
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

// ArgSource specifies how an argument can be provided
type ArgSource struct {
	Stdin    *string `json:"stdin,omitempty"`
	Position *int    `json:"position,omitempty"`
	CliFlag  *string `json:"cli_flag,omitempty"`
}

// GetType returns the type of this source
func (s *ArgSource) GetType() string {
	if s.Stdin != nil {
		return "stdin"
	}
	if s.Position != nil {
		return "position"
	}
	if s.CliFlag != nil {
		return "cli_flag"
	}
	return ""
}

// IsStdin returns true if this is a stdin source
func (s *ArgSource) IsStdin() bool {
	return s.Stdin != nil
}

// IsPosition returns true if this is a position source
func (s *ArgSource) IsPosition() bool {
	return s.Position != nil
}

// IsCliFlag returns true if this is a cli_flag source
func (s *ArgSource) IsCliFlag() bool {
	return s.CliFlag != nil
}

// CapArg represents an argument definition with sources
type CapArg struct {
	MediaUrn       string              `json:"media_urn"`
	Required       bool                `json:"required"`
	Sources        []ArgSource         `json:"sources"`
	ArgDescription string              `json:"arg_description,omitempty"`
	Validation     *ArgumentValidation `json:"validation,omitempty"`
	DefaultValue   any                 `json:"default_value,omitempty"`
	Metadata       any                 `json:"metadata,omitempty"`
}

// NewCapArg creates a new cap argument
func NewCapArg(mediaUrn string, required bool, sources []ArgSource) CapArg {
	return CapArg{
		MediaUrn: mediaUrn,
		Required: required,
		Sources:  sources,
	}
}

// NewCapArgWithDescription creates a new cap argument with description
func NewCapArgWithDescription(mediaUrn string, required bool, sources []ArgSource, description string) CapArg {
	return CapArg{
		MediaUrn:       mediaUrn,
		Required:       required,
		Sources:        sources,
		ArgDescription: description,
	}
}

// HasStdinSource returns true if this argument has a stdin source
func (a *CapArg) HasStdinSource() bool {
	for _, s := range a.Sources {
		if s.IsStdin() {
			return true
		}
	}
	return false
}

// GetStdinMediaUrn returns the stdin media URN if present
func (a *CapArg) GetStdinMediaUrn() *string {
	for _, s := range a.Sources {
		if s.Stdin != nil {
			return s.Stdin
		}
	}
	return nil
}

// HasPositionSource returns true if this argument has a position source
func (a *CapArg) HasPositionSource() bool {
	for _, s := range a.Sources {
		if s.IsPosition() {
			return true
		}
	}
	return false
}

// GetPosition returns the position if present
func (a *CapArg) GetPosition() *int {
	for _, s := range a.Sources {
		if s.Position != nil {
			return s.Position
		}
	}
	return nil
}

// HasCliFlagSource returns true if this argument has a cli_flag source
func (a *CapArg) HasCliFlagSource() bool {
	for _, s := range a.Sources {
		if s.IsCliFlag() {
			return true
		}
	}
	return false
}

// GetCliFlag returns the cli_flag if present
func (a *CapArg) GetCliFlag() *string {
	for _, s := range a.Sources {
		if s.CliFlag != nil {
			return s.CliFlag
		}
	}
	return nil
}

// Resolve resolves the argument's media URN to a ResolvedMediaSpec
func (a *CapArg) Resolve(mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	return ResolveMediaUrn(a.MediaUrn, mediaSpecs)
}

// IsBinary checks if this argument expects binary data.
func (a *CapArg) IsBinary(mediaSpecs map[string]MediaSpecDef) (bool, error) {
	resolved, err := a.Resolve(mediaSpecs)
	if err != nil {
		return false, fmt.Errorf("failed to resolve argument media_urn '%s': %w", a.MediaUrn, err)
	}
	return resolved.IsBinary(), nil
}

// IsJSON checks if this argument expects JSON data.
func (a *CapArg) IsJSON(mediaSpecs map[string]MediaSpecDef) (bool, error) {
	resolved, err := a.Resolve(mediaSpecs)
	if err != nil {
		return false, fmt.Errorf("failed to resolve argument media_urn '%s': %w", a.MediaUrn, err)
	}
	return resolved.IsJSON(), nil
}

// GetMediaType returns the resolved media type for this argument.
func (a *CapArg) GetMediaType(mediaSpecs map[string]MediaSpecDef) (string, error) {
	resolved, err := a.Resolve(mediaSpecs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve argument media_urn '%s': %w", a.MediaUrn, err)
	}
	return resolved.MediaType, nil
}

// CapOutput represents the output definition for a cap
type CapOutput struct {
	MediaUrn          string              `json:"media_urn"`
	OutputDescription string              `json:"output_description"`
	Validation        *ArgumentValidation `json:"validation,omitempty"`
	Metadata          any                 `json:"metadata,omitempty"`
}

// Resolve resolves the output's media URN to a ResolvedMediaSpec
func (co *CapOutput) Resolve(mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	return ResolveMediaUrn(co.MediaUrn, mediaSpecs)
}

// IsBinary checks if this output produces binary data.
func (co *CapOutput) IsBinary(mediaSpecs map[string]MediaSpecDef) (bool, error) {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return false, fmt.Errorf("failed to resolve output media_urn '%s': %w", co.MediaUrn, err)
	}
	return resolved.IsBinary(), nil
}

// IsJSON checks if this output produces JSON data.
func (co *CapOutput) IsJSON(mediaSpecs map[string]MediaSpecDef) (bool, error) {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return false, fmt.Errorf("failed to resolve output media_urn '%s': %w", co.MediaUrn, err)
	}
	return resolved.IsJSON(), nil
}

// GetMediaType returns the resolved media type for this output.
func (co *CapOutput) GetMediaType(mediaSpecs map[string]MediaSpecDef) (string, error) {
	resolved, err := co.Resolve(mediaSpecs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve output media_urn '%s': %w", co.MediaUrn, err)
	}
	return resolved.MediaType, nil
}

// GetMetadata gets the metadata JSON for CapOutput
func (co *CapOutput) GetMetadata() any {
	return co.Metadata
}

// SetMetadata sets the metadata JSON for CapOutput
func (co *CapOutput) SetMetadata(metadata any) {
	co.Metadata = metadata
}

// NewCapOutput creates a new output definition with a media URN
func NewCapOutput(mediaUrn string, description string) *CapOutput {
	return &CapOutput{
		MediaUrn:          mediaUrn,
		OutputDescription: description,
	}
}

// RegisteredBy represents registration attribution - who registered a capability and when
type RegisteredBy struct {
	Username     string `json:"username"`
	RegisteredAt string `json:"registered_at"`
}

// NewRegisteredBy creates a new registration attribution
func NewRegisteredBy(username string, registeredAt string) RegisteredBy {
	return RegisteredBy{
		Username:     username,
		RegisteredAt: registeredAt,
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

// Cap represents a formal cap definition
type Cap struct {
	Urn            *CapUrn                  `json:"urn"`
	Title          string                   `json:"title"`
	CapDescription *string                  `json:"cap_description,omitempty"`
	Metadata       map[string]string        `json:"metadata,omitempty"`
	Command        string                   `json:"command"`
	MediaSpecs     map[string]MediaSpecDef  `json:"media_specs,omitempty"`
	Args           []CapArg                 `json:"args,omitempty"`
	Output         *CapOutput               `json:"output,omitempty"`
	MetadataJSON   any                      `json:"metadata_json,omitempty"`
	RegisteredBy   *RegisteredBy            `json:"registered_by,omitempty"`
}

// NewCap creates a new cap
func NewCap(urn *CapUrn, title string, command string) *Cap {
	return &Cap{
		Urn:        urn,
		Title:      title,
		Command:    command,
		Metadata:   make(map[string]string),
		MediaSpecs: make(map[string]MediaSpecDef),
		Args:       []CapArg{},
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
		Args:           []CapArg{},
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
		Args:       []CapArg{},
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

// ResolveMediaUrn resolves a media URN using this cap's media_specs table
func (c *Cap) ResolveMediaUrn(mediaUrn string) (*ResolvedMediaSpec, error) {
	return ResolveMediaUrn(mediaUrn, c.GetMediaSpecs())
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

// GetOutput gets the output definition if defined
func (c *Cap) GetOutput() *CapOutput {
	return c.Output
}

// SetOutput sets the output definition
func (c *Cap) SetOutput(output *CapOutput) {
	c.Output = output
}

// GetMetadataJSON gets the metadata JSON
func (c *Cap) GetMetadataJSON() any {
	return c.MetadataJSON
}

// SetMetadataJSON sets the metadata JSON
func (c *Cap) SetMetadataJSON(metadata any) {
	c.MetadataJSON = metadata
}

// GetRegisteredBy gets the registration attribution
func (c *Cap) GetRegisteredBy() *RegisteredBy {
	return c.RegisteredBy
}

// SetRegisteredBy sets the registration attribution
func (c *Cap) SetRegisteredBy(registeredBy *RegisteredBy) {
	c.RegisteredBy = registeredBy
}

// GetStdinMediaUrn returns the stdin media URN from args (first stdin source found)
func (c *Cap) GetStdinMediaUrn() *string {
	for _, arg := range c.Args {
		if urn := arg.GetStdinMediaUrn(); urn != nil {
			return urn
		}
	}
	return nil
}

// AcceptsStdin returns true if any arg has a stdin source
func (c *Cap) AcceptsStdin() bool {
	return c.GetStdinMediaUrn() != nil
}

// GetArgs returns the args
func (c *Cap) GetArgs() []CapArg {
	return c.Args
}

// AddArg adds an argument
func (c *Cap) AddArg(arg CapArg) {
	c.Args = append(c.Args, arg)
}

// GetRequiredArgs returns all required arguments
func (c *Cap) GetRequiredArgs() []CapArg {
	var required []CapArg
	for _, arg := range c.Args {
		if arg.Required {
			required = append(required, arg)
		}
	}
	return required
}

// GetOptionalArgs returns all optional arguments
func (c *Cap) GetOptionalArgs() []CapArg {
	var optional []CapArg
	for _, arg := range c.Args {
		if !arg.Required {
			optional = append(optional, arg)
		}
	}
	return optional
}

// FindArgByMediaUrn finds an argument by media_urn
func (c *Cap) FindArgByMediaUrn(mediaUrn string) *CapArg {
	for i := range c.Args {
		if c.Args[i].MediaUrn == mediaUrn {
			return &c.Args[i]
		}
	}
	return nil
}

// GetPositionalArgs returns arguments that have position sources, sorted by position
func (c *Cap) GetPositionalArgs() []CapArg {
	var positional []CapArg
	for _, arg := range c.Args {
		if arg.HasPositionSource() {
			positional = append(positional, arg)
		}
	}
	// Sort by position
	for i := 0; i < len(positional)-1; i++ {
		for j := i + 1; j < len(positional); j++ {
			posI := positional[i].GetPosition()
			posJ := positional[j].GetPosition()
			if posI != nil && posJ != nil && *posI > *posJ {
				positional[i], positional[j] = positional[j], positional[i]
			}
		}
	}
	return positional
}

// GetFlagArgs returns arguments that have cli_flag sources
func (c *Cap) GetFlagArgs() []CapArg {
	var flagArgs []CapArg
	for _, arg := range c.Args {
		if arg.HasCliFlagSource() {
			flagArgs = append(flagArgs, arg)
		}
	}
	return flagArgs
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

	if !reflect.DeepEqual(c.MediaSpecs, other.MediaSpecs) {
		return false
	}

	if !reflect.DeepEqual(c.Args, other.Args) {
		return false
	}

	if !reflect.DeepEqual(c.Output, other.Output) {
		return false
	}

	if !reflect.DeepEqual(c.MetadataJSON, other.MetadataJSON) {
		return false
	}

	if !reflect.DeepEqual(c.RegisteredBy, other.RegisteredBy) {
		return false
	}

	return true
}

// MarshalJSON implements custom JSON marshaling
func (c *Cap) MarshalJSON() ([]byte, error) {
	allTags := make(map[string]string)
	allTags["in"] = c.Urn.inSpec
	allTags["out"] = c.Urn.outSpec
	for k, v := range c.Urn.tags {
		allTags[k] = v
	}

	capData := map[string]any{
		"urn": map[string]any{
			"tags": allTags,
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

	if len(c.Args) > 0 {
		capData["args"] = c.Args
	}

	if c.Output != nil {
		capData["output"] = c.Output
	}

	if c.MetadataJSON != nil {
		capData["metadata_json"] = c.MetadataJSON
	}

	if c.RegisteredBy != nil {
		capData["registered_by"] = c.RegisteredBy
	}

	return json.Marshal(capData)
}

// UnmarshalJSON implements custom JSON unmarshaling
func (c *Cap) UnmarshalJSON(data []byte) error {
	var raw map[string]any
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
	case map[string]any:
		tagsField, ok := v["tags"]
		if !ok {
			return fmt.Errorf("URN object missing 'tags' field")
		}
		tagsMap, ok := tagsField.(map[string]any)
		if !ok {
			return fmt.Errorf("URN tags field must be an object")
		}

		tags := make(map[string]string)
		for k, v := range tagsMap {
			if s, ok := v.(string); ok {
				tags[k] = s
			}
		}
		urn, err = NewCapUrnFromTags(tags)
		if err != nil {
			return fmt.Errorf("failed to create URN from tags: %v", err)
		}
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

	if metadata, ok := raw["metadata"].(map[string]any); ok {
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
		if err := json.Unmarshal(mediaSpecsBytes, &mediaSpecs); err != nil {
			return fmt.Errorf("failed to unmarshal media_specs: %w", err)
		}
		c.MediaSpecs = mediaSpecs
	}

	// Handle args
	if argsRaw, ok := raw["args"]; ok {
		argsBytes, _ := json.Marshal(argsRaw)
		var args []CapArg
		if err := json.Unmarshal(argsBytes, &args); err != nil {
			return fmt.Errorf("failed to unmarshal args: %w", err)
		}
		c.Args = args
	}

	// Handle output
	if output, ok := raw["output"]; ok {
		outputBytes, _ := json.Marshal(output)
		var capOutput CapOutput
		if err := json.Unmarshal(outputBytes, &capOutput); err != nil {
			return fmt.Errorf("failed to unmarshal output: %w", err)
		}
		c.Output = &capOutput
	}

	if metadataJSON, ok := raw["metadata_json"]; ok {
		c.MetadataJSON = metadataJSON
	}

	if registeredByRaw, ok := raw["registered_by"]; ok {
		registeredByBytes, _ := json.Marshal(registeredByRaw)
		var registeredBy RegisteredBy
		if err := json.Unmarshal(registeredByBytes, &registeredBy); err != nil {
			return fmt.Errorf("failed to unmarshal registered_by: %w", err)
		}
		c.RegisteredBy = &registeredBy
	}

	return nil
}
