// Package capns provides MediaSpec parsing and media URN resolution
//
// Media URNs reference media type definitions in the media_specs array.
// Format: `media:<type>` with optional tags.
//
// Examples:
// - `media:textable;form=scalar`
// - `media:pdf;bytes`
//
// MediaSpecDef is always a structured object - NO string form parsing.
package media

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/filegrind/capns-go/urn"
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
	MediaMd         = "media:md;textable"
	MediaTxt        = "media:txt;textable"
	MediaRst        = "media:rst;textable"
	MediaLog        = "media:log;textable"
	MediaHtml       = "media:html;textable"
	MediaXml        = "media:xml;textable"
	MediaJson       = "media:json;textable;form=map"
	MediaJsonSchema = "media:json;json-schema;textable;form=map"
	MediaYaml       = "media:yaml;textable;form=map"
	// Semantic input types
	MediaModelSpec = "media:model-spec;textable;form=scalar"
	MediaModelRepo = "media:model-repo;textable;form=map"
	// File path types
	MediaFilePath      = "media:file-path;textable;form=scalar"
	MediaFilePathArray = "media:file-path;textable;form=list"
	// Semantic output types
	MediaModelDim      = "media:model-dim;integer;textable;numeric;form=scalar"
	MediaDecision      = "media:decision;bool;textable;form=scalar"
	MediaDecisionArray = "media:decision;bool;textable;form=list"
	// Semantic output types
	MediaLlmInferenceOutput = "media:generated-text;textable;form=map"
	// Semantic output types for model operations
	MediaAvailabilityOutput = "media:model-availability;textable;form=map"
	MediaPathOutput         = "media:model-path;textable;form=map"
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

// MediaSpecDef represents a media spec definition - always a structured object
// The Urn field identifies the media spec within a cap's media_specs array
type MediaSpecDef struct {
	Urn         string                 `json:"urn"`
	MediaType   string                 `json:"media_type"`
	ProfileURI  string                 `json:"profile_uri,omitempty"`
	Schema      interface{}            `json:"schema,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Validation  *MediaValidation       `json:"validation,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Extensions  []string               `json:"extensions,omitempty"`
}

// NewMediaSpecDef creates a media spec def with required fields
func NewMediaSpecDef(urn, mediaType, profileURI string) MediaSpecDef {
	return MediaSpecDef{
		Urn:        urn,
		MediaType:  mediaType,
		ProfileURI: profileURI,
	}
}

// NewMediaSpecDefWithTitle creates a media spec def with title
func NewMediaSpecDefWithTitle(urn, mediaType, profileURI, title string) MediaSpecDef {
	return MediaSpecDef{
		Urn:        urn,
		MediaType:  mediaType,
		ProfileURI: profileURI,
		Title:      title,
	}
}

// NewMediaSpecDefWithSchema creates a media spec def with schema
func NewMediaSpecDefWithSchema(urn, mediaType, profileURI string, schema interface{}) MediaSpecDef {
	return MediaSpecDef{
		Urn:        urn,
		MediaType:  mediaType,
		ProfileURI: profileURI,
		Schema:     schema,
	}
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
	Metadata map[string]interface{}
	// Extensions are the file extensions for storing this media type (e.g., ["pdf"], ["jpg", "jpeg"])
	Extensions []string
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

// IsImage returns true if the "image" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsImage() bool {
	return HasMediaUrnTag(r.SpecID, "image")
}

// IsAudio returns true if the "audio" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsAudio() bool {
	return HasMediaUrnTag(r.SpecID, "audio")
}

// IsVideo returns true if the "video" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsVideo() bool {
	return HasMediaUrnTag(r.SpecID, "video")
}

// IsNumeric returns true if the "numeric" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsNumeric() bool {
	return HasMediaUrnTag(r.SpecID, "numeric")
}

// IsBool returns true if the "bool" marker tag is present in the source media URN.
func (r *ResolvedMediaSpec) IsBool() bool {
	return HasMediaUrnTag(r.SpecID, "bool")
}

// HasMediaUrnTag checks if a media URN has a marker tag (e.g., bytes, json, textable).
// Uses tagged-urn parsing for proper tag detection.
// Requires a valid, non-empty media URN - panics otherwise.
func HasMediaUrnTag(mediaUrn, tagName string) bool {
	if mediaUrn == "" {
		panic("HasMediaUrnTag called with empty mediaUrn - this indicates the MediaSpec was not resolved via ResolveMediaUrn")
	}
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse media URN '%s': %v - this indicates invalid data", mediaUrn, err))
	}
	_, exists := parsed.GetTag(tagName)
	return exists
}

// HasMediaUrnTagValue checks if a media URN has a tag with a specific value (e.g., form=map).
// Uses tagged-urn parsing for proper tag detection.
// Requires a valid, non-empty media URN - panics otherwise.
func HasMediaUrnTagValue(mediaUrn, tagKey, tagValue string) bool {
	if mediaUrn == "" {
		panic("HasMediaUrnTagValue called with empty mediaUrn - this indicates the MediaSpec was not resolved via ResolveMediaUrn")
	}
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse media URN '%s': %v - this indicates invalid data", mediaUrn, err))
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
	ErrInvalidMediaUrn      = &MediaSpecError{"invalid media URN - must start with 'media:'"}
	ErrDuplicateMediaUrn    = &MediaSpecError{"duplicate media URN in media_specs array"}
)

// NewUnresolvableMediaUrnError creates an error for unresolvable media URNs
func NewUnresolvableMediaUrnError(mediaUrn string) error {
	return &MediaSpecError{
		Message: fmt.Sprintf("media URN '%s' cannot be resolved - not found in media_specs", mediaUrn),
	}
}

// NewDuplicateMediaUrnError creates an error for duplicate URNs in media_specs
func NewDuplicateMediaUrnError(mediaUrn string) error {
	return &MediaSpecError{
		Message: fmt.Sprintf("duplicate media URN '%s' in media_specs array", mediaUrn),
	}
}

// ValidateNoMediaSpecDuplicates checks for duplicate URNs in the media_specs array
func ValidateNoMediaSpecDuplicates(mediaSpecs []MediaSpecDef) error {
	seen := make(map[string]bool)
	for _, spec := range mediaSpecs {
		if seen[spec.Urn] {
			return NewDuplicateMediaUrnError(spec.Urn)
		}
		seen[spec.Urn] = true
	}
	return nil
}

// ResolveMediaUrn resolves a media URN to a ResolvedMediaSpec
//
// This is the SINGLE resolution path for all media URN lookups.
//
// Resolution order (matches Rust implementation):
//  1. Cap's local media_specs array (HIGHEST - cap-specific definitions)
//  2. Registry's bundled standard specs
//  3. (Future: Registry's cache and online fetch)
//  4. If none resolve → FAIL HARD
//
// Arguments:
//   - mediaUrn: The media URN to resolve (e.g., "media:textable;form=scalar")
//   - mediaSpecs: Optional media_specs array from the cap definition (nil = none)
//   - registry: The MediaUrnRegistry for standard spec lookups
//
// Returns:
//   - ResolvedMediaSpec if found
//   - Error if media URN cannot be resolved from any source
func ResolveMediaUrn(mediaUrn string, mediaSpecs []MediaSpecDef, registry *MediaUrnRegistry) (*ResolvedMediaSpec, error) {
	// Validate it's a media URN
	if !strings.HasPrefix(mediaUrn, "media:") {
		return nil, ErrInvalidMediaUrn
	}

	// 1. First, try cap's local media_specs (highest priority - cap-specific definitions)
	if mediaSpecs != nil {
		for i := range mediaSpecs {
			if mediaSpecs[i].Urn == mediaUrn {
				return resolveMediaSpecDef(&mediaSpecs[i])
			}
		}
	}

	// 2. Try registry (checks bundled standard specs, then cache, then online)
	if registry != nil {
		storedSpec, err := registry.GetMediaSpec(mediaUrn)
		if err == nil {
			return &ResolvedMediaSpec{
				SpecID:      mediaUrn,
				MediaType:   storedSpec.MediaType,
				ProfileURI:  storedSpec.ProfileURI,
				Schema:      storedSpec.Schema,
				Title:       storedSpec.Title,
				Description: storedSpec.Description,
				Validation:  storedSpec.Validation,
				Metadata:    storedSpec.Metadata,
				Extensions:  storedSpec.Extensions,
			}, nil
		}
		// Registry lookup failed - log warning and continue to error
		fmt.Printf("[WARN] Media URN '%s' not found in registry: %v - "+
			"ensure it's defined in capns_dot_org/standard/media/\n",
			mediaUrn, err)
	}

	// Fail - not found in any source
	return nil, &MediaSpecError{
		Message: fmt.Sprintf("cannot resolve media URN '%s' - not found in cap's media_specs or registry", mediaUrn),
	}
}

// resolveMediaSpecDef resolves a MediaSpecDef to a ResolvedMediaSpec
func resolveMediaSpecDef(def *MediaSpecDef) (*ResolvedMediaSpec, error) {
	return &ResolvedMediaSpec{
		SpecID:      def.Urn,
		MediaType:   def.MediaType,
		ProfileURI:  def.ProfileURI,
		Schema:      def.Schema,
		Title:       def.Title,
		Description: def.Description,
		Validation:  def.Validation,
		Metadata:    def.Metadata,
		Extensions:  def.Extensions,
	}, nil
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
func GetMediaSpecFromCapUrn(urn *urn.CapUrn, mediaSpecs []MediaSpecDef, registry *MediaUrnRegistry) (*ResolvedMediaSpec, error) {
	outUrn := urn.OutSpec()
	if outUrn == "" {
		return nil, errors.New("no 'out' tag found in cap URN")
	}
	return ResolveMediaUrn(outUrn, mediaSpecs, registry)
}
