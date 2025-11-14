package capdef

import (
	"encoding/json"
	"sort"
)

// PluginCapabilities manages a collection of capabilities with searching, matching, and querying functionality
type PluginCapabilities struct {
	// Capabilities is the array of capabilities
	Capabilities []*Capability `json:"capabilities"`
}

// NewPluginCapabilities creates a new empty capabilities collection
func NewPluginCapabilities() *PluginCapabilities {
	return &PluginCapabilities{
		Capabilities: make([]*Capability, 0),
	}
}

// NewPluginCapabilitiesWithArray creates capabilities collection with an array of capabilities
func NewPluginCapabilitiesWithArray(capabilities []*Capability) *PluginCapabilities {
	caps := make([]*Capability, len(capabilities))
	copy(caps, capabilities)
	return &PluginCapabilities{
		Capabilities: caps,
	}
}

// AddCapability adds a capability to the collection
func (pc *PluginCapabilities) AddCapability(capability *Capability) {
	if capability != nil {
		pc.Capabilities = append(pc.Capabilities, capability)
	}
}

// RemoveCapability removes a capability from the collection
func (pc *PluginCapabilities) RemoveCapability(capability *Capability) bool {
	for i, cap := range pc.Capabilities {
		if cap.Equals(capability) {
			pc.Capabilities = append(pc.Capabilities[:i], pc.Capabilities[i+1:]...)
			return true
		}
	}
	return false
}

// CanHandleCapability checks if the plugin has a specific capability
func (pc *PluginCapabilities) CanHandleCapability(capabilityRequest string) bool {
	for _, capability := range pc.Capabilities {
		if capability.MatchesRequest(capabilityRequest) {
			return true
		}
	}
	return false
}

// CapabilityKeys gets all capability identifiers as strings
func (pc *PluginCapabilities) CapabilityKeys() []string {
	identifiers := make([]string, len(pc.Capabilities))
	for i, capability := range pc.Capabilities {
		identifiers[i] = capability.IdString()
	}
	return identifiers
}

// FindCapabilityWithIdentifier finds a capability by identifier
func (pc *PluginCapabilities) FindCapabilityWithIdentifier(identifier string) *Capability {
	searchId, err := NewCapabilityKeyFromString(identifier)
	if err != nil {
		return nil
	}

	for _, capability := range pc.Capabilities {
		if capability.Id.Equals(searchId) {
			return capability
		}
	}
	return nil
}

// FindBestCapabilityForRequest finds the most specific capability that can handle a request
func (pc *PluginCapabilities) FindBestCapabilityForRequest(request string) *Capability {
	requestId, err := NewCapabilityKeyFromString(request)
	if err != nil {
		return nil
	}

	capabilityKeys := make([]*CapabilityKey, len(pc.Capabilities))
	for i, capability := range pc.Capabilities {
		capabilityKeys[i] = capability.Id
	}

	bestId := FindBestMatchStatic(capabilityKeys, requestId)
	if bestId == nil {
		return nil
	}

	for _, capability := range pc.Capabilities {
		if capability.Id.Equals(bestId) {
			return capability
		}
	}
	return nil
}

// CapabilitiesWithMetadata gets capabilities that have specific metadata
func (pc *PluginCapabilities) CapabilitiesWithMetadata(key string, value *string) []*Capability {
	var matches []*Capability

	for _, capability := range pc.Capabilities {
		if value != nil {
			if metadataValue, exists := capability.GetMetadata(key); exists && metadataValue == *value {
				matches = append(matches, capability)
			}
		} else {
			if capability.HasMetadata(key) {
				matches = append(matches, capability)
			}
		}
	}

	return matches
}

// AllMetadataKeys gets all unique metadata keys across all capabilities
func (pc *PluginCapabilities) AllMetadataKeys() []string {
	keySet := make(map[string]struct{})

	for _, capability := range pc.Capabilities {
		for key := range capability.Metadata {
			keySet[key] = struct{}{}
		}
	}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

// CapabilitiesWithVersion gets capabilities by version
func (pc *PluginCapabilities) CapabilitiesWithVersion(version string) []*Capability {
	var matches []*Capability

	for _, capability := range pc.Capabilities {
		if capability.Version == version {
			matches = append(matches, capability)
		}
	}

	return matches
}

// Count gets the count of capabilities
func (pc *PluginCapabilities) Count() int {
	return len(pc.Capabilities)
}

// IsEmpty checks if the collection is empty
func (pc *PluginCapabilities) IsEmpty() bool {
	return len(pc.Capabilities) == 0
}

// Equals checks if this capabilities collection is equal to another
func (pc *PluginCapabilities) Equals(other *PluginCapabilities) bool {
	if other == nil {
		return false
	}

	if len(pc.Capabilities) != len(other.Capabilities) {
		return false
	}

	for i, capability := range pc.Capabilities {
		if !capability.Equals(other.Capabilities[i]) {
			return false
		}
	}

	return true
}

// Clone creates a deep copy of the capabilities collection
func (pc *PluginCapabilities) Clone() *PluginCapabilities {
	return NewPluginCapabilitiesWithArray(pc.Capabilities)
}

// MarshalJSON implements custom JSON marshaling
func (pc *PluginCapabilities) MarshalJSON() ([]byte, error) {
	type PluginCapabilitiesAlias PluginCapabilities
	return json.Marshal((*PluginCapabilitiesAlias)(pc))
}

// UnmarshalJSON implements custom JSON unmarshaling
func (pc *PluginCapabilities) UnmarshalJSON(data []byte) error {
	type PluginCapabilitiesAlias PluginCapabilities
	aux := (*PluginCapabilitiesAlias)(pc)
	return json.Unmarshal(data, aux)
}