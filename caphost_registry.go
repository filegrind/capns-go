// Package capns provides CapHost registry for unified capability host discovery
package capns

import (
	"fmt"
)

// CapHostRegistryError represents errors that can occur during capability host operations
type CapHostRegistryError struct {
	Type    string
	Message string
}

func (e *CapHostRegistryError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewNoHostsFoundError creates a new error for when no hosts are found
func NewNoHostsFoundError(capability string) *CapHostRegistryError {
	return &CapHostRegistryError{
		Type:    "NoHostsFound",
		Message: fmt.Sprintf("No capability hosts found for capability: %s", capability),
	}
}

// NewInvalidUrnError creates a new error for invalid URNs
func NewInvalidUrnError(urn, reason string) *CapHostRegistryError {
	return &CapHostRegistryError{
		Type:    "InvalidUrn",
		Message: fmt.Sprintf("Invalid capability URN: %s: %s", urn, reason),
	}
}

// NewRegistryError creates a new general registry error
func NewRegistryError(message string) *CapHostRegistryError {
	return &CapHostRegistryError{
		Type:    "RegistryError",
		Message: message,
	}
}

// capHostEntry represents a registered capability host
type capHostEntry struct {
	name         string
	host         CapHost
	capabilities []*Cap
}

// CapHostRegistry provides unified registry for capability hosts (providers and plugins)
type CapHostRegistry struct {
	hosts map[string]*capHostEntry
}

// NewCapHostRegistry creates a new empty capability host registry
func NewCapHostRegistry() *CapHostRegistry {
	return &CapHostRegistry{
		hosts: make(map[string]*capHostEntry),
	}
}

// RegisterCapHost registers a capability host with its supported capabilities
func (r *CapHostRegistry) RegisterCapHost(name string, host CapHost, capabilities []*Cap) error {
	entry := &capHostEntry{
		name:         name,
		host:         host,
		capabilities: capabilities,
	}
	
	r.hosts[name] = entry
	return nil
}

// FindCapHosts finds capability hosts that can handle the requested capability
// Uses subset matching: host capabilities must be a subset of or match the request
func (r *CapHostRegistry) FindCapHosts(requestUrn string) ([]CapHost, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, NewInvalidUrnError(requestUrn, err.Error())
	}
	
	var matchingHosts []CapHost
	
	for _, entry := range r.hosts {
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
		return nil, NewNoHostsFoundError(requestUrn)
	}
	
	return matchingHosts, nil
}

// FindBestCapHost finds the best capability host and cap definition for the request using specificity ranking
func (r *CapHostRegistry) FindBestCapHost(requestUrn string) (CapHost, *Cap, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, nil, NewInvalidUrnError(requestUrn, err.Error())
	}
	
	var bestHost CapHost
	var bestCap *Cap
	var bestSpecificity int = -1
	
	for _, entry := range r.hosts {
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
		return nil, nil, NewNoHostsFoundError(requestUrn)
	}
	
	return bestHost, bestCap, nil
}

// GetHostNames returns all registered capability host names
func (r *CapHostRegistry) GetHostNames() []string {
	names := make([]string, 0, len(r.hosts))
	for name := range r.hosts {
		names = append(names, name)
	}
	return names
}

// GetAllCapabilities returns all capabilities from all registered hosts
func (r *CapHostRegistry) GetAllCapabilities() []*Cap {
	var capabilities []*Cap
	for _, entry := range r.hosts {
		capabilities = append(capabilities, entry.capabilities...)
	}
	return capabilities
}

// CanHandle checks if any host can handle the specified capability
func (r *CapHostRegistry) CanHandle(requestUrn string) bool {
	_, err := r.FindCapHosts(requestUrn)
	return err == nil
}

// UnregisterCapHost unregisters a capability host
func (r *CapHostRegistry) UnregisterCapHost(name string) bool {
	if _, exists := r.hosts[name]; exists {
		delete(r.hosts, name)
		return true
	}
	return false
}

// Clear removes all registered hosts
func (r *CapHostRegistry) Clear() {
	r.hosts = make(map[string]*capHostEntry)
}