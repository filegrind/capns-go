package capns

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryCreation(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)
	assert.NotNil(t, registry)
}

func TestRegistryGetCap(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)

	// Test with a fake URN that won't exist
	testUrn := "cap:op=test;target=fake"
	
	_, err = registry.GetCap(testUrn)
	// Should get an error since the cap doesn't exist
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in registry")
}

func TestRegistryValidation(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)

	// Create a test cap
	capUrn, err := NewCapUrnFromString("cap:op=test;target=fake")
	require.NoError(t, err)
	cap := NewCap(capUrn, "Test Command", "test-cmd")

	// Validation should fail since this cap doesn't exist in registry
	err = ValidateCapCanonical(registry, cap)
	assert.Error(t, err)
}

func TestCacheOperations(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)

	// Test clearing empty cache (should not error)
	err = registry.ClearCache()
	assert.NoError(t, err)
}

func TestCapExists(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)

	// Test with a URN that doesn't exist
	exists := registry.CapExists("cap:op=nonexistent;target=fake")
	assert.False(t, exists)
}