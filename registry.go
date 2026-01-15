package capns

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	RegistryBaseURL       = "https://capns.org"
	CacheDurationHours    = 24
	HTTPTimeoutSeconds    = 10
)

// CacheEntry represents a cached cap definition
type CacheEntry struct {
	Definition Cap   `json:"definition"`
	CachedAt   int64 `json:"cached_at"`
	TTLHours   int64 `json:"ttl_hours"`
}

func (e *CacheEntry) isExpired() bool {
	return time.Now().Unix() > e.CachedAt+(e.TTLHours*3600)
}

// RegistryCapResponse represents the response format from capns.org registry
type RegistryCapResponse struct {
	Urn          interface{} `json:"urn"`          // Can be string or object with tags
	Version      string      `json:"version"`
	CapDescription  *string     `json:"cap_description,omitempty"`
	Metadata     map[string]string `json:"metadata"`
	Command      string      `json:"command"`
	Arguments    CapArguments `json:"arguments"`
	Output       *CapOutput  `json:"output,omitempty"`
	AcceptsStdin bool        `json:"accepts_stdin"`
}

// ToCap converts a registry response to a standard Cap
func (r *RegistryCapResponse) ToCap() (*Cap, error) {
	// Handle URN conversion
	var capUrn *CapUrn
	var err error
	
	switch urnData := r.Urn.(type) {
	case string:
		capUrn, err = NewCapUrnFromString(urnData)
		if err != nil {
			return nil, fmt.Errorf("invalid URN string: %w", err)
		}
	case map[string]interface{}:
		if tags, ok := urnData["tags"].(map[string]interface{}); ok {
			builder := NewCapUrnBuilder()
			for key, value := range tags {
				if strVal, ok := value.(string); ok {
					builder = builder.Tag(key, strVal)
				}
			}
			capUrn, err = builder.Build()
			if err != nil {
				return nil, fmt.Errorf("invalid URN tags: %w", err)
			}
		} else {
			return nil, fmt.Errorf("invalid URN object format")
		}
	default:
		return nil, fmt.Errorf("URN must be string or object")
	}
	
	// Use description as title if available, otherwise use a default based on command
	title := "Registry Capability"
	if r.CapDescription != nil && *r.CapDescription != "" {
		title = *r.CapDescription
	}
	
	cap := NewCap(capUrn, title, r.Command)
	cap.CapDescription = r.CapDescription
	if r.Metadata != nil {
		cap.Metadata = r.Metadata
	}
	cap.Arguments = &r.Arguments
	cap.Output = r.Output
	cap.AcceptsStdin = r.AcceptsStdin
	
	return cap, nil
}

// CapRegistry handles communication with the capns registry
type CapRegistry struct {
	client     *http.Client
	cacheDir   string
	cachedCaps map[string]*Cap
	mutex      sync.RWMutex
}

// NewCapRegistry creates a new registry client
func NewCapRegistry() (*CapRegistry, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	client := &http.Client{
		Timeout: HTTPTimeoutSeconds * time.Second,
	}

	// Load all cached caps into memory
	cachedCaps, err := loadAllCachedCaps(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load cached caps: %w", err)
	}

	return &CapRegistry{
		client:     client,
		cacheDir:   cacheDir,
		cachedCaps: cachedCaps,
	}, nil
}

// GetCap gets a cap from in-memory cache or fetch from registry
func (r *CapRegistry) GetCap(urn string) (*Cap, error) {
	// Check in-memory cache first
	r.mutex.RLock()
	if cap, exists := r.cachedCaps[urn]; exists {
		r.mutex.RUnlock()
		return cap, nil
	}
	r.mutex.RUnlock()

	// Not in cache, fetch from registry and update in-memory cache
	cap, err := r.fetchFromRegistry(urn)
	if err != nil {
		return nil, err
	}

	// Update in-memory cache
	r.mutex.Lock()
	r.cachedCaps[urn] = cap
	r.mutex.Unlock()

	return cap, nil
}

// GetCaps gets multiple caps at once - fails if any cap is not available
func (r *CapRegistry) GetCaps(urns []string) ([]*Cap, error) {
	var caps []*Cap
	for _, urn := range urns {
		cap, err := r.GetCap(urn)
		if err != nil {
			return nil, err
		}
		caps = append(caps, cap)
	}
	return caps, nil
}

// ValidateCap validates a local cap against its canonical definition
func (r *CapRegistry) ValidateCap(cap *Cap) error {
	canonicalCap, err := r.GetCap(cap.UrnString())
	if err != nil {
		return err
	}

	if cap.Command != canonicalCap.Command {
		return fmt.Errorf("command mismatch. Local: %s, Canonical: %s", cap.Command, canonicalCap.Command)
	}

	if cap.AcceptsStdin != canonicalCap.AcceptsStdin {
		return fmt.Errorf("accepts_stdin mismatch. Local: %t, Canonical: %t", cap.AcceptsStdin, canonicalCap.AcceptsStdin)
	}

	return nil
}

// CapExists checks if a cap URN exists in registry (either cached or available online)
func (r *CapRegistry) CapExists(urn string) bool {
	_, err := r.GetCap(urn)
	return err == nil
}

// GetCachedCaps gets all currently cached caps from in-memory cache
func (r *CapRegistry) GetCachedCaps() []*Cap {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	caps := make([]*Cap, 0, len(r.cachedCaps))
	for _, cap := range r.cachedCaps {
		caps = append(caps, cap)
	}
	return caps
}

// ClearCache removes all cached registry definitions
func (r *CapRegistry) ClearCache() error {
	// Clear in-memory cache
	r.mutex.Lock()
	r.cachedCaps = make(map[string]*Cap)
	r.mutex.Unlock()

	// Clear filesystem cache
	if err := os.RemoveAll(r.cacheDir); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return os.MkdirAll(r.cacheDir, 0755)
}

// Private helper methods

func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	// Use standard cache location based on OS
	var cacheBase string
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		cacheBase = xdgCache
	} else {
		cacheBase = filepath.Join(homeDir, ".cache")
	}
	
	return filepath.Join(cacheBase, "capns"), nil
}

func (r *CapRegistry) cacheKey(urn string) string {
	hasher := sha256.New()
	hasher.Write([]byte(urn))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (r *CapRegistry) cacheFilePath(urn string) string {
	key := r.cacheKey(urn)
	return filepath.Join(r.cacheDir, key+".json")
}

func loadAllCachedCaps(cacheDir string) (map[string]*Cap, error) {
	caps := make(map[string]*Cap)

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return caps, nil
	}

	files, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(cacheDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read cache file %s: %w", filePath, err)
		}

		var entry CacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("failed to parse cache file %s: %w", filePath, err)
		}

		if entry.isExpired() {
			// Remove expired cache file
			os.Remove(filePath)
			continue
		}

		urn := entry.Definition.UrnString()
		caps[urn] = &entry.Definition
	}

	return caps, nil
}

func (r *CapRegistry) saveToCache(cap *Cap) error {
	urn := cap.UrnString()
	entry := CacheEntry{
		Definition: *cap,
		CachedAt:   time.Now().Unix(),
		TTLHours:   CacheDurationHours,
	}

	data, err := json.MarshalIndent(&entry, "", "  ")
	if err != nil {
		return err
	}

	cacheFile := r.cacheFilePath(urn)
	return os.WriteFile(cacheFile, data, 0644)
}

func (r *CapRegistry) fetchFromRegistry(urn string) (*Cap, error) {
	// Normalize the cap URN using the proper parser
	normalizedUrn := urn
	if parsed, err := NewCapUrnFromString(urn); err == nil {
		normalizedUrn = parsed.String()
	}

	// URL-encode only the tags part (after "cap:") while keeping "cap:" literal
	tagsPart := strings.TrimPrefix(normalizedUrn, "cap:")
	encodedTags := url.PathEscape(tagsPart)
	registryURL := fmt.Sprintf("%s/cap:%s", RegistryBaseURL, encodedTags)
	resp, err := r.client.Get(registryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("cap '%s' not found in registry (HTTP %d)", urn, resp.StatusCode)
		}
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the registry response format
	var registryResp RegistryCapResponse
	if err := json.Unmarshal(body, &registryResp); err != nil {
		return nil, fmt.Errorf("failed to parse registry response for '%s': %w", urn, err)
	}

	// Convert to Cap format
	cap, err := registryResp.ToCap()
	if err != nil {
		return nil, fmt.Errorf("failed to convert registry response to cap for '%s': %w", urn, err)
	}

	// Cache the result
	if err := r.saveToCache(cap); err != nil {
		return nil, fmt.Errorf("failed to cache cap: %w", err)
	}

	return cap, nil
}

// Validation functions

// ValidateCapCanonical validates a cap against its canonical definition
func ValidateCapCanonical(registry *CapRegistry, cap *Cap) error {
	return registry.ValidateCap(cap)
}