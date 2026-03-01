package urn

import (
	"encoding/json"
	"strings"

	"github.com/machinefabric/capdag-go/standard"
	taggedurn "github.com/machinefabric/tagged-urn-go"
)

// MediaUrn represents a media type URN with semantic tags
// Wraps TaggedUrn with media-specific functionality
type MediaUrn struct {
	inner *taggedurn.TaggedUrn
}

// NewMediaUrnFromString parses a media URN string
func NewMediaUrnFromString(s string) (*MediaUrn, error) {
	urn, err := taggedurn.NewTaggedUrnFromString(s)
	if err != nil {
		return nil, err
	}

	// Verify it has the "media:" prefix by checking the string representation
	urnStr := urn.String()
	if !strings.HasPrefix(strings.ToLower(urnStr), "media:") {
		return nil, &taggedurn.TaggedUrnError{
			Code:    taggedurn.ErrorPrefixMismatch,
			Message: "invalid prefix for media URN: expected 'media:'",
		}
	}

	return &MediaUrn{inner: urn}, nil
}

// String returns the canonical string representation
func (m *MediaUrn) String() string {
	if m.inner == nil {
		return ""
	}
	return m.inner.String()
}

// HasTag checks if the URN has a specific tag (presence check)
func (m *MediaUrn) HasTag(tag string) bool {
	if m.inner == nil {
		return false
	}
	_, ok := m.inner.GetTag(tag)
	return ok
}

// GetTag retrieves a tag value
func (m *MediaUrn) GetTag(tag string) (string, bool) {
	if m.inner == nil {
		return "", false
	}
	return m.inner.GetTag(tag)
}

// IsBinary returns true if this represents binary (non-text) data.
// Returns true if the "textable" marker tag is NOT present.
func (m *MediaUrn) IsBinary() bool {
	return !m.HasTag("textable")
}

// IsTextable returns true if this has the "textable" tag
func (m *MediaUrn) IsTextable() bool {
	return m.HasTag("textable")
}

// IsVoid returns true if this represents void/no data
func (m *MediaUrn) IsVoid() bool {
	return m.HasTag("void")
}

// IsJson returns true if this has the "json" tag
func (m *MediaUrn) IsJson() bool {
	return m.HasTag("json")
}

// Accepts checks if this MediaUrn (pattern/handler) accepts the given instance (request).
// Uses TaggedUrn.Accepts semantics: pattern accepts instance if instance satisfies pattern's constraints.
func (m *MediaUrn) Accepts(instance *MediaUrn) bool {
	if m.inner == nil || instance == nil || instance.inner == nil {
		return false
	}
	match, err := m.inner.Accepts(instance.inner)
	if err != nil {
		return false
	}
	return match
}

// ConformsTo checks if this MediaUrn (instance) conforms to the given pattern's constraints.
// Equivalent to pattern.Accepts(self).
func (m *MediaUrn) ConformsTo(pattern *MediaUrn) bool {
	if m.inner == nil || pattern == nil || pattern.inner == nil {
		return false
	}
	match, err := m.inner.ConformsTo(pattern.inner)
	if err != nil {
		return false
	}
	return match
}

// Equals checks if two MediaUrns are semantically equal
func (m *MediaUrn) Equals(other *MediaUrn) bool {
	if m == nil || other == nil {
		return m == other
	}
	if m.inner == nil || other.inner == nil {
		return m.inner == other.inner
	}
	return m.inner.Equals(other.inner)
}

// Specificity returns the specificity score (number of tags)
func (m *MediaUrn) Specificity() int {
	if m.inner == nil {
		return 0
	}
	return m.inner.Specificity()
}

// TagCount returns the raw number of tags (not weighted by type).
// This matches Rust's in_media.inner().tags.len() used in CapUrn specificity scoring.
func (m *MediaUrn) TagCount() int {
	if m.inner == nil {
		return 0
	}
	return len(m.inner.AllTags())
}

// MarshalJSON implements json.Marshaler
func (m *MediaUrn) MarshalJSON() ([]byte, error) {
	if m.inner == nil {
		return json.Marshal("")
	}
	return json.Marshal(m.inner.String())
}

// UnmarshalJSON implements json.Unmarshaler
func (m *MediaUrn) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if s == "" {
		m.inner = nil
		return nil
	}

	urn, err := NewMediaUrnFromString(s)
	if err != nil {
		return err
	}

	m.inner = urn.inner
	return nil
}

// Helper functions for common media URN operations

// =========================================================================
// CARDINALITY (list marker)
// =========================================================================

// IsList returns true if this media is a list (has `list` marker tag).
// Returns false if scalar (no `list` marker = default).
func (m *MediaUrn) IsList() bool {
	return m.hasMarkerTag("list")
}

// IsScalar returns true if this media is a scalar (no `list` marker).
// Scalar is the default cardinality.
func (m *MediaUrn) IsScalar() bool {
	return !m.hasMarkerTag("list")
}

// =========================================================================
// STRUCTURE (record marker)
// =========================================================================

// IsRecord returns true if this media is a record (has `record` marker tag).
// A record has internal key-value structure (e.g., JSON object).
func (m *MediaUrn) IsRecord() bool {
	return m.hasMarkerTag("record")
}

// IsOpaque returns true if this media is opaque (no `record` marker).
// Opaque is the default structure - no internal fields recognized.
func (m *MediaUrn) IsOpaque() bool {
	return !m.hasMarkerTag("record")
}

// IsStructured returns true for record data (has internal structure).
// For list detection, use IsList separately.
func (m *MediaUrn) IsStructured() bool {
	return m.IsRecord()
}

// =========================================================================
// HELPER: Check for marker tag presence
// =========================================================================

// hasMarkerTag checks if a marker tag (tag with wildcard/no value) is present.
// A marker tag is stored as key="*" in the tagged URN.
func (m *MediaUrn) hasMarkerTag(tagName string) bool {
	if m.inner == nil {
		return false
	}
	val, ok := m.inner.GetTag(tagName)
	return ok && val == "*"
}

// IsImage returns true if this has the "image" marker tag
func (m *MediaUrn) IsImage() bool {
	return m.HasTag("image")
}

// IsAudio returns true if this has the "audio" marker tag
func (m *MediaUrn) IsAudio() bool {
	return m.HasTag("audio")
}

// IsVideo returns true if this has the "video" marker tag
func (m *MediaUrn) IsVideo() bool {
	return m.HasTag("video")
}

// IsNumeric returns true if this has the "numeric" marker tag
func (m *MediaUrn) IsNumeric() bool {
	return m.HasTag("numeric")
}

// IsBool returns true if this has the "bool" marker tag
func (m *MediaUrn) IsBool() bool {
	return m.HasTag("bool")
}

// IsFilePath returns true if this has the "file-path" marker tag AND NOT IsList()
func (m *MediaUrn) IsFilePath() bool {
	return m.HasTag("file-path") && !m.IsList()
}

// IsFilePathArray returns true if this has the "file-path" marker tag AND IsList()
func (m *MediaUrn) IsFilePathArray() bool {
	return m.HasTag("file-path") && m.IsList()
}

// IsAnyFilePath returns true if this has the "file-path" marker tag (single or array)
func (m *MediaUrn) IsAnyFilePath() bool {
	return m.HasTag("file-path")
}

// GetExtension returns the ext tag value if present
func (m *MediaUrn) GetExtension() (string, bool) {
	return m.GetTag("ext")
}

// Built-in media URN constructors matching Rust

// MediaUrnVoid creates a void media URN
func MediaUrnVoid() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaVoid)
	return m
}

// MediaUrnString creates a string media URN
func MediaUrnString() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaString)
	return m
}

// MediaUrnBytes creates a binary media URN
func MediaUrnBytes() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaBinary)
	return m
}

// MediaUrnObject creates an object media URN
func MediaUrnObject() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaObject)
	return m
}

// MediaUrnInteger creates an integer media URN
func MediaUrnInteger() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaInteger)
	return m
}

// MediaUrnNumber creates a number media URN
func MediaUrnNumber() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaNumber)
	return m
}

// MediaUrnBoolean creates a boolean media URN
func MediaUrnBoolean() *MediaUrn {
	m, _ := NewMediaUrnFromString(standard.MediaBoolean)
	return m
}
