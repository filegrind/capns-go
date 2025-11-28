// Package capns provides the unified cap-based manifest interface
package capns

// CapManifest represents unified cap manifest for --manifest output
type CapManifest struct {
	// Component name
	Name string `json:"name"`

	// Component version
	Version string `json:"version"`

	// Component description
	Description string `json:"description"`

	// Component caps with formal definitions
	Caps []Cap `json:"caps"`

	// Component author/maintainer
	Author *string `json:"author,omitempty"`
}

// NewCapManifest creates a new cap manifest
func NewCapManifest(name, version, description string, caps []Cap) *CapManifest {
	return &CapManifest{
		Name:         name,
		Version:      version,
		Description:  description,
		Caps: caps,
	}
}

// WithAuthor sets the author of the component
func (cm *CapManifest) WithAuthor(author string) *CapManifest {
	cm.Author = &author
	return cm
}

// ComponentMetadata interface for components to provide metadata about themselves
type ComponentMetadata interface {
	// ComponentManifest returns the component manifest
	ComponentManifest() *CapManifest

	// Caps returns the component caps
	Caps() []Cap
}