package capns

import (
	"sync"
)

// Global registry singleton for production use
// Initialized lazily on first access
var (
	globalRegistry     *MediaUrnRegistry
	globalRegistryOnce sync.Once
	globalRegistryErr  error
)

// GetGlobalRegistry returns the global media URN registry singleton
// This is created once and reused for all production code paths
// For tests, use testRegistry() to get a fresh instance
func GetGlobalRegistry() (*MediaUrnRegistry, error) {
	globalRegistryOnce.Do(func() {
		globalRegistry, globalRegistryErr = NewMediaUrnRegistry()
	})
	return globalRegistry, globalRegistryErr
}

// ResetGlobalRegistry resets the global registry (for testing only)
func ResetGlobalRegistry() {
	globalRegistry = nil
	globalRegistryErr = nil
	globalRegistryOnce = sync.Once{}
}
