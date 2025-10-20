package capdef

import (
	"encoding/json"
)

// Capability represents a formal capability definition
//
// This defines the structure for formal capability definitions that include
// the capability identifier, versioning, and metadata. Capabilities are general-purpose
// and do not assume any specific domain like files or documents.
type Capability struct {
	// Id is the formal capability identifier with hierarchical naming
	Id *CapabilityId `json:"id"`

	// Version is the capability version
	Version string `json:"version"`

	// Description is an optional description
	Description *string `json:"description,omitempty"`

	// Metadata contains optional metadata as key-value pairs
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewCapability creates a new capability
func NewCapability(id *CapabilityId, version string) *Capability {
	return &Capability{
		Id:       id,
		Version:  version,
		Metadata: make(map[string]string),
	}
}

// NewCapabilityWithDescription creates a new capability with description
func NewCapabilityWithDescription(id *CapabilityId, version string, description string) *Capability {
	return &Capability{
		Id:          id,
		Version:     version,
		Description: &description,
		Metadata:    make(map[string]string),
	}
}

// NewCapabilityWithMetadata creates a new capability with metadata
func NewCapabilityWithMetadata(id *CapabilityId, version string, metadata map[string]string) *Capability {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Capability{
		Id:       id,
		Version:  version,
		Metadata: metadata,
	}
}

// NewCapabilityWithDescriptionAndMetadata creates a new capability with description and metadata
func NewCapabilityWithDescriptionAndMetadata(id *CapabilityId, version string, description string, metadata map[string]string) *Capability {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Capability{
		Id:          id,
		Version:     version,
		Description: &description,
		Metadata:    metadata,
	}
}

// MatchesRequest checks if this capability matches a request string
func (c *Capability) MatchesRequest(request string) bool {
	requestId, err := NewCapabilityIdFromString(request)
	if err != nil {
		return false
	}
	return c.Id.CanHandle(requestId)
}

// CanHandleRequest checks if this capability can handle a request
func (c *Capability) CanHandleRequest(request *CapabilityId) bool {
	return c.Id.CanHandle(request)
}

// IsMoreSpecificThan checks if this capability is more specific than another
func (c *Capability) IsMoreSpecificThan(other *Capability) bool {
	if other == nil {
		return true
	}
	return c.Id.IsMoreSpecificThan(other.Id)
}

// GetMetadata gets a metadata value by key
func (c *Capability) GetMetadata(key string) (string, bool) {
	if c.Metadata == nil {
		return "", false
	}
	value, exists := c.Metadata[key]
	return value, exists
}

// SetMetadata sets a metadata value
func (c *Capability) SetMetadata(key, value string) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	c.Metadata[key] = value
}

// RemoveMetadata removes a metadata value
func (c *Capability) RemoveMetadata(key string) bool {
	if c.Metadata == nil {
		return false
	}
	_, exists := c.Metadata[key]
	if exists {
		delete(c.Metadata, key)
	}
	return exists
}

// HasMetadata checks if this capability has specific metadata
func (c *Capability) HasMetadata(key string) bool {
	if c.Metadata == nil {
		return false
	}
	_, exists := c.Metadata[key]
	return exists
}

// IdString gets the capability identifier as a string
func (c *Capability) IdString() string {
	return c.Id.ToString()
}

// Equals checks if this capability is equal to another
func (c *Capability) Equals(other *Capability) bool {
	if other == nil {
		return false
	}

	if !c.Id.Equals(other.Id) {
		return false
	}

	if c.Version != other.Version {
		return false
	}

	if (c.Description == nil) != (other.Description == nil) {
		return false
	}

	if c.Description != nil && *c.Description != *other.Description {
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

	return true
}

// MarshalJSON implements custom JSON marshaling
func (c *Capability) MarshalJSON() ([]byte, error) {
	type CapabilityAlias Capability
	return json.Marshal((*CapabilityAlias)(c))
}

// UnmarshalJSON implements custom JSON unmarshaling
func (c *Capability) UnmarshalJSON(data []byte) error {
	type CapabilityAlias Capability
	aux := (*CapabilityAlias)(c)
	return json.Unmarshal(data, aux)
}