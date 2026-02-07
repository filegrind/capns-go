package capns

import (
	"encoding/json"
	"strings"

	taggedurn "github.com/filegrind/tagged-urn-go"
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

// IsBinary returns true if this represents binary data (has "bytes" tag)
func (m *MediaUrn) IsBinary() bool {
	return m.HasTag("bytes")
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

// GetForm returns the form tag value (scalar, map, list) if present
func (m *MediaUrn) GetForm() (string, bool) {
	return m.GetTag("form")
}

// IsScalar returns true if form=scalar
func (m *MediaUrn) IsScalar() bool {
	form, ok := m.GetForm()
	return ok && form == "scalar"
}

// IsMap returns true if form=map
func (m *MediaUrn) IsMap() bool {
	form, ok := m.GetForm()
	return ok && form == "map"
}

// IsList returns true if form=list
func (m *MediaUrn) IsList() bool {
	form, ok := m.GetForm()
	return ok && form == "list"
}

// IsStructured returns true for map or list forms
func (m *MediaUrn) IsStructured() bool {
	return m.IsMap() || m.IsList()
}

// GetExtension returns the ext tag value if present
func (m *MediaUrn) GetExtension() (string, bool) {
	return m.GetTag("ext")
}

// Built-in media URN constructors matching Rust

// MediaUrnVoid creates a void media URN
func MediaUrnVoid() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaVoid)
	return m
}

// MediaUrnString creates a string media URN
func MediaUrnString() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaString)
	return m
}

// MediaUrnBytes creates a bytes media URN
func MediaUrnBytes() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaBinary)
	return m
}

// MediaUrnObject creates an object media URN
func MediaUrnObject() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaObject)
	return m
}

// MediaUrnInteger creates an integer media URN
func MediaUrnInteger() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaInteger)
	return m
}

// MediaUrnNumber creates a number media URN
func MediaUrnNumber() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaNumber)
	return m
}

// MediaUrnBoolean creates a boolean media URN
func MediaUrnBoolean() *MediaUrn {
	m, _ := NewMediaUrnFromString(MediaBoolean)
	return m
}
