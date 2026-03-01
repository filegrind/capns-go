// Package capns provides MediaSpec parsing and media URN resolution
//
// Media URNs reference media type definitions in the media_specs array.
// Format: `media:<type>` with optional tags.
//
// Examples:
// - `media:textable`
// - `media:pdf`
//
// MediaSpecDef is always a structured object - NO string form parsing.
package media

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/machinefabric/capns-go/urn"
	taggedurn "github.com/machinefabric/tagged-urn-go"
)

// Built-in media URN constants with coercion tags
const (
	MediaVoid         = "media:void"
	MediaString       = "media:textable"
	MediaInteger      = "media:integer;textable;numeric"
	MediaNumber       = "media:textable;numeric"
	MediaBoolean      = "media:bool;textable"
	MediaObject       = "media:record;textable"
	MediaBinary       = "media:"
	MediaStringArray  = "media:textable;list"
	MediaIntegerArray = "media:integer;textable;numeric;list"
	MediaNumberArray  = "media:textable;numeric;list"
	MediaBooleanArray = "media:bool;textable;list"
	MediaObjectArray  = "media:list;textable"
	// Semantic content types
	MediaImage = "media:image;png"
	MediaAudio = "media:wav;audio"
	MediaVideo = "media:video"
	// Semantic AI input types
	MediaAudioSpeech    = "media:audio;wav;speech"
	MediaImageThumbnail = "media:image;png;thumbnail"
	// Document types (PRIMARY naming - type IS the format)
	MediaPdf  = "media:pdf"
	MediaEpub = "media:epub"
	// Text format types (PRIMARY naming - type IS the format)
	MediaMd         = "media:md;textable"
	MediaTxt        = "media:txt;textable"
	MediaRst        = "media:rst;textable"
	MediaLog        = "media:log;textable"
	MediaHtml       = "media:html;textable"
	MediaXml        = "media:xml;textable"
	MediaJson       = "media:json;textable;record"
	MediaJsonSchema = "media:json;json-schema;textable;record"
	MediaYaml       = "media:yaml;textable;record"
	// Semantic input types
	MediaModelSpec = "media:model-spec;textable"
	MediaModelRepo = "media:model-repo;textable;record"
	// File path types
	MediaFilePath      = "media:file-path;textable"
	MediaFilePathArray = "media:file-path;textable;list"
	// Semantic output types
	MediaModelDim      = "media:model-dim;integer;textable;numeric"
	MediaDecision      = "media:decision;bool;textable"
	MediaDecisionArray = "media:decision;bool;textable;list"
	// Semantic output types
	MediaLlmInferenceOutput = "media:generated-text;textable;record"
	// Semantic output types for model operations
	MediaAvailabilityOutput = "media:model-availability;textable;record"
	MediaPathOutput         = "media:model-path;textable;record"
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

// IsBinary returns true if the "textable" marker tag is NOT present in the source media URN.
func (r *ResolvedMediaSpec) IsBinary() bool {
	return !HasMediaUrnTag(r.SpecID, "textable")
}

// IsRecord returns true if record marker tag is present (has internal key-value structure).
func (r *ResolvedMediaSpec) IsRecord() bool {
	return HasMediaUrnMarkerTag(r.SpecID, "record")
}

// IsOpaque returns true if no record marker is present (opaque = default structure).
func (r *ResolvedMediaSpec) IsOpaque() bool {
	return !r.IsRecord()
}

// IsScalar returns true if no list marker is present (scalar = default cardinality).
func (r *ResolvedMediaSpec) IsScalar() bool {
	return !r.IsList()
}

// IsList returns true if list marker tag is present (array/list cardinality).
func (r *ResolvedMediaSpec) IsList() bool {
	return HasMediaUrnMarkerTag(r.SpecID, "list")
}

// IsJSON returns true if the "json" marker tag is present in the source media URN.
// Note: This checks for JSON representation specifically, not record structure (use IsRecord for that).
func (r *ResolvedMediaSpec) IsJSON() bool {
	return HasMediaUrnTag(r.SpecID, "json")
}

// IsStructured returns true if this represents structured data (has record marker).
// Structured data has internal key-value fields that can be accessed.
// Note: This does NOT check for the explicit `json` tag - use IsJSON() for that.
func (r *ResolvedMediaSpec) IsStructured() bool {
	return r.IsRecord()
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

// HasMediaUrnTag checks if a media URN has a marker tag (e.g., json, textable).
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

// HasMediaUrnTagValue checks if a media URN has a tag with a specific value (e.g., record).
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

// HasMediaUrnMarkerTag checks if a media URN has a marker tag (tag with wildcard value "*").
// Marker tags are used for boolean flags like `list` and `record`.
// Uses tagged-urn parsing for proper tag detection.
// Requires a valid, non-empty media URN - panics otherwise.
func HasMediaUrnMarkerTag(mediaUrn, tagName string) bool {
	if mediaUrn == "" {
		panic("HasMediaUrnMarkerTag called with empty mediaUrn - this indicates the MediaSpec was not resolved via ResolveMediaUrn")
	}
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse media URN '%s': %v - this indicates invalid data", mediaUrn, err))
	}
	value, exists := parsed.GetTag(tagName)
	return exists && value == "*"
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
//   - mediaUrn: The media URN to resolve (e.g., "media:textable")
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
			"ensure it's defined in capns-dot-org/standard/media/\n",
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
// Determines type based on media URN marker tags: no textable->binary, record marker->object, list marker->array, etc.
func GetTypeFromMediaUrn(mediaUrn string) string {
	// Parse the media URN to check tags
	parsed, err := taggedurn.NewTaggedUrnFromString(mediaUrn)
	if err != nil {
		return "unknown"
	}

	// Check for void
	if _, ok := parsed.GetTag("void"); ok {
		return "void"
	}

	// Check for binary (no "textable" tag)
	if _, ok := parsed.GetTag("textable"); !ok {
		return "binary"
	}

	// Check for record marker (has internal key-value structure)
	if val, ok := parsed.GetTag("record"); ok && val == "*" {
		return "object"
	}

	// Check for explicit json tag (also represents object)
	if _, ok := parsed.GetTag("json"); ok {
		return "object"
	}

	// Check for list marker (array/list cardinality)
	if val, ok := parsed.GetTag("list"); ok && val == "*" {
		return "array"
	}

	// Check specific type tags (for scalar types)
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
	// Check for record structure (has internal fields) OR explicit json tag
	if resolved.IsRecord() || resolved.IsJSON() {
		return "object"
	}
	// Check for list structure (list)
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
