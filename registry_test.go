package capns

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper for registry tests
func regTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:void";out="media:object"`
	}
	return `cap:in="media:void";out="media:object";` + tags
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

// URL Encoding Tests - Guard against the bug where encoding "cap:" causes 404s

// buildRegistryURL replicates the URL construction logic from fetchFromRegistry
func buildRegistryURL(urn string) string {
	normalizedUrn := urn
	if parsed, err := NewCapUrnFromString(urn); err == nil {
		normalizedUrn = parsed.String()
	}
	tagsPart := strings.TrimPrefix(normalizedUrn, "cap:")
	encodedTags := url.PathEscape(tagsPart)
	return fmt.Sprintf("%s/cap:%s", RegistryBaseURL, encodedTags)
}

// TestURLKeepsCapPrefixLiteral tests that "cap:" is NOT URL-encoded
func TestURLKeepsCapPrefixLiteral(t *testing.T) {
	urn := `cap:in="media:string";op=test;out="media:object"`
	registryURL := buildRegistryURL(urn)

	// URL must contain literal "/cap:" not encoded
	assert.Contains(t, registryURL, "/cap:", "URL must contain literal '/cap:' not encoded")
	// URL must NOT contain "cap%3A" (encoded version)
	assert.NotContains(t, registryURL, "cap%3A", "URL must not encode 'cap:' as 'cap%3A'")
}

// TestURLEncodesMediaUrns tests that media URN values are properly handled in URLs
func TestURLEncodesMediaUrns(t *testing.T) {
	// Colons don't need quoting, so the canonical form won't have quotes
	urn := `cap:in=media:listing-id;op=use_grinder;out=media:task-id`
	registryURL := buildRegistryURL(urn)

	// URL should contain the media URN values
	assert.Contains(t, registryURL, "media:listing-id", "URL should contain media URN")
	// Note: url.PathEscape doesn't encode =, :, or ; as they're valid in paths
	// The key requirement is that the URL is valid and the Netlify function can decode it
}

// TestURLFormatIsValid tests the URL format is valid and can be parsed
func TestURLFormatIsValid(t *testing.T) {
	// Colons don't need quoting, so the canonical form won't have quotes
	urn := `cap:in=media:listing-id;op=use_grinder;out=media:task-id`
	registryURL := buildRegistryURL(urn)

	// URL should be parseable
	parsed, err := url.Parse(registryURL)
	require.NoError(t, err, "Generated URL must be valid")

	// Host should be capns.org
	assert.Equal(t, "capns.org", parsed.Host, "Host must be capns.org")

	// Raw URL string should start with the correct base
	assert.True(t, strings.HasPrefix(registryURL, RegistryBaseURL+"/cap:"), "URL must start with base URL and /cap:")
}

// TestNormalizeHandlesDifferentTagOrders tests that different tag orders normalize to the same URL
func TestNormalizeHandlesDifferentTagOrders(t *testing.T) {
	urn1 := `cap:op=test;in="media:string";out="media:object"`
	urn2 := `cap:in="media:string";out="media:object";op=test`

	url1 := buildRegistryURL(urn1)
	url2 := buildRegistryURL(urn2)

	assert.Equal(t, url1, url2, "Different tag orders should produce the same URL")
}
