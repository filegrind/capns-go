// Package capns provides MediaSpec parsing and handling
//
// Parses media_spec values in the format:
// `content-type: <mime-type>; profile=<url>`
//
// Examples:
// - `content-type: application/json; profile="https://capns.org/schema/document-outline"`
// - `content-type: image/png; profile="https://capns.org/schema/thumbnail-image"`
// - `content-type: text/plain; profile=https://capns.org/schema/utf8-text`
package capns

import (
	"errors"
	"fmt"
	"strings"
)

// MediaSpec represents a parsed media_spec value
type MediaSpec struct {
	// ContentType is the MIME content type (e.g., "application/json", "image/png")
	ContentType string
	// Profile is the optional profile URL
	Profile string
}

// MediaSpecError represents an error parsing a media_spec
type MediaSpecError struct {
	Message string
}

func (e *MediaSpecError) Error() string {
	return e.Message
}

var (
	ErrMissingContentType = &MediaSpecError{"media_spec must start with 'content-type:'"}
	ErrEmptyContentType   = &MediaSpecError{"content-type value cannot be empty"}
	ErrUnterminatedQuote  = &MediaSpecError{"unterminated quote in profile value"}
)

// ParseMediaSpec parses a media_spec string
// Format: `content-type: <mime-type>; profile=<url>`
func ParseMediaSpec(s string) (*MediaSpec, error) {
	s = strings.TrimSpace(s)

	// Must start with "content-type:" (case-insensitive)
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "content-type:") {
		return nil, ErrMissingContentType
	}

	// Get everything after "content-type:"
	afterPrefix := strings.TrimSpace(s[13:])

	// Split by semicolon to separate mime type from parameters
	parts := strings.SplitN(afterPrefix, ";", 2)

	contentType := strings.TrimSpace(parts[0])
	if contentType == "" {
		return nil, ErrEmptyContentType
	}

	// Parse profile if present
	var profile string
	if len(parts) > 1 {
		params := strings.TrimSpace(parts[1])
		var err error
		profile, err = parseProfile(params)
		if err != nil {
			return nil, err
		}
	}

	return &MediaSpec{
		ContentType: contentType,
		Profile:     profile,
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
		// Find closing quote
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

// IsBinary returns true if this media spec represents binary output
func (m *MediaSpec) IsBinary() bool {
	ct := strings.ToLower(m.ContentType)

	// Binary content types
	return strings.HasPrefix(ct, "image/") ||
		strings.HasPrefix(ct, "audio/") ||
		strings.HasPrefix(ct, "video/") ||
		ct == "application/octet-stream" ||
		ct == "application/pdf" ||
		strings.HasPrefix(ct, "application/x-") ||
		strings.Contains(ct, "+zip") ||
		strings.Contains(ct, "+gzip")
}

// IsJSON returns true if this media spec represents JSON output
func (m *MediaSpec) IsJSON() bool {
	ct := strings.ToLower(m.ContentType)
	return ct == "application/json" || strings.HasSuffix(ct, "+json")
}

// IsText returns true if this media spec represents text output
func (m *MediaSpec) IsText() bool {
	ct := strings.ToLower(m.ContentType)
	return strings.HasPrefix(ct, "text/") || (!m.IsBinary() && !m.IsJSON())
}

// PrimaryType returns the primary type (e.g., "image" from "image/png")
func (m *MediaSpec) PrimaryType() string {
	parts := strings.SplitN(m.ContentType, "/", 2)
	return parts[0]
}

// Subtype returns the subtype (e.g., "png" from "image/png")
func (m *MediaSpec) Subtype() string {
	parts := strings.SplitN(m.ContentType, "/", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// String returns the canonical string representation
func (m *MediaSpec) String() string {
	if m.Profile != "" {
		return fmt.Sprintf("content-type: %s; profile=\"%s\"", m.ContentType, m.Profile)
	}
	return fmt.Sprintf("content-type: %s", m.ContentType)
}

// IsBinaryMediaSpec checks if a media_spec string represents binary output
func IsBinaryMediaSpec(mediaSpec string) bool {
	ms, err := ParseMediaSpec(mediaSpec)
	if err != nil {
		return false
	}
	return ms.IsBinary()
}

// IsJSONMediaSpec checks if a media_spec string represents JSON output
func IsJSONMediaSpec(mediaSpec string) bool {
	ms, err := ParseMediaSpec(mediaSpec)
	if err != nil {
		return false
	}
	return ms.IsJSON()
}

// GetMediaSpecFromCapUrn extracts media_spec from a CapUrn using the 'out' tag
func GetMediaSpecFromCapUrn(urn *CapUrn) (*MediaSpec, error) {
	if spec, exists := urn.GetTag("out"); exists {
		return ParseMediaSpec(spec)
	}

	return nil, errors.New("no 'out' tag found in cap URN")
}
