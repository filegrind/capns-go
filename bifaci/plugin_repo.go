package bifaci

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PluginRepoError represents errors from plugin repository operations
type PluginRepoError struct {
	Kind    string
	Message string
}

func (e *PluginRepoError) Error() string {
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// NewHttpError creates an HTTP error
func NewHttpError(msg string) *PluginRepoError {
	return &PluginRepoError{Kind: "HttpError", Message: msg}
}

// NewParseError creates a parse error
func NewParseError(msg string) *PluginRepoError {
	return &PluginRepoError{Kind: "ParseError", Message: msg}
}

// NewStatusError creates a status error
func NewStatusError(status int) *PluginRepoError {
	return &PluginRepoError{Kind: "StatusError", Message: fmt.Sprintf("Registry request failed with status %d", status)}
}

// PluginCapSummary represents a plugin's capability summary
type PluginCapSummary struct {
	Urn         string `json:"urn"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// PluginDistributionInfo represents package or binary distribution data
type PluginDistributionInfo struct {
	Name   string `json:"name"`
	Sha256 string `json:"sha256"`
	Size   uint64 `json:"size"`
}

// PluginVersionData represents a plugin version's distribution data (v3.0 schema)
type PluginVersionData struct {
	ReleaseDate   string                 `json:"releaseDate"`
	Changelog     []string               `json:"changelog,omitempty"`
	MinAppVersion string                 `json:"minAppVersion,omitempty"`
	Platform      string                 `json:"platform"`
	Package       PluginDistributionInfo `json:"package"`
	Binary        PluginDistributionInfo `json:"binary"`
}

// PluginRegistryEntry represents a plugin entry in the v3.0 registry (nested format)
type PluginRegistryEntry struct {
	Name          string                       `json:"name"`
	Description   string                       `json:"description"`
	Author        string                       `json:"author"`
	PageUrl       string                       `json:"pageUrl,omitempty"`
	TeamId        string                       `json:"teamId"`
	MinAppVersion string                       `json:"minAppVersion,omitempty"`
	Caps          []PluginCapSummary           `json:"caps,omitempty"`
	Categories    []string                     `json:"categories,omitempty"`
	Tags          []string                     `json:"tags,omitempty"`
	LatestVersion string                       `json:"latestVersion"`
	Versions      map[string]PluginVersionData `json:"versions"`
}

// PluginRegistryV3 represents the v3.0 plugin registry (nested schema)
type PluginRegistryV3 struct {
	SchemaVersion string                         `json:"schemaVersion"`
	LastUpdated   string                         `json:"lastUpdated"`
	Plugins       map[string]PluginRegistryEntry `json:"plugins"`
}

// PluginInfo represents a plugin in the flat API response format
type PluginInfo struct {
	Id                string              `json:"id"`
	Name              string              `json:"name"`
	Version           string              `json:"version"`
	Description       string              `json:"description"`
	Author            string              `json:"author"`
	Homepage          string              `json:"homepage"`
	TeamId            string              `json:"teamId"`
	SignedAt          string              `json:"signedAt"`
	MinAppVersion     string              `json:"minAppVersion"`
	PageUrl           string              `json:"pageUrl"`
	Categories        []string            `json:"categories,omitempty"`
	Tags              []string            `json:"tags,omitempty"`
	Caps              []PluginCapSummary  `json:"caps"`
	Platform          string              `json:"platform"`
	PackageName       string              `json:"packageName"`
	PackageSha256     string              `json:"packageSha256"`
	PackageSize       uint64              `json:"packageSize"`
	BinaryName        string              `json:"binaryName"`
	BinarySha256      string              `json:"binarySha256"`
	BinarySize        uint64              `json:"binarySize"`
	Changelog         map[string][]string `json:"changelog,omitempty"`
	AvailableVersions []string            `json:"availableVersions,omitempty"`
}

// IsSigned checks if plugin is signed (has team_id and signed_at)
func (p *PluginInfo) IsSigned() bool {
	return p.TeamId != "" && p.SignedAt != ""
}

// HasBinary checks if binary download info is available
func (p *PluginInfo) HasBinary() bool {
	return p.BinaryName != "" && p.BinarySha256 != ""
}

// PluginRegistryResponse represents the plugin registry response (flat format)
type PluginRegistryResponse struct {
	Plugins []PluginInfo `json:"plugins"`
}

// PluginSuggestion represents a plugin suggestion for a missing cap
type PluginSuggestion struct {
	PluginId          string `json:"pluginId"`
	PluginName        string `json:"pluginName"`
	PluginDescription string `json:"pluginDescription"`
	CapUrn            string `json:"capUrn"`
	CapTitle          string `json:"capTitle"`
	LatestVersion     string `json:"latestVersion"`
	RepoUrl           string `json:"repoUrl"`
	PageUrl           string `json:"pageUrl"`
}

// PluginRepoServer serves registry data with queries
// Transforms v3.0 nested registry schema to flat API response format
type PluginRepoServer struct {
	registry PluginRegistryV3
}

// NewPluginRepoServer creates a new server instance from v3.0 registry
func NewPluginRepoServer(registry PluginRegistryV3) (*PluginRepoServer, error) {
	// Validate schema version - fail hard
	if registry.SchemaVersion != "3.0" {
		return nil, NewParseError(fmt.Sprintf(
			"Unsupported registry schema version: %s. Required: 3.0",
			registry.SchemaVersion,
		))
	}

	return &PluginRepoServer{registry: registry}, nil
}

// validateVersionData validates that version data has all required fields
func validateVersionData(id, version string, versionData *PluginVersionData) error {
	if versionData.Platform == "" {
		return NewParseError(fmt.Sprintf(
			"Plugin %s v%s: missing required field 'platform'",
			id, version,
		))
	}
	if versionData.Package.Name == "" {
		return NewParseError(fmt.Sprintf(
			"Plugin %s v%s: missing required field 'package.name'",
			id, version,
		))
	}
	if versionData.Binary.Name == "" {
		return NewParseError(fmt.Sprintf(
			"Plugin %s v%s: missing required field 'binary.name'",
			id, version,
		))
	}
	return nil
}

// compareVersions compares semantic version strings
func compareVersions(a, b string) int {
	partsA := parseVersion(a)
	partsB := parseVersion(b)

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		numA := uint32(0)
		numB := uint32(0)

		if i < len(partsA) {
			numA = partsA[i]
		}
		if i < len(partsB) {
			numB = partsB[i]
		}

		if numA < numB {
			return -1
		} else if numA > numB {
			return 1
		}
	}

	return 0
}

// parseVersion parses a version string into numeric parts
func parseVersion(v string) []uint32 {
	parts := strings.Split(v, ".")
	nums := make([]uint32, 0, len(parts))

	for _, p := range parts {
		if num, err := strconv.ParseUint(p, 10, 32); err == nil {
			nums = append(nums, uint32(num))
		}
	}

	return nums
}

// buildChangelogMap builds changelog map from versions
func buildChangelogMap(versions map[string]PluginVersionData) map[string][]string {
	changelog := make(map[string][]string)
	for version, data := range versions {
		if len(data.Changelog) > 0 {
			changelog[version] = data.Changelog
		}
	}
	return changelog
}

// TransformToPluginArray transforms registry to flat plugin array
func (s *PluginRepoServer) TransformToPluginArray() ([]PluginInfo, error) {
	plugins := make([]PluginInfo, 0, len(s.registry.Plugins))

	for id, plugin := range s.registry.Plugins {
		latestVersion := plugin.LatestVersion
		versionData, ok := plugin.Versions[latestVersion]
		if !ok {
			return nil, NewParseError(fmt.Sprintf(
				"Plugin %s: latest version %s not found in versions",
				id, latestVersion,
			))
		}

		// Validate required fields - fail hard
		if err := validateVersionData(id, latestVersion, &versionData); err != nil {
			return nil, err
		}

		// Get all versions sorted descending
		availableVersions := make([]string, 0, len(plugin.Versions))
		for version := range plugin.Versions {
			availableVersions = append(availableVersions, version)
		}
		sort.Slice(availableVersions, func(i, j int) bool {
			return compareVersions(availableVersions[i], availableVersions[j]) > 0
		})

		// Build flat plugin object
		packageUrl := fmt.Sprintf("https://filegrind.com/plugins/packages/%s", versionData.Package.Name)
		pageUrl := plugin.PageUrl
		if pageUrl == "" {
			pageUrl = packageUrl
		}

		minAppVersion := versionData.MinAppVersion
		if minAppVersion == "" {
			minAppVersion = plugin.MinAppVersion
		}

		caps := plugin.Caps
		if caps == nil {
			caps = []PluginCapSummary{}
		}

		categories := plugin.Categories
		if categories == nil {
			categories = []string{}
		}

		tags := plugin.Tags
		if tags == nil {
			tags = []string{}
		}

		pluginInfo := PluginInfo{
			Id:                id,
			Name:              plugin.Name,
			Version:           latestVersion,
			Description:       plugin.Description,
			Author:            plugin.Author,
			Homepage:          "",
			TeamId:            plugin.TeamId,
			SignedAt:          versionData.ReleaseDate,
			MinAppVersion:     minAppVersion,
			PageUrl:           pageUrl,
			Categories:        categories,
			Tags:              tags,
			Caps:              caps,
			Platform:          versionData.Platform,
			PackageName:       versionData.Package.Name,
			PackageSha256:     versionData.Package.Sha256,
			PackageSize:       versionData.Package.Size,
			BinaryName:        versionData.Binary.Name,
			BinarySha256:      versionData.Binary.Sha256,
			BinarySize:        versionData.Binary.Size,
			Changelog:         buildChangelogMap(plugin.Versions),
			AvailableVersions: availableVersions,
		}

		plugins = append(plugins, pluginInfo)
	}

	return plugins, nil
}

// GetPlugins returns all plugins (API response format)
func (s *PluginRepoServer) GetPlugins() (*PluginRegistryResponse, error) {
	plugins, err := s.TransformToPluginArray()
	if err != nil {
		return nil, err
	}
	return &PluginRegistryResponse{Plugins: plugins}, nil
}

// GetPluginById returns a plugin by ID
func (s *PluginRepoServer) GetPluginById(id string) (*PluginInfo, error) {
	plugins, err := s.TransformToPluginArray()
	if err != nil {
		return nil, err
	}

	for _, plugin := range plugins {
		if plugin.Id == id {
			return &plugin, nil
		}
	}

	return nil, nil
}

// SearchPlugins searches plugins by query
func (s *PluginRepoServer) SearchPlugins(query string) ([]PluginInfo, error) {
	plugins, err := s.TransformToPluginArray()
	if err != nil {
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	results := make([]PluginInfo, 0)

	for _, plugin := range plugins {
		// Search in name
		if strings.Contains(strings.ToLower(plugin.Name), lowerQuery) {
			results = append(results, plugin)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(plugin.Description), lowerQuery) {
			results = append(results, plugin)
			continue
		}

		// Search in tags
		found := false
		for _, tag := range plugin.Tags {
			if strings.Contains(strings.ToLower(tag), lowerQuery) {
				found = true
				break
			}
		}
		if found {
			results = append(results, plugin)
			continue
		}

		// Search in caps
		for _, cap := range plugin.Caps {
			if strings.Contains(strings.ToLower(cap.Urn), lowerQuery) ||
				strings.Contains(strings.ToLower(cap.Title), lowerQuery) {
				found = true
				break
			}
		}
		if found {
			results = append(results, plugin)
		}
	}

	return results, nil
}

// GetPluginsByCategory returns plugins by category
func (s *PluginRepoServer) GetPluginsByCategory(category string) ([]PluginInfo, error) {
	plugins, err := s.TransformToPluginArray()
	if err != nil {
		return nil, err
	}

	results := make([]PluginInfo, 0)
	for _, plugin := range plugins {
		for _, cat := range plugin.Categories {
			if cat == category {
				results = append(results, plugin)
				break
			}
		}
	}

	return results, nil
}

// GetPluginsByCap returns plugins that provide a specific cap
func (s *PluginRepoServer) GetPluginsByCap(capUrn string) ([]PluginInfo, error) {
	plugins, err := s.TransformToPluginArray()
	if err != nil {
		return nil, err
	}

	results := make([]PluginInfo, 0)
	for _, plugin := range plugins {
		for _, cap := range plugin.Caps {
			if cap.Urn == capUrn {
				results = append(results, plugin)
				break
			}
		}
	}

	return results, nil
}

// PluginRepoCache holds cached plugin repository data
type PluginRepoCache struct {
	plugins      map[string]PluginInfo
	capToPlugins map[string][]string
	lastUpdated  time.Time
	repoUrl      string
}

// PluginRepo is a service for fetching and caching plugin repository data
type PluginRepo struct {
	httpClient *http.Client
	caches     map[string]*PluginRepoCache
	cacheTTL   time.Duration
	mu         sync.RWMutex
}

// NewPluginRepo creates a new plugin repo service
func NewPluginRepo(cacheTTLSeconds uint64) *PluginRepo {
	return &PluginRepo{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		caches:   make(map[string]*PluginRepoCache),
		cacheTTL: time.Duration(cacheTTLSeconds) * time.Second,
	}
}

// fetchRegistry fetches plugin registry from a URL
func (r *PluginRepo) fetchRegistry(repoUrl string) (*PluginRegistryResponse, error) {
	resp, err := r.httpClient.Get(repoUrl)
	if err != nil {
		return nil, NewHttpError(fmt.Sprintf("Failed to fetch from %s: %v", repoUrl, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, NewStatusError(resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewHttpError(fmt.Sprintf("Failed to read response from %s: %v", repoUrl, err))
	}

	var registry PluginRegistryResponse
	if err := json.Unmarshal(body, &registry); err != nil {
		return nil, NewParseError(fmt.Sprintf("Failed to parse from %s: %v", repoUrl, err))
	}

	return &registry, nil
}

// updateCache updates cache from a registry response
func (r *PluginRepo) updateCache(repoUrl string, registry *PluginRegistryResponse) {
	plugins := make(map[string]PluginInfo)
	capToPlugins := make(map[string][]string)

	for _, pluginInfo := range registry.Plugins {
		pluginId := pluginInfo.Id
		for _, cap := range pluginInfo.Caps {
			capToPlugins[cap.Urn] = append(capToPlugins[cap.Urn], pluginId)
		}
		plugins[pluginId] = pluginInfo
	}

	r.mu.Lock()
	r.caches[repoUrl] = &PluginRepoCache{
		plugins:      plugins,
		capToPlugins: capToPlugins,
		lastUpdated:  time.Now(),
		repoUrl:      repoUrl,
	}
	r.mu.Unlock()
}

// SyncRepos syncs plugin data from the given repository URLs
func (r *PluginRepo) SyncRepos(repoUrls []string) {
	for _, repoUrl := range repoUrls {
		registry, err := r.fetchRegistry(repoUrl)
		if err != nil {
			// Continue with other repos on error
			continue
		}
		r.updateCache(repoUrl, registry)
	}
}

// isCacheStale checks if a cache is stale
func (r *PluginRepo) isCacheStale(cache *PluginRepoCache) bool {
	return time.Since(cache.lastUpdated) > r.cacheTTL
}

// GetSuggestionsForCap gets plugin suggestions for a cap URN
func (r *PluginRepo) GetSuggestionsForCap(capUrn string) []PluginSuggestion {
	r.mu.RLock()
	defer r.mu.RUnlock()

	suggestions := make([]PluginSuggestion, 0)

	for _, cache := range r.caches {
		pluginIds, ok := cache.capToPlugins[capUrn]
		if !ok {
			continue
		}

		for _, pluginId := range pluginIds {
			plugin, ok := cache.plugins[pluginId]
			if !ok {
				continue
			}

			// Find the matching cap info
			for _, capInfo := range plugin.Caps {
				if capInfo.Urn == capUrn {
					pageUrl := plugin.PageUrl
					if pageUrl == "" {
						pageUrl = cache.repoUrl
					}

					suggestions = append(suggestions, PluginSuggestion{
						PluginId:          pluginId,
						PluginName:        plugin.Name,
						PluginDescription: plugin.Description,
						CapUrn:            capUrn,
						CapTitle:          capInfo.Title,
						LatestVersion:     plugin.Version,
						RepoUrl:           cache.repoUrl,
						PageUrl:           pageUrl,
					})
					break
				}
			}
		}
	}

	return suggestions
}

// GetAllPlugins gets all available plugins from all repos
func (r *PluginRepo) GetAllPlugins() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]PluginInfo, 0)
	for _, cache := range r.caches {
		for _, plugin := range cache.plugins {
			plugins = append(plugins, plugin)
		}
	}

	return plugins
}

// GetAllAvailableCaps gets all caps available from plugins
func (r *PluginRepo) GetAllAvailableCaps() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capsSet := make(map[string]bool)
	for _, cache := range r.caches {
		for cap := range cache.capToPlugins {
			capsSet[cap] = true
		}
	}

	caps := make([]string, 0, len(capsSet))
	for cap := range capsSet {
		caps = append(caps, cap)
	}
	sort.Strings(caps)

	return caps
}

// NeedsSync checks if any repo needs syncing
func (r *PluginRepo) NeedsSync(repoUrls []string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, repoUrl := range repoUrls {
		cache, ok := r.caches[repoUrl]
		if !ok {
			return true
		}
		if r.isCacheStale(cache) {
			return true
		}
	}

	return false
}

// GetPlugin gets plugin info by ID
func (r *PluginRepo) GetPlugin(pluginId string) *PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cache := range r.caches {
		if plugin, ok := cache.plugins[pluginId]; ok {
			return &plugin
		}
	}

	return nil
}
