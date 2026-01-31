// Package capns provides MediaSpec parsing and media URN resolution
//
// Media URNs reference media type definitions in the media_specs table.
// Format: `media:<type>;v=<version>` with optional profile tag.
//
// Examples:
// - `media:string`
// - `media:object;profile="https://example.com/schema.json"`
//
// NO LEGACY SUPPORT: The old `std:xxx.v1` format is NOT supported and will fail hard.
package capns

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	taggedurn "github.com/filegrind/tagged-urn-go"
)

// Built-in media URN constants with coercion tags
const (
	MediaVoid         = "media:void"
	MediaString       = "media:textable;form=scalar"
	MediaInteger      = "media:integer;textable;numeric;form=scalar"
	MediaNumber       = "media:textable;numeric;form=scalar"
	MediaBoolean      = "media:bool;textable;form=scalar"
	MediaObject       = "media:form=map;textable"
	MediaBinary       = "media:bytes"
	MediaStringArray  = "media:textable;form=list"
	MediaIntegerArray = "media:integer;textable;numeric;form=list"
	MediaNumberArray  = "media:textable;numeric;form=list"
	MediaBooleanArray = "media:bool;textable;form=list"
	MediaObjectArray  = "media:form=list;textable"
	// Semantic content types
	MediaImage = "media:image;png;bytes"
	MediaAudio = "media:wav;audio;bytes;"
	MediaVideo = "media:video;bytes"
	// Semantic AI input types
	MediaAudioSpeech    = "media:audio;wav;bytes;speech"
	MediaImageThumbnail = "media:image;png;bytes;thumbnail"
	// Document types (PRIMARY naming - type IS the format)
	MediaPdf  = "media:pdf;bytes"
	MediaEpub = "media:epub;bytes"
	// Text format types (PRIMARY naming - type IS the format)
	MediaMd   = "media:md;textable"
	MediaTxt  = "media:txt;textable"
	MediaRst  = "media:rst;textable"
	MediaLog  = "media:log;textable"
	MediaHtml = "media:html;textable"
	MediaXml  = "media:xml;textable"
	MediaJson       = "media:json;textable;form=map"
	MediaJsonSchema = "media:json;json-schema;textable;form=map"
	MediaYaml       = "media:yaml;textable;form=map"
	// Semantic input types
	MediaModelSpec = "media:model-spec;textable;form=scalar"
	MediaModelRepo = "media:model-repo;textable;form=map"
	// Semantic output types
	MediaModelDim      = "media:model-dim;integer;textable;numeric;form=scalar"
	MediaDecision      = "media:decision;bool;textable;form=scalar"
	MediaDecisionArray = "media:decision;bool;textable;form=list"
)

// Profile URL constants (defaults, use GetSchemaBase() for configurable version)
const (
	SchemaBase       = "https://capns.org/schema"
	ProfileStr       = "https://capns.org/schema/str"
	ProfileInt       = "https://capns.org/schema/int"
	ProfileNum       = "https://capns.org/schema/num"
	ProfileBool      = "https://capns.org/schema/bool"
	ProfileObj       = "https://capns.org/schema/obj"
	ProfileStrArray  = "https://capns.org/schema/str-array"
	ProfileIntArray  = "https://capns.org/schema/int-array"
	ProfileNumArray  = "https://capns.org/schema/num-array"
	ProfileBoolArray = "https://capns.org/schema/bool-array"
	ProfileObjArray  = "https://capns.org/schema/obj-array"
	ProfileVoid      = "https://capns.org/schema/void"
	// Semantic content type profiles
	ProfileImage = "https://capns.org/schema/image"
	ProfileAudio = "https://capns.org/schema/audio"
	ProfileVideo = "https://capns.org/schema/video"
	ProfileText  = "https://capns.org/schema/text"
	// Document type profiles (PRIMARY naming)
	ProfilePdf  = "https://capns.org/schema/pdf"
	ProfileEpub = "https://capns.org/schema/epub"
	// Text format type profiles (PRIMARY naming)
	ProfileMd   = "https://capns.org/schema/md"
	ProfileTxt  = "https://capns.org/schema/txt"
	ProfileRst  = "https://capns.org/schema/rst"
	ProfileLog  = "https://capns.org/schema/log"
	ProfileHtml = "https://capns.org/schema/html"
	ProfileXml  = "https://capns.org/schema/xml"
	ProfileJson = "https://capns.org/schema/json"
	ProfileYaml = "https://capns.org/schema/yaml"
)

// GetSchemaBase returns the schema base URL from environment variables or default
//
// Checks in order:
//  1. CAPNS_SCHEMA_BASE_URL environment variable
//  2. CAPNS_REGISTRY_URL environment variable + "/schema"
//  3. Default: "https://capns.org/schema"
func GetSchemaBase() string {
	if schemaURL := os.Getenv("CAPNS_SCHEMA_BASE_URL"); schemaURL != "" {
		return schemaURL
	}
	if registryURL := os.Getenv("CAPNS_REGISTRY_URL"); registryURL != "" {
		return registryURL + "/schema"
	}
	return SchemaBase
}

// GetProfileURL returns a profile URL for the given profile name
//
// Example:
//
//	url := GetProfileURL("str") // Returns "{schema_base}/str"
func GetProfileURL(profileName string) string {
	return GetSchemaBase() + "/" + profileName
}

// MediaSpecDefObject represents the rich object form of a media spec definition
type MediaSpecDefObject struct {
	MediaType   string                 `json:"media_type"`
	ProfileURI  string                 `json:"profile_uri"`
	Schema      interface{}            `json:"schema,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Validation  *MediaValidation       `json:"validation,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Extension   string                 `json:"extension,omitempty"`
}

// MediaSpecDef represents a media spec definition - can be string (compact) or object (rich)
// String form: "text/plain; profile=https://..."
// Object form: { media_type, profile_uri, schema? }
type MediaSpecDef struct {
	// IsString indicates if this is the compact string form
	IsString bool
	// StringValue holds the value when IsString is true
	StringValue string
	// ObjectValue holds the value when IsString is false
	ObjectValue *MediaSpecDefObject
}

// NewMediaSpecDefString creates a compact string form media spec def
func NewMediaSpecDefString(value string) MediaSpecDef {
	return MediaSpecDef{
		IsString:    true,
		StringValue: value,
	}
}

// NewMediaSpecDefObject creates a rich object form media spec def
func NewMediaSpecDefObject(mediaType, profileURI string) MediaSpecDef {
	return MediaSpecDef{
		IsString: false,
		ObjectValue: &MediaSpecDefObject{
			MediaType:  mediaType,
			ProfileURI: profileURI,
		},
	}
}

// NewMediaSpecDefObjectWithSchema creates a rich object form with schema
func NewMediaSpecDefObjectWithSchema(mediaType, profileURI string, schema interface{}) MediaSpecDef {
	return MediaSpecDef{
		IsString: false,
		ObjectValue: &MediaSpecDefObject{
			MediaType:  mediaType,
			ProfileURI: profileURI,
			Schema:     schema,
		},
	}
}

// MarshalJSON implements custom JSON marshaling
func (m MediaSpecDef) MarshalJSON() ([]byte, error) {
	if m.IsString {
		return json.Marshal(m.StringValue)
	}
	return json.Marshal(m.ObjectValue)
}

// UnmarshalJSON implements custom JSON unmarshaling
func (m *MediaSpecDef) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		m.IsString = true
		m.StringValue = str
		return nil
	}

	// Try object
	var obj MediaSpecDefObject
	if err := json.Unmarshal(data, &obj); err == nil {
		m.IsString = false
		m.ObjectValue = &obj
		return nil
	}

	return errors.New("MediaSpecDef must be string or object")
}

// ResolvedMediaSpec represents a fully resolved media spec with all fields populated
type ResolvedMediaSpec struct {
	SpecID      string
	MediaType   string
	ProfileURI  string
	Schema      interface{}
	Title       string
	Description string
	Validation  *MediaValidation
	// Metadata contains arbitrary key-value pairs for display/categorization
	Metadata  map[string]interface{}
	// Extension is the optional file extension for storing this media type (e.g., "pdf", "json", "txt")
	Extension string
}

// IsBinary returns true if the "bytes" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsBinary() bool {
	return HasMediaUrnTag(r.SpecID, "bytes")
}

// IsMap returns true if form=map tag is present (key-value structure).
func (r *ResolvedMediaSpec) IsMap() bool {
	return HasMediaUrnTagValue(r.SpecID, "form", "map")
}

// IsScalar returns true if form=scalar tag is present (single value).
func (r *ResolvedMediaSpec) IsScalar() bool {
	return HasMediaUrnTagValue(r.SpecID, "form", "scalar")
}

// IsList returns true if form=list tag is present (ordered collection).
func (r *ResolvedMediaSpec) IsList() bool {
	return HasMediaUrnTagValue(r.SpecID, "form", "list")
}

// IsJSON returns true if the "json" marker tag is present in the source media URN.
// Note: This checks for JSON representation specifically, not map structure (use IsMap for that).
func (r *ResolvedMediaSpec) IsJSON() bool {
	return HasMediaUrnTag(r.SpecID, "json")
}

// IsStructured returns true if this represents structured data (map or list).
// Structured data can be serialized as JSON when transmitted as text.
// Note: This does NOT check for the explicit `json` tag - use IsJSON() for that.
func (r *ResolvedMediaSpec) IsStructured() bool {
	return r.IsMap() || r.IsList()
}

// IsText returns true if the "textable" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsText() bool {
	return HasMediaUrnTag(r.SpecID, "textable")
}

// HasMediaUrnTag checks if a media URN has a marker tag (e.g., bytes, json, textable).
// Uses tagged-urn parsing for proper tag detection.
func HasMediaUrnTag(mediaUrn, tagName string) bool {
	if mediaUrn == "" {
		return false
	}
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		return false
	}
	_, exists := parsed.GetTag(tagName)
	return exists
}

// HasMediaUrnTagValue checks if a media URN has a tag with a specific value (e.g., form=map).
// Uses tagged-urn parsing for proper tag detection.
func HasMediaUrnTagValue(mediaUrn, tagKey, tagValue string) bool {
	if mediaUrn == "" {
		return false
	}
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		return false
	}
	value, exists := parsed.GetTag(tagKey)
	return exists && value == tagValue
}

// PrimaryType returns the primary type (e.g., "image" from "image/png")
func (r *ResolvedMediaSpec) PrimaryType() string {
	parts := strings.SplitN(r.MediaType, "/", 2)
	return parts[0]
}

// Subtype returns the subtype (e.g., "png" from "image/png")
func (r *ResolvedMediaSpec) Subtype() string {
	parts := strings.SplitN(r.MediaType, "/", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// String returns the canonical string representation
func (r *ResolvedMediaSpec) String() string {
	if r.ProfileURI != "" {
		return fmt.Sprintf("%s; profile=%s", r.MediaType, r.ProfileURI)
	}
	return r.MediaType
}

// MediaSpecError represents an error in media spec operations
type MediaSpecError struct {
	Message string
}

func (e *MediaSpecError) Error() string {
	return e.Message
}

var (
	ErrUnresolvableMediaUrn = &MediaSpecError{"media URN cannot be resolved"}
	ErrLegacyFormat         = &MediaSpecError{"legacy 'std:xxx.v1' format is not supported - use media URN format"}
	ErrEmptyMediaType       = &MediaSpecError{"media type cannot be empty"}
	ErrUnterminatedQuote    = &MediaSpecError{"unterminated quote in profile value"}
	ErrInvalidMediaUrn      = &MediaSpecError{"invalid media URN - must start with 'media:'"}
)

// NewUnresolvableMediaUrnError creates an error for unresolvable media URNs
func NewUnresolvableMediaUrnError(mediaUrn string) error {
	return &MediaSpecError{
		Message: fmt.Sprintf("media URN '%s' cannot be resolved - not found in media_specs", mediaUrn),
	}
}

// ResolveMediaUrn resolves a media URN to a ResolvedMediaSpec
// Resolution: Look up in provided media_specs map, FAIL HARD if not found
func ResolveMediaUrn(mediaUrn string, mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	// Validate it's a media URN
	if !strings.HasPrefix(mediaUrn, "media:") {
		return nil, ErrInvalidMediaUrn
	}

	// Look up in the provided media_specs
	if def, exists := mediaSpecs[mediaUrn]; exists {
		return resolveMediaSpecDef(mediaUrn, &def)
	}

	// FAIL HARD - media URN must be in media_specs
	return nil, NewUnresolvableMediaUrnError(mediaUrn)
}

// resolveMediaSpecDef resolves a MediaSpecDef to a ResolvedMediaSpec
func resolveMediaSpecDef(specID string, def *MediaSpecDef) (*ResolvedMediaSpec, error) {
	if def.IsString {
		// Parse the string form: "media_type; profile=url"
		parsed, err := ParseMediaSpec(def.StringValue)
		if err != nil {
			return nil, err
		}
		return &ResolvedMediaSpec{
			SpecID:      specID,
			MediaType:   parsed.MediaType,
			ProfileURI:  parsed.ProfileURI,
			Schema:      nil,
			Title:       "",
			Description: "",
			Validation:  nil,      // String form has no validation
			Metadata:    nil,      // String form has no metadata
		}, nil
	}

	// Object form
	if def.ObjectValue == nil {
		return nil, &MediaSpecError{Message: "invalid media spec def: object value is nil"}
	}
	return &ResolvedMediaSpec{
		SpecID:      specID,
		MediaType:   def.ObjectValue.MediaType,
		ProfileURI:  def.ObjectValue.ProfileURI,
		Schema:      def.ObjectValue.Schema,
		Title:       def.ObjectValue.Title,
		Description: def.ObjectValue.Description,
		Validation:  def.ObjectValue.Validation,
		Metadata:    def.ObjectValue.Metadata,  // Propagate metadata
		Extension:   def.ObjectValue.Extension, // Propagate extension
	}, nil
}

// extractBaseType extracts the base type identifier from a media URN
// e.g., "media:textable;form=scalar" -> "string"
func extractBaseType(mediaUrn string) string {
	if !strings.HasPrefix(mediaUrn, "media:") {
		return ""
	}
	// Parse tags from the media URN
	rest := mediaUrn[6:] // Skip "media:"
	parts := strings.Split(rest, ";")

	var typeVal, vVal string
	for _, part := range parts {
		if strings.HasPrefix(part, "type=") {
			typeVal = part[5:]
		} else if strings.HasPrefix(part, "v=") {
			vVal = part[2:]
		}
	}
	if typeVal != "" && vVal != "" {
		return typeVal + ";v=" + vVal
	}
	if typeVal != "" {
		return typeVal
	}
	return ""
}

// ParsedMediaSpec represents a parsed media spec string (canonical form)
type ParsedMediaSpec struct {
	MediaType  string
	ProfileURI string
}

// ParseMediaSpec parses a media spec string in CANONICAL format only
// Format: `<mime-type>; profile=<url>`
// NO SUPPORT for legacy `content-type:` prefix - will FAIL HARD
func ParseMediaSpec(s string) (*ParsedMediaSpec, error) {
	s = strings.TrimSpace(s)

	// FAIL HARD on legacy format
	if strings.HasPrefix(strings.ToLower(s), "content-type:") {
		return nil, ErrLegacyFormat
	}

	// Split by semicolon to separate mime type from parameters
	parts := strings.SplitN(s, ";", 2)

	mediaType := strings.TrimSpace(parts[0])
	if mediaType == "" {
		return nil, ErrEmptyMediaType
	}

	// Parse profile if present
	var profileURI string
	if len(parts) > 1 {
		params := strings.TrimSpace(parts[1])
		var err error
		profileURI, err = parseProfile(params)
		if err != nil {
			return nil, err
		}
	}

	return &ParsedMediaSpec{
		MediaType:  mediaType,
		ProfileURI: profileURI,
	}, nil
}

// parseProfile extracts the profile value from parameters string
func parseProfile(params string) (string, error) {
	// Look for profile= (case-insensitive)
	lower := strings.ToLower(params)
	pos := strings.Index(lower, "profile=")
	if pos == -1 {
		return "", nil
	}

	afterProfile := params[pos+8:]

	// Handle quoted value
	if strings.HasPrefix(afterProfile, "\"") {
		rest := afterProfile[1:]
		endPos := strings.Index(rest, "\"")
		if endPos == -1 {
			return "", ErrUnterminatedQuote
		}
		return rest[:endPos], nil
	}

	// Unquoted value - take until semicolon or end
	semicolonPos := strings.Index(afterProfile, ";")
	if semicolonPos != -1 {
		return strings.TrimSpace(afterProfile[:semicolonPos]), nil
	}
	return strings.TrimSpace(afterProfile), nil
}

// GetTypeFromMediaUrn returns the base type (string, integer, number, boolean, object, binary, etc.) from a media URN
// This is useful for validation to determine what Go type to expect
// Determines type based on media URN tags: bytes->binary, form=map->object, form=list->array, etc.
func GetTypeFromMediaUrn(mediaUrn string) string {
	// Parse the media URN to check tags
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		return "unknown"
	}

	// Check for binary (has "bytes" tag)
	if _, ok := parsed.GetTag("bytes"); ok {
		return "binary"
	}

	// Check for void
	if _, ok := parsed.GetTag("void"); ok {
		return "void"
	}

	// Check form tag for structure type
	if form, ok := parsed.GetTag("form"); ok {
		switch form {
		case "map":
			return "object"
		case "list":
			return "array"
		case "scalar":
			// Explicit scalar - check specific type tags below
		}
	}

	// Check specific type tags (works regardless of whether form is specified)
	if _, ok := parsed.GetTag("integer"); ok {
		return "integer"
	}
	if _, ok := parsed.GetTag("numeric"); ok {
		return "number"
	}
	if _, ok := parsed.GetTag("number"); ok {
		return "number"
	}
	if _, ok := parsed.GetTag("bool"); ok {
		return "boolean"
	}
	if _, ok := parsed.GetTag("textable"); ok {
		return "string"
	}

	return "unknown"
}

// GetTypeFromResolvedMediaSpec determines the type from a resolved media spec
func GetTypeFromResolvedMediaSpec(resolved *ResolvedMediaSpec) string {
	if resolved.IsBinary() {
		return "binary"
	}
	// Check for map structure (form=map) OR explicit json tag
	if resolved.IsMap() || resolved.IsJSON() {
		return "object"
	}
	// Check for list structure (form=list)
	if resolved.IsList() {
		return "array"
	}
	// Scalar or text types
	if resolved.IsText() || resolved.IsScalar() {
		return "string"
	}
	return "unknown"
}

// GetMediaSpecFromCapUrn extracts media spec from a CapUrn using the 'out' tag
// The 'out' tag contains a media URN
func GetMediaSpecFromCapUrn(urn *CapUrn, mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	outUrn := urn.OutSpec()
	if outUrn == "" {
		return nil, errors.New("no 'out' tag found in cap URN")
	}
	return ResolveMediaUrn(outUrn, mediaSpecs)
}
