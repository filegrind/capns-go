package capns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper for registry tests
func regTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:type=void;v=1";out="media:type=object;v=1"`
	}
	return `cap:in="media:type=void;v=1";out="media:type=object;v=1";` + tags
}

func TestRegistryCreation(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)
	assert.NotNil(t, registry)
}

func TestRegistryGetCap(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)

	// Test with a fake URN that won't exist (still needs in/out)
	testUrn := regTestUrn("op=test;target=fake")

	_, err = registry.GetCap(testUrn)
	// Should get an error since the cap doesn't exist
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in registry")
}

func TestRegistryValidation(t *testing.T) {
	registry, err := NewCapRegistry()
	require.NoError(t, err)

	// Create a test cap
	capUrn, err := NewCapUrnFromString(regTestUrn("op=test;target=fake"))
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
	exists := registry.CapExists(regTestUrn("op=nonexistent;target=fake"))
	assert.False(t, exists)
}
