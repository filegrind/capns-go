package bifaci

import (
	"testing"
)

// TEST320: Construct PluginInfo and verify fields
func Test320_plugin_info_construction(t *testing.T) {
	// TEST320: Construct PluginInfo and verify fields
	plugin := PluginInfo{
		Id:                "testplugin",
		Name:              "Test Plugin",
		Version:           "1.0.0",
		Description:       "A test plugin",
		Author:            "Test Author",
		Homepage:          "https://example.com",
		TeamId:            "TEAM123",
		SignedAt:          "2026-02-07T00:00:00Z",
		MinAppVersion:     "1.0.0",
		PageUrl:           "https://example.com/plugin",
		Categories:        []string{"test"},
		Tags:              []string{"testing"},
		Caps:              []PluginCapSummary{},
		Platform:          "darwin-arm64",
		PackageName:       "test-1.0.0.pkg",
		PackageSha256:     "abc123",
		PackageSize:       1000,
		BinaryName:        "test-1.0.0-darwin-arm64",
		BinarySha256:      "def456",
		BinarySize:        2000,
		Changelog:         make(map[string][]string),
		AvailableVersions: []string{"1.0.0"},
	}

	if plugin.Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", plugin.Id)
	}
	if plugin.Name != "Test Plugin" {
		t.Errorf("Expected name 'Test Plugin', got '%s'", plugin.Name)
	}
	if plugin.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", plugin.Version)
	}
}

// TEST321: Verify IsSigned() method
func Test321_plugin_info_is_signed(t *testing.T) {
	// TEST321: Verify IsSigned() method
	plugin := PluginInfo{
		Id:                "testplugin",
		Name:              "Test",
		Version:           "1.0.0",
		Description:       "",
		Author:            "",
		Homepage:          "",
		TeamId:            "TEAM123",
		SignedAt:          "2026-02-07T00:00:00Z",
		MinAppVersion:     "",
		PageUrl:           "",
		Categories:        []string{},
		Tags:              []string{},
		Caps:              []PluginCapSummary{},
		Platform:          "",
		PackageName:       "",
		PackageSha256:     "",
		PackageSize:       0,
		BinaryName:        "",
		BinarySha256:      "",
		BinarySize:        0,
		Changelog:         make(map[string][]string),
		AvailableVersions: []string{},
	}

	if !plugin.IsSigned() {
		t.Error("Expected plugin to be signed")
	}

	plugin.TeamId = ""
	if plugin.IsSigned() {
		t.Error("Expected plugin not to be signed when team_id is empty")
	}

	plugin.TeamId = "TEAM123"
	plugin.SignedAt = ""
	if plugin.IsSigned() {
		t.Error("Expected plugin not to be signed when signed_at is empty")
	}
}

// TEST322: Verify HasBinary() method
func Test322_plugin_info_has_binary(t *testing.T) {
	// TEST322: Verify HasBinary() method
	plugin := PluginInfo{
		Id:                "testplugin",
		Name:              "Test",
		Version:           "1.0.0",
		Description:       "",
		Author:            "",
		Homepage:          "",
		TeamId:            "",
		SignedAt:          "",
		MinAppVersion:     "",
		PageUrl:           "",
		Categories:        []string{},
		Tags:              []string{},
		Caps:              []PluginCapSummary{},
		Platform:          "",
		PackageName:       "",
		PackageSha256:     "",
		PackageSize:       0,
		BinaryName:        "test-1.0.0",
		BinarySha256:      "abc123",
		BinarySize:        0,
		Changelog:         make(map[string][]string),
		AvailableVersions: []string{},
	}

	if !plugin.HasBinary() {
		t.Error("Expected plugin to have binary")
	}

	plugin.BinaryName = ""
	if plugin.HasBinary() {
		t.Error("Expected plugin not to have binary when binary_name is empty")
	}

	plugin.BinaryName = "test-1.0.0"
	plugin.BinarySha256 = ""
	if plugin.HasBinary() {
		t.Error("Expected plugin not to have binary when binary_sha256 is empty")
	}
}

// TEST323: Validate registry schema version
func Test323_plugin_repo_server_validate_registry(t *testing.T) {
	// TEST323: Validate registry schema version
	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       make(map[string]PluginRegistryEntry),
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Errorf("Expected no error for v3.0, got %v", err)
	}
	if server == nil {
		t.Error("Expected server to be created")
	}

	// Test v2.0 schema rejection
	oldRegistry := PluginRegistryV3{
		SchemaVersion: "2.0",
		LastUpdated:   "2026-02-07",
		Plugins:       make(map[string]PluginRegistryEntry),
	}

	server, err = NewPluginRepoServer(oldRegistry)
	if err == nil {
		t.Error("Expected error for v2.0 schema")
	}
	if server != nil {
		t.Error("Expected no server to be created for v2.0")
	}
}

// TEST324: Transform v3 registry to flat plugin array
func Test324_plugin_repo_server_transform_to_array(t *testing.T) {
	// TEST324: Transform v3 registry to flat plugin array
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{"Initial release"},
		MinAppVersion: "1.0.0",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "test-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "test-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	plugins["testplugin"] = PluginRegistryEntry{
		Name:          "Test Plugin",
		Description:   "A test plugin",
		Author:        "Test Author",
		PageUrl:       "https://example.com",
		TeamId:        "TEAM123",
		MinAppVersion: "1.0.0",
		Caps:          []PluginCapSummary{},
		Categories:    []string{"test"},
		Tags:          []string{"testing"},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	pluginsArray, err := server.TransformToPluginArray()
	if err != nil {
		t.Fatalf("Failed to transform: %v", err)
	}

	if len(pluginsArray) != 1 {
		t.Fatalf("Expected 1 plugin, got %d", len(pluginsArray))
	}
	if pluginsArray[0].Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", pluginsArray[0].Id)
	}
	if pluginsArray[0].Name != "Test Plugin" {
		t.Errorf("Expected name 'Test Plugin', got '%s'", pluginsArray[0].Name)
	}
	if pluginsArray[0].Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", pluginsArray[0].Version)
	}
	if pluginsArray[0].BinaryName != "test-1.0.0-darwin-arm64" {
		t.Errorf("Expected binary_name 'test-1.0.0-darwin-arm64', got '%s'", pluginsArray[0].BinaryName)
	}
}

// TEST325: Get all plugins via GetPlugins()
func Test325_plugin_repo_server_get_plugins(t *testing.T) {
	// TEST325: Get all plugins via GetPlugins()
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{},
		MinAppVersion: "",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "test-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "test-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	plugins["testplugin"] = PluginRegistryEntry{
		Name:          "Test Plugin",
		Description:   "A test plugin",
		Author:        "Test Author",
		PageUrl:       "",
		TeamId:        "TEAM123",
		MinAppVersion: "",
		Caps:          []PluginCapSummary{},
		Categories:    []string{},
		Tags:          []string{},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	response, err := server.GetPlugins()
	if err != nil {
		t.Fatalf("Failed to get plugins: %v", err)
	}

	if len(response.Plugins) != 1 {
		t.Fatalf("Expected 1 plugin, got %d", len(response.Plugins))
	}
	if response.Plugins[0].Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", response.Plugins[0].Id)
	}
}

// TEST326: Get plugin by ID
func Test326_plugin_repo_server_get_plugin_by_id(t *testing.T) {
	// TEST326: Get plugin by ID
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{},
		MinAppVersion: "",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "test-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "test-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	plugins["testplugin"] = PluginRegistryEntry{
		Name:          "Test Plugin",
		Description:   "A test plugin",
		Author:        "Test Author",
		PageUrl:       "",
		TeamId:        "TEAM123",
		MinAppVersion: "",
		Caps:          []PluginCapSummary{},
		Categories:    []string{},
		Tags:          []string{},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	result, err := server.GetPluginById("testplugin")
	if err != nil {
		t.Fatalf("Failed to get plugin: %v", err)
	}
	if result == nil {
		t.Fatal("Expected plugin to be found")
	}
	if result.Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", result.Id)
	}

	notFound, err := server.GetPluginById("nonexistent")
	if err != nil {
		t.Fatalf("Failed to get plugin: %v", err)
	}
	if notFound != nil {
		t.Error("Expected plugin not to be found")
	}
}

// TEST327: Search plugins by text query
func Test327_plugin_repo_server_search_plugins(t *testing.T) {
	// TEST327: Search plugins by text query
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{},
		MinAppVersion: "",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "pdf-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "pdf-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	plugins["pdfplugin"] = PluginRegistryEntry{
		Name:          "PDF Plugin",
		Description:   "Process PDF documents",
		Author:        "Test Author",
		PageUrl:       "",
		TeamId:        "TEAM123",
		MinAppVersion: "",
		Caps:          []PluginCapSummary{},
		Categories:    []string{},
		Tags:          []string{"document"},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	results, err := server.SearchPlugins("pdf")
	if err != nil {
		t.Fatalf("Failed to search plugins: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Id != "pdfplugin" {
		t.Errorf("Expected id 'pdfplugin', got '%s'", results[0].Id)
	}

	noMatch, err := server.SearchPlugins("nonexistent")
	if err != nil {
		t.Fatalf("Failed to search plugins: %v", err)
	}
	if len(noMatch) != 0 {
		t.Errorf("Expected 0 results, got %d", len(noMatch))
	}
}

// TEST328: Filter plugins by category
func Test328_plugin_repo_server_get_by_category(t *testing.T) {
	// TEST328: Filter plugins by category
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{},
		MinAppVersion: "",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "test-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "test-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	plugins["docplugin"] = PluginRegistryEntry{
		Name:          "Doc Plugin",
		Description:   "Process documents",
		Author:        "Test Author",
		PageUrl:       "",
		TeamId:        "TEAM123",
		MinAppVersion: "",
		Caps:          []PluginCapSummary{},
		Categories:    []string{"document"},
		Tags:          []string{},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	results, err := server.GetPluginsByCategory("document")
	if err != nil {
		t.Fatalf("Failed to get plugins by category: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Id != "docplugin" {
		t.Errorf("Expected id 'docplugin', got '%s'", results[0].Id)
	}

	noMatch, err := server.GetPluginsByCategory("nonexistent")
	if err != nil {
		t.Fatalf("Failed to get plugins by category: %v", err)
	}
	if len(noMatch) != 0 {
		t.Errorf("Expected 0 results, got %d", len(noMatch))
	}
}

// TEST329: Find plugins by cap URN
func Test329_plugin_repo_server_get_by_cap(t *testing.T) {
	// TEST329: Find plugins by cap URN
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{},
		MinAppVersion: "",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "test-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "test-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	capUrn := `cap:in="media:pdf";op=disbind;out="media:disbound-page;textable;list"`
	plugins["pdfplugin"] = PluginRegistryEntry{
		Name:          "PDF Plugin",
		Description:   "Process PDFs",
		Author:        "Test Author",
		PageUrl:       "",
		TeamId:        "TEAM123",
		MinAppVersion: "",
		Caps: []PluginCapSummary{
			{
				Urn:         capUrn,
				Title:       "Disbind PDF",
				Description: "Extract pages",
			},
		},
		Categories:    []string{},
		Tags:          []string{},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	results, err := server.GetPluginsByCap(capUrn)
	if err != nil {
		t.Fatalf("Failed to get plugins by cap: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Id != "pdfplugin" {
		t.Errorf("Expected id 'pdfplugin', got '%s'", results[0].Id)
	}

	noMatch, err := server.GetPluginsByCap("cap:nonexistent")
	if err != nil {
		t.Fatalf("Failed to get plugins by cap: %v", err)
	}
	if len(noMatch) != 0 {
		t.Errorf("Expected 0 results, got %d", len(noMatch))
	}
}

// TEST330: PluginRepoClient cache update
func Test330_plugin_repo_client_update_cache(t *testing.T) {
	// TEST330: PluginRepoClient cache update
	repo := NewPluginRepo(3600)

	// Create a mock registry response
	registry := &PluginRegistryResponse{
		Plugins: []PluginInfo{
			{
				Id:                "testplugin",
				Name:              "Test Plugin",
				Version:           "1.0.0",
				Description:       "",
				Author:            "",
				Homepage:          "",
				TeamId:            "TEAM123",
				SignedAt:          "2026-02-07",
				MinAppVersion:     "",
				PageUrl:           "",
				Categories:        []string{},
				Tags:              []string{},
				Caps:              []PluginCapSummary{},
				Platform:          "",
				PackageName:       "",
				PackageSha256:     "",
				PackageSize:       0,
				BinaryName:        "test-binary",
				BinarySha256:      "abc123",
				BinarySize:        0,
				Changelog:         make(map[string][]string),
				AvailableVersions: []string{},
			},
		},
	}

	// Update cache directly (simulating a fetch)
	repo.updateCache("https://example.com/plugins", registry)

	// Verify cache was updated
	plugin := repo.GetPlugin("testplugin")
	if plugin == nil {
		t.Fatal("Expected plugin to be found")
	}
	if plugin.Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", plugin.Id)
	}
}

// TEST331: Get suggestions for missing cap
func Test331_plugin_repo_client_get_suggestions(t *testing.T) {
	// TEST331: Get suggestions for missing cap
	repo := NewPluginRepo(3600)

	capUrn := `cap:in="media:pdf";op=disbind;out="media:disbound-page;textable;list"`
	registry := &PluginRegistryResponse{
		Plugins: []PluginInfo{
			{
				Id:            "pdfplugin",
				Name:          "PDF Plugin",
				Version:       "1.0.0",
				Description:   "Process PDFs",
				Author:        "",
				Homepage:      "",
				TeamId:        "TEAM123",
				SignedAt:      "2026-02-07",
				MinAppVersion: "",
				PageUrl:       "https://example.com/pdf",
				Categories:    []string{},
				Tags:          []string{},
				Caps: []PluginCapSummary{
					{
						Urn:         capUrn,
						Title:       "Disbind PDF",
						Description: "Extract pages",
					},
				},
				Platform:          "",
				PackageName:       "",
				PackageSha256:     "",
				PackageSize:       0,
				BinaryName:        "",
				BinarySha256:      "",
				BinarySize:        0,
				Changelog:         make(map[string][]string),
				AvailableVersions: []string{},
			},
		},
	}

	repo.updateCache("https://example.com/plugins", registry)

	suggestions := repo.GetSuggestionsForCap(capUrn)
	if len(suggestions) != 1 {
		t.Fatalf("Expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].PluginId != "pdfplugin" {
		t.Errorf("Expected plugin_id 'pdfplugin', got '%s'", suggestions[0].PluginId)
	}
	if suggestions[0].CapUrn != capUrn {
		t.Errorf("Expected cap_urn '%s', got '%s'", capUrn, suggestions[0].CapUrn)
	}
}

// TEST332: Get plugin by ID from client
func Test332_plugin_repo_client_get_plugin(t *testing.T) {
	// TEST332: Get plugin by ID from client
	repo := NewPluginRepo(3600)

	registry := &PluginRegistryResponse{
		Plugins: []PluginInfo{
			{
				Id:                "testplugin",
				Name:              "Test Plugin",
				Version:           "1.0.0",
				Description:       "",
				Author:            "",
				Homepage:          "",
				TeamId:            "",
				SignedAt:          "",
				MinAppVersion:     "",
				PageUrl:           "",
				Categories:        []string{},
				Tags:              []string{},
				Caps:              []PluginCapSummary{},
				Platform:          "",
				PackageName:       "",
				PackageSha256:     "",
				PackageSize:       0,
				BinaryName:        "",
				BinarySha256:      "",
				BinarySize:        0,
				Changelog:         make(map[string][]string),
				AvailableVersions: []string{},
			},
		},
	}

	repo.updateCache("https://example.com/plugins", registry)

	plugin := repo.GetPlugin("testplugin")
	if plugin == nil {
		t.Fatal("Expected plugin to be found")
	}
	if plugin.Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", plugin.Id)
	}

	notFound := repo.GetPlugin("nonexistent")
	if notFound != nil {
		t.Error("Expected plugin not to be found")
	}
}

// TEST333: Get all available caps
func Test333_plugin_repo_client_get_all_caps(t *testing.T) {
	// TEST333: Get all available caps
	repo := NewPluginRepo(3600)

	cap1 := `cap:in="media:pdf";op=disbind;out="media:disbound-page;textable;list"`
	cap2 := `cap:in="media:txt;textable";op=disbind;out="media:disbound-page;textable;list"`

	registry := &PluginRegistryResponse{
		Plugins: []PluginInfo{
			{
				Id:            "plugin1",
				Name:          "Plugin 1",
				Version:       "1.0.0",
				Description:   "",
				Author:        "",
				Homepage:      "",
				TeamId:        "",
				SignedAt:      "",
				MinAppVersion: "",
				PageUrl:       "",
				Categories:    []string{},
				Tags:          []string{},
				Caps: []PluginCapSummary{
					{
						Urn:         cap1,
						Title:       "cap.Cap 1",
						Description: "",
					},
				},
				Platform:          "",
				PackageName:       "",
				PackageSha256:     "",
				PackageSize:       0,
				BinaryName:        "",
				BinarySha256:      "",
				BinarySize:        0,
				Changelog:         make(map[string][]string),
				AvailableVersions: []string{},
			},
			{
				Id:            "plugin2",
				Name:          "Plugin 2",
				Version:       "1.0.0",
				Description:   "",
				Author:        "",
				Homepage:      "",
				TeamId:        "",
				SignedAt:      "",
				MinAppVersion: "",
				PageUrl:       "",
				Categories:    []string{},
				Tags:          []string{},
				Caps: []PluginCapSummary{
					{
						Urn:         cap2,
						Title:       "cap.Cap 2",
						Description: "",
					},
				},
				Platform:          "",
				PackageName:       "",
				PackageSha256:     "",
				PackageSize:       0,
				BinaryName:        "",
				BinarySha256:      "",
				BinarySize:        0,
				Changelog:         make(map[string][]string),
				AvailableVersions: []string{},
			},
		},
	}

	repo.updateCache("https://example.com/plugins", registry)

	caps := repo.GetAllAvailableCaps()
	if len(caps) != 2 {
		t.Fatalf("Expected 2 caps, got %d", len(caps))
	}

	// Check both caps are present
	capFound1 := false
	capFound2 := false
	for _, cap := range caps {
		if cap == cap1 {
			capFound1 = true
		}
		if cap == cap2 {
			capFound2 = true
		}
	}
	if !capFound1 {
		t.Error("Expected cap1 to be found")
	}
	if !capFound2 {
		t.Error("Expected cap2 to be found")
	}
}

// TEST334: Check if client needs sync
func Test334_plugin_repo_client_needs_sync(t *testing.T) {
	// TEST334: Check if client needs sync
	repo := NewPluginRepo(3600)

	urls := []string{"https://example.com/plugins"}

	// Empty cache should need sync
	if !repo.NeedsSync(urls) {
		t.Error("Expected to need sync with empty cache")
	}

	// After update, should not need sync
	registry := &PluginRegistryResponse{Plugins: []PluginInfo{}}
	repo.updateCache("https://example.com/plugins", registry)

	if repo.NeedsSync(urls) {
		t.Error("Expected not to need sync after update")
	}
}

// TEST335: Server creates response, client consumes it
func Test335_plugin_repo_server_client_integration(t *testing.T) {
	// TEST335: Server creates response, client consumes it
	plugins := make(map[string]PluginRegistryEntry)
	versions := make(map[string]PluginVersionData)

	versions["1.0.0"] = PluginVersionData{
		ReleaseDate:   "2026-02-07",
		Changelog:     []string{},
		MinAppVersion: "",
		Platform:      "darwin-arm64",
		Package: PluginDistributionInfo{
			Name:   "test-1.0.0.pkg",
			Sha256: "abc123",
			Size:   1000,
		},
		Binary: PluginDistributionInfo{
			Name:   "test-1.0.0-darwin-arm64",
			Sha256: "def456",
			Size:   2000,
		},
	}

	capUrn := `cap:in="media:test";op=test;out="media:result"`
	plugins["testplugin"] = PluginRegistryEntry{
		Name:          "Test Plugin",
		Description:   "A test plugin",
		Author:        "Test Author",
		PageUrl:       "https://example.com",
		TeamId:        "TEAM123",
		MinAppVersion: "",
		Caps: []PluginCapSummary{
			{
				Urn:         capUrn,
				Title:       "Test cap.Cap",
				Description: "Test capability",
			},
		},
		Categories:    []string{"test"},
		Tags:          []string{},
		LatestVersion: "1.0.0",
		Versions:      versions,
	}

	registry := PluginRegistryV3{
		SchemaVersion: "3.0",
		LastUpdated:   "2026-02-07",
		Plugins:       plugins,
	}

	// Server transforms registry
	server, err := NewPluginRepoServer(registry)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	response, err := server.GetPlugins()
	if err != nil {
		t.Fatalf("Failed to get plugins: %v", err)
	}

	// Verify response structure
	if len(response.Plugins) != 1 {
		t.Fatalf("Expected 1 plugin, got %d", len(response.Plugins))
	}

	plugin := &response.Plugins[0]
	if plugin.Id != "testplugin" {
		t.Errorf("Expected id 'testplugin', got '%s'", plugin.Id)
	}
	if plugin.Name != "Test Plugin" {
		t.Errorf("Expected name 'Test Plugin', got '%s'", plugin.Name)
	}
	if !plugin.IsSigned() {
		t.Error("Expected plugin to be signed")
	}
	if !plugin.HasBinary() {
		t.Error("Expected plugin to have binary")
	}
	if len(plugin.Caps) != 1 {
		t.Fatalf("Expected 1 cap, got %d", len(plugin.Caps))
	}
	if plugin.Caps[0].Urn != capUrn {
		t.Errorf("Expected cap URN '%s', got '%s'", capUrn, plugin.Caps[0].Urn)
	}

	// Simulate client consuming this response
	if plugin.BinaryName != "test-1.0.0-darwin-arm64" {
		t.Errorf("Expected binary_name 'test-1.0.0-darwin-arm64', got '%s'", plugin.BinaryName)
	}
	if plugin.BinarySha256 != "def456" {
		t.Errorf("Expected binary_sha256 'def456', got '%s'", plugin.BinarySha256)
	}
}
