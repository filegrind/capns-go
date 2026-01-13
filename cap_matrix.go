// Package capns provides CapSet registry for unified capability host discovery
package capns

import (
	"fmt"
)

// CapMatrixError represents errors that can occur during capability host operations
type CapMatrixError struct {
	Type    string
	Message string
}

func (e *CapMatrixError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewNoSetsFoundError creates a new error for when no sets are found
func NewNoSetsFoundError(capability string) *CapMatrixError {
	return &CapMatrixError{
		Type:    "NoSetsFound",
		Message: fmt.Sprintf("No cap sets found for capability: %s", capability),
	}
}

// NewInvalidUrnError creates a new error for invalid URNs
func NewInvalidUrnError(urn, reason string) *CapMatrixError {
	return &CapMatrixError{
		Type:    "InvalidUrn",
		Message: fmt.Sprintf("Invalid capability URN: %s: %s", urn, reason),
	}
}

// NewRegistryError creates a new general registry error
func NewRegistryError(message string) *CapMatrixError {
	return &CapMatrixError{
		Type:    "RegistryError",
		Message: message,
	}
}

// capSetEntry represents a registered capability host
type capSetEntry struct {
	name         string
	host         CapSet
	capabilities []*Cap
}

// CapMatrix provides unified registry for cap sets (providers and plugins)
type CapMatrix struct {
	sets map[string]*capSetEntry
}

// NewCapMatrix creates a new empty capability host registry
func NewCapMatrix() *CapMatrix {
	return &CapMatrix{
		sets: make(map[string]*capSetEntry),
	}
}

// RegisterCapSet registers a capability host with its supported capabilities
func (r *CapMatrix) RegisterCapSet(name string, host CapSet, capabilities []*Cap) error {
	entry := &capSetEntry{
		name:         name,
		host:         host,
		capabilities: capabilities,
	}
	
	r.sets[name] = entry
	return nil
}

// FindCapSets finds cap sets that can handle the requested capability
// Uses subset matching: host capabilities must be a subset of or match the request
func (r *CapMatrix) FindCapSets(requestUrn string) ([]CapSet, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, NewInvalidUrnError(requestUrn, err.Error())
	}
	
	var matchingHosts []CapSet
	
	for _, entry := range r.sets {
		for _, cap := range entry.capabilities {
			capUrn, err := NewCapUrnFromString(cap.Urn.String())
			if err != nil {
				return nil, NewRegistryError(
					fmt.Sprintf("Invalid capability URN in registry for host %s: %s", entry.name, err.Error()),
				)
			}
			
			if capUrn.Matches(request) {
				matchingHosts = append(matchingHosts, entry.host)
				break // Found a matching capability for this host, no need to check others
			}
		}
	}
	
	if len(matchingHosts) == 0 {
		return nil, NewNoSetsFoundError(requestUrn)
	}
	
	return matchingHosts, nil
}

// FindBestCapSet finds the best capability host and cap definition for the request using specificity ranking
func (r *CapMatrix) FindBestCapSet(requestUrn string) (CapSet, *Cap, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, nil, NewInvalidUrnError(requestUrn, err.Error())
	}
	
	var bestHost CapSet
	var bestCap *Cap
	var bestSpecificity int = -1
	
	for _, entry := range r.sets {
		for _, cap := range entry.capabilities {
			capUrn, err := NewCapUrnFromString(cap.Urn.String())
			if err != nil {
				return nil, nil, NewRegistryError(
					fmt.Sprintf("Invalid capability URN in registry for host %s: %s", entry.name, err.Error()),
				)
			}
			
			if capUrn.Matches(request) {
				specificity := capUrn.Specificity()
				if bestSpecificity == -1 || specificity > bestSpecificity {
					bestHost = entry.host
					bestCap = cap
					bestSpecificity = specificity
				}
				break // Found a matching capability for this host, check next host
			}
		}
	}
	
	if bestHost == nil {
		return nil, nil, NewNoSetsFoundError(requestUrn)
	}
	
	return bestHost, bestCap, nil
}

// GetHostNames returns all registered capability host names
func (r *CapMatrix) GetHostNames() []string {
	names := make([]string, 0, len(r.sets))
	for name := range r.sets {
		names = append(names, name)
	}
	return names
}

// GetAllCapabilities returns all capabilities from all registered sets
func (r *CapMatrix) GetAllCapabilities() []*Cap {
	var capabilities []*Cap
	for _, entry := range r.sets {
		capabilities = append(capabilities, entry.capabilities...)
	}
	return capabilities
}

// CanHandle checks if any host can handle the specified capability
func (r *CapMatrix) CanHandle(requestUrn string) bool {
	_, err := r.FindCapSets(requestUrn)
	return err == nil
}

// UnregisterCapSet unregisters a capability host
func (r *CapMatrix) UnregisterCapSet(name string) bool {
	if _, exists := r.sets[name]; exists {
		delete(r.sets, name)
		return true
	}
	return false
}

// Clear removes all registered sets
func (r *CapMatrix) Clear() {
	r.sets = make(map[string]*capSetEntry)
}