// Package capdef provides the unified capability-based manifest interface
package capdef

// CapabilityManifest represents unified capability manifest for --manifest output
type CapabilityManifest struct {
	// Component name
	Name string `json:"name"`

	// Component version
	Version string `json:"version"`

	// Component description
	Description string `json:"description"`

	// Component capabilities with formal definitions
	Capabilities []Capability `json:"capabilities"`

	// Component author/maintainer
	Author *string `json:"author,omitempty"`
}

// NewCapabilityManifest creates a new capability manifest
func NewCapabilityManifest(name, version, description string, capabilities []Capability) *CapabilityManifest {
	return &CapabilityManifest{
		Name:         name,
		Version:      version,
		Description:  description,
		Capabilities: capabilities,
	}
}

// WithAuthor sets the author of the component
func (cm *CapabilityManifest) WithAuthor(author string) *CapabilityManifest {
	cm.Author = &author
	return cm
}

// ComponentMetadata interface for components to provide metadata about themselves
type ComponentMetadata interface {
	// ComponentManifest returns the component manifest
	ComponentManifest() *CapabilityManifest

	// Capabilities returns the component capabilities
	Capabilities() []Capability
}