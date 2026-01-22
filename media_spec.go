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
	"strings"

	taggedurn "github.com/fgrnd/tagged-urn-go"
)

// Built-in media URN constants with coercion tags
const (
	MediaVoid         = "media:void"
	MediaString       = "media:string;textable;scalar"
	MediaInteger      = "media:integer;textable;numeric;scalar"
	MediaNumber       = "media:number;textable;numeric;scalar"
	MediaBoolean      = "media:boolean;textable;scalar"
	MediaObject       = "media:object;textable;keyed"
	MediaBinary       = "media:raw;binary"
	MediaStringArray  = "media:string-array;textable;sequence"
	MediaIntegerArray = "media:integer-array;textable;numeric;sequence"
	MediaNumberArray  = "media:number-array;textable;numeric;sequence"
	MediaBooleanArray = "media:boolean-array;textable;sequence"
	MediaObjectArray  = "media:object-array;textable;keyed;sequence"
	// Semantic content types
	MediaImage = "media:png;binary"
	MediaAudio = "media:wav;audio;binary;"
	MediaVideo = "media:video;binary"
	MediaText  = "media:text;textable"
	// Document types (PRIMARY naming - type IS the format)
	MediaPdf  = "media:pdf;binary"
	MediaEpub = "media:epub;binary"
	// Text format types (PRIMARY naming - type IS the format)
	MediaMd   = "media:md;textable"
	MediaTxt  = "media:txt;textable"
	MediaRst  = "media:rst;textable"
	MediaLog  = "media:log;textable"
	MediaHtml = "media:html;textable"
	MediaXml  = "media:xml;textable"
	MediaJson = "media:json;textable;keyed"
	MediaYaml = "media:yaml;textable;keyed"
)

// Profile URL constants
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

// MediaSpecDefObject represents the rich object form of a media spec definition
type MediaSpecDefObject struct {
	MediaType   string      `json:"media_type"`
	ProfileURI  string      `json:"profile_uri"`
	Schema      interface{} `json:"schema,omitempty"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
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
}

// IsBinary returns true if the "binary" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsBinary() bool {
	return HasMediaUrnTag(r.SpecID, "binary")
}

// IsJSON returns true if the "keyed" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsJSON() bool {
	return HasMediaUrnTag(r.SpecID, "keyed")
}

// IsText returns true if the "textable" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsText() bool {
	return HasMediaUrnTag(r.SpecID, "textable")
}

// HasMediaUrnTag checks if a media URN has a marker tag (e.g., binary, keyed, textable).
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
		Message: fmt.Sprintf("media URN '%s' cannot be resolved - not found in media_specs and not a built-in", mediaUrn),
	}
}

// ResolveMediaUrn resolves a media URN to a ResolvedMediaSpec
// Resolution order:
// 1. Look up in provided media_specs map
// 2. Check if it's a built-in primitive
// 3. FAIL HARD if not found - no fallbacks
func ResolveMediaUrn(mediaUrn string, mediaSpecs map[string]MediaSpecDef) (*ResolvedMediaSpec, error) {
	// Validate it's a media URN
	if !strings.HasPrefix(mediaUrn, "media:") {
		return nil, ErrInvalidMediaUrn
	}

	// First, try to look up in the provided media_specs
	if def, exists := mediaSpecs[mediaUrn]; exists {
		return resolveMediaSpecDef(mediaUrn, &def)
	}

	// Second, check if it's a built-in primitive
	if resolved := resolveBuiltin(mediaUrn); resolved != nil {
		return resolved, nil
	}

	// FAIL HARD - no fallbacks
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
			SpecID:     specID,
			MediaType:  parsed.MediaType,
			ProfileURI: parsed.ProfileURI,
			Schema:     nil,
			Title:      "",
			Description: "",
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
	}, nil
}

// extractBaseType extracts the base type identifier from a media URN
// e.g., "media:string;textable;scalar" -> "string"
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

// resolveBuiltin resolves built-in media URNs by exact match against constants
// Mirrors the Rust reference implementation exactly
func resolveBuiltin(mediaUrn string) *ResolvedMediaSpec {
	switch mediaUrn {
	// Standard primitives
	case MediaString:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileStr}
	case MediaInteger:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileInt}
	case MediaNumber:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileNum}
	case MediaBoolean:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileBool}
	case MediaObject:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileObj}
	case MediaStringArray:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileStrArray}
	case MediaIntegerArray:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileIntArray}
	case MediaNumberArray:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileNumArray}
	case MediaBooleanArray:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileBoolArray}
	case MediaObjectArray:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileObjArray}
	case MediaBinary:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/octet-stream", ProfileURI: ""}
	case MediaVoid:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/x-void", ProfileURI: ProfileVoid}
	// Semantic content types
	case MediaImage:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "image/png", ProfileURI: ProfileImage}
	case MediaAudio:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "audio/wav", ProfileURI: ProfileAudio}
	case MediaVideo:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "video/mp4", ProfileURI: ProfileVideo}
	case MediaText:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileText}
	// Document types (PRIMARY naming)
	case MediaPdf:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/pdf", ProfileURI: ProfilePdf}
	case MediaEpub:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/epub+zip", ProfileURI: ProfileEpub}
	// Text format types (PRIMARY naming)
	case MediaMd:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/markdown", ProfileURI: ProfileMd}
	case MediaTxt:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileTxt}
	case MediaRst:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/x-rst", ProfileURI: ProfileRst}
	case MediaLog:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/plain", ProfileURI: ProfileLog}
	case MediaHtml:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "text/html", ProfileURI: ProfileHtml}
	case MediaXml:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/xml", ProfileURI: ProfileXml}
	case MediaJson:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/json", ProfileURI: ProfileJson}
	case MediaYaml:
		return &ResolvedMediaSpec{SpecID: mediaUrn, MediaType: "application/x-yaml", ProfileURI: ProfileYaml}
	default:
		return nil
	}
}

// IsBuiltinMediaUrn checks if a media URN is a built-in
func IsBuiltinMediaUrn(mediaUrn string) bool {
	return resolveBuiltin(mediaUrn) != nil
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
func GetTypeFromMediaUrn(mediaUrn string) string {
	resolved := resolveBuiltin(mediaUrn)
	if resolved == nil {
		// For non-builtin media URNs, we need to resolve and check media type
		return "unknown"
	}

	switch mediaUrn {
	case MediaString:
		return "string"
	case MediaInteger:
		return "integer"
	case MediaNumber:
		return "number"
	case MediaBoolean:
		return "boolean"
	case MediaObject:
		return "object"
	case MediaStringArray, MediaIntegerArray, MediaNumberArray, MediaBooleanArray, MediaObjectArray:
		return "array"
	case MediaBinary:
		return "binary"
	case MediaVoid:
		return "void"
	default:
		return "unknown"
	}
}

// GetTypeFromResolvedMediaSpec determines the type from a resolved media spec
func GetTypeFromResolvedMediaSpec(resolved *ResolvedMediaSpec) string {
	if resolved.IsBinary() {
		return "binary"
	}
	if resolved.IsJSON() {
		return "object" // JSON can be object or array, but object is the default assumption
	}
	if resolved.IsText() {
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

// extractMediaUrnTag extracts a tag value from a media URN string
// e.g., extractMediaUrnTag("media:image;ext=png", "type") returns "image"
func extractMediaUrnTag(mediaUrn, tagName string) string {
	prefix := tagName + "="
	pos := strings.Index(mediaUrn, prefix)
	if pos == -1 {
		return ""
	}

	start := pos + len(prefix)
	rest := mediaUrn[start:]
	semicolonPos := strings.Index(rest, ";")
	if semicolonPos == -1 {
		return rest
	}
	return rest[:semicolonPos]
}

// MediaUrnSatisfies checks if a provided media URN satisfies another media URN's requirements.
// Used for cap matching - checks if a provided media type can satisfy a cap's input requirement.
//
// Matching rules:
// - Type must match (e.g., "image" != "binary")
// - Extension must match if specified in requirement
// - Version must match if specified in requirement
func MediaUrnSatisfies(providedUrn, requirementUrn string) bool {
	if providedUrn == "" || requirementUrn == "" {
		return false
	}

	// Extract type from both URNs
	providedType := extractMediaUrnTag(providedUrn, "type")
	requiredType := extractMediaUrnTag(requirementUrn, "type")

	// Type must match if required
	if requiredType != "" {
		if providedType == "" || providedType != requiredType {
			return false
		}
	}

	// Extension must match if specified in requirement
	requiredExt := extractMediaUrnTag(requirementUrn, "ext")
	if requiredExt != "" {
		providedExt := extractMediaUrnTag(providedUrn, "ext")
		if providedExt == "" || providedExt != requiredExt {
			return false
		}
	}

	// Version must match if specified in requirement
	requiredVersion := extractMediaUrnTag(requirementUrn, "v")
	if requiredVersion != "" {
		providedVersion := extractMediaUrnTag(providedUrn, "v")
		if providedVersion == "" || providedVersion != requiredVersion {
			return false
		}
	}

	return true
}

// Legacy compatibility shim - REMOVED
// The old MediaSpec type that required content-type: prefix is gone.
// Use ParsedMediaSpec and ResolvedMediaSpec instead.
