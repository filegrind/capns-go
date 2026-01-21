package capns

import (
	"context"
	"testing"
)

// MockCapSetForRegistry for testing (avoid conflict with existing MockCapSet)
type MockCapSetForRegistry struct {
	name string
}

func (m *MockCapSetForRegistry) ExecuteCap(
	ctx context.Context,
	capUrn string,
	positionalArgs []string,
	namedArgs map[string]string,
	stdinSource *StdinSource,
) (*HostResult, error) {
	return &HostResult{
		TextOutput: "Mock response from " + m.name,
	}, nil
}

// Test helper for matrix tests
func matrixTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:type=void;v=1";out="media:type=object;v=1"`
	}
	return `cap:in="media:type=void;v=1";out="media:type=object;v=1";` + tags
}

func TestRegisterAndFindCapSet(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "test-host"}

	capUrn, err := NewCapUrnFromString(matrixTestUrn("op=test;type=basic"))
	if err != nil {
		t.Fatalf("Failed to create CapUrn: %v", err)
	}

	cap := &Cap{
		Urn:            capUrn,
		CapDescription: stringPtr("Test capability"),
		Metadata:       make(map[string]string),
		Command:        "test",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}

	err = registry.RegisterCapSet("test-host", host, []*Cap{cap})
	if err != nil {
		t.Fatalf("Failed to register cap host: %v", err)
	}

	// Test exact match
	sets, err := registry.FindCapSets(matrixTestUrn("op=test;type=basic"))
	if err != nil {
		t.Fatalf("Failed to find cap sets: %v", err)
	}
	if len(sets) != 1 {
		t.Errorf("Expected 1 host, got %d", len(sets))
	}

	// Test subset match (request has more specific requirements)
	sets, err = registry.FindCapSets(matrixTestUrn("model=gpt-4;op=test;type=basic"))
	if err != nil {
		t.Fatalf("Failed to find cap sets for subset match: %v", err)
	}
	if len(sets) != 1 {
		t.Errorf("Expected 1 host for subset match, got %d", len(sets))
	}

	// Test no match (different direction specs)
	_, err = registry.FindCapSets(`cap:in="media:type=binary;v=1";op=different;out="media:type=object;v=1"`)
	if err == nil {
		t.Error("Expected error for non-matching capability, got nil")
	}
}

func TestBestCapSetSelection(t *testing.T) {
	registry := NewCapMatrix()

	// Register general host
	generalHost := &MockCapSetForRegistry{name: "general"}
	generalCapUrn, _ := NewCapUrnFromString(matrixTestUrn("op=generate"))
	generalCap := &Cap{
		Urn:            generalCapUrn,
		CapDescription: stringPtr("General generation"),
		Metadata:       make(map[string]string),
		Command:        "generate",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}

	// Register specific host
	specificHost := &MockCapSetForRegistry{name: "specific"}
	specificCapUrn, _ := NewCapUrnFromString(matrixTestUrn("model=gpt-4;op=generate;type=text"))
	specificCap := &Cap{
		Urn:            specificCapUrn,
		CapDescription: stringPtr("Specific text generation"),
		Metadata:       make(map[string]string),
		Command:        "generate",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}

	registry.RegisterCapSet("general", generalHost, []*Cap{generalCap})
	registry.RegisterCapSet("specific", specificHost, []*Cap{specificCap})

	// Request should match the more specific host
	bestHost, bestCap, err := registry.FindBestCapSet(matrixTestUrn("model=gpt-4;op=generate;temperature=0.7;type=text"))
	if err != nil {
		t.Fatalf("Failed to find best cap host: %v", err)
	}

	// Should get the specific host (though we can't directly compare interfaces)
	if bestHost == nil {
		t.Error("Expected a host, got nil")
	}
	if bestCap == nil {
		t.Error("Expected a cap definition, got nil")
	}

	// Both sets should match
	allHosts, err := registry.FindCapSets(matrixTestUrn("model=gpt-4;op=generate;temperature=0.7;type=text"))
	if err != nil {
		t.Fatalf("Failed to find all matching sets: %v", err)
	}
	if len(allHosts) != 2 {
		t.Errorf("Expected 2 sets, got %d", len(allHosts))
	}
}

func TestInvalidUrnHandling(t *testing.T) {
	registry := NewCapMatrix()

	_, err := registry.FindCapSets("invalid-urn")
	if err == nil {
		t.Error("Expected error for invalid URN, got nil")
	}

	capSetErr, ok := err.(*CapMatrixError)
	if !ok {
		t.Errorf("Expected CapMatrixError, got %T", err)
	} else if capSetErr.Type != "InvalidUrn" {
		t.Errorf("Expected InvalidUrn error type, got %s", capSetErr.Type)
	}
}

func TestCanHandle(t *testing.T) {
	registry := NewCapMatrix()

	// Empty registry
	if registry.CanHandle(matrixTestUrn("op=test")) {
		t.Error("Empty registry should not handle any capability")
	}

	// After registration
	host := &MockCapSetForRegistry{name: "test"}
	capUrn, _ := NewCapUrnFromString(matrixTestUrn("op=test"))
	cap := &Cap{
		Urn:            capUrn,
		CapDescription: stringPtr("Test"),
		Metadata:       make(map[string]string),
		Command:        "test",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}

	registry.RegisterCapSet("test", host, []*Cap{cap})

	if !registry.CanHandle(matrixTestUrn("op=test")) {
		t.Error("Registry should handle registered capability")
	}
	if !registry.CanHandle(matrixTestUrn("extra=param;op=test")) {
		t.Error("Registry should handle capability with extra parameters")
	}
	if registry.CanHandle(matrixTestUrn("op=different")) {
		t.Error("Registry should not handle unregistered capability")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}

// ============================================================================
// CapCube Tests
// ============================================================================

// Helper to create a Cap for testing
func makeCap(urn string, title string) *Cap {
	capUrn, _ := NewCapUrnFromString(urn)
	return &Cap{
		Urn:            capUrn,
		Title:          title,
		CapDescription: stringPtr(title),
		Metadata:       make(map[string]string),
		Command:        "test",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}
}

func TestCapCubeMoreSpecificWins(t *testing.T) {
	// This is the key test: provider has less specific cap, plugin has more specific
	// The more specific one should win regardless of registry order

	providerRegistry := NewCapMatrix()
	pluginRegistry := NewCapMatrix()

	// Provider: less specific cap (no ext tag)
	providerHost := &MockCapSetForRegistry{name: "provider"}
	providerCap := makeCap(
		`cap:in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`,
		"Provider Thumbnail Generator (generic)",
	)
	providerRegistry.RegisterCapSet("provider", providerHost, []*Cap{providerCap})

	// Plugin: more specific cap (has ext=pdf)
	pluginHost := &MockCapSetForRegistry{name: "plugin"}
	pluginCap := makeCap(
		`cap:ext=pdf;in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`,
		"Plugin PDF Thumbnail Generator (specific)",
	)
	pluginRegistry.RegisterCapSet("plugin", pluginHost, []*Cap{pluginCap})

	// Create composite with provider first (normally would have priority on ties)
	composite := NewCapCube()
	composite.AddRegistry("providers", providerRegistry)
	composite.AddRegistry("plugins", pluginRegistry)

	// Request for PDF thumbnails - plugin's more specific cap should win
	request := `cap:ext=pdf;in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`
	best, err := composite.FindBestCapSet(request)
	if err != nil {
		t.Fatalf("Failed to find best cap set: %v", err)
	}

	// Plugin registry has specificity 4 (in, op, out, ext)
	// Provider registry has specificity 3 (in, op, out)
	// Plugin should win even though providers were added first
	if best.RegistryName != "plugins" {
		t.Errorf("Expected plugins registry to win, got %s", best.RegistryName)
	}
	if best.Specificity != 4 {
		t.Errorf("Expected specificity 4, got %d", best.Specificity)
	}
	if best.Cap.Title != "Plugin PDF Thumbnail Generator (specific)" {
		t.Errorf("Expected plugin cap title, got %s", best.Cap.Title)
	}
}

func TestCapCubeTieGoesToFirst(t *testing.T) {
	// When specificity is equal, first registry wins

	registry1 := NewCapMatrix()
	registry2 := NewCapMatrix()

	// Both have same specificity
	host1 := &MockCapSetForRegistry{name: "host1"}
	cap1 := makeCap(matrixTestUrn("ext=pdf;op=generate"), "Registry 1 Cap")
	registry1.RegisterCapSet("host1", host1, []*Cap{cap1})

	host2 := &MockCapSetForRegistry{name: "host2"}
	cap2 := makeCap(matrixTestUrn("ext=pdf;op=generate"), "Registry 2 Cap")
	registry2.RegisterCapSet("host2", host2, []*Cap{cap2})

	composite := NewCapCube()
	composite.AddRegistry("first", registry1)
	composite.AddRegistry("second", registry2)

	best, err := composite.FindBestCapSet(matrixTestUrn("ext=pdf;op=generate"))
	if err != nil {
		t.Fatalf("Failed to find best cap set: %v", err)
	}

	// Both have same specificity, first registry should win
	if best.RegistryName != "first" {
		t.Errorf("Expected first registry to win on tie, got %s", best.RegistryName)
	}
	if best.Cap.Title != "Registry 1 Cap" {
		t.Errorf("Expected Registry 1 Cap, got %s", best.Cap.Title)
	}
}

func TestCapCubePollsAll(t *testing.T) {
	// Test that all registries are polled

	registry1 := NewCapMatrix()
	registry2 := NewCapMatrix()
	registry3 := NewCapMatrix()

	// Registry 1: doesn't match
	host1 := &MockCapSetForRegistry{name: "host1"}
	cap1 := makeCap(matrixTestUrn("op=different"), "Registry 1")
	registry1.RegisterCapSet("host1", host1, []*Cap{cap1})

	// Registry 2: matches but less specific
	host2 := &MockCapSetForRegistry{name: "host2"}
	cap2 := makeCap(matrixTestUrn("op=generate"), "Registry 2")
	registry2.RegisterCapSet("host2", host2, []*Cap{cap2})

	// Registry 3: matches and most specific
	host3 := &MockCapSetForRegistry{name: "host3"}
	cap3 := makeCap(matrixTestUrn("ext=pdf;format=thumbnail;op=generate"), "Registry 3")
	registry3.RegisterCapSet("host3", host3, []*Cap{cap3})

	composite := NewCapCube()
	composite.AddRegistry("r1", registry1)
	composite.AddRegistry("r2", registry2)
	composite.AddRegistry("r3", registry3)

	best, err := composite.FindBestCapSet(matrixTestUrn("ext=pdf;format=thumbnail;op=generate"))
	if err != nil {
		t.Fatalf("Failed to find best cap set: %v", err)
	}

	// Registry 3 has more specific tags
	if best.RegistryName != "r3" {
		t.Errorf("Expected r3 (most specific) to win, got %s", best.RegistryName)
	}
}

func TestCapCubeNoMatch(t *testing.T) {
	registry := NewCapMatrix()

	composite := NewCapCube()
	composite.AddRegistry("empty", registry)

	_, err := composite.FindBestCapSet(matrixTestUrn("op=nonexistent"))
	if err == nil {
		t.Error("Expected error for non-matching capability, got nil")
	}

	capSetErr, ok := err.(*CapMatrixError)
	if !ok {
		t.Errorf("Expected CapMatrixError, got %T", err)
	} else if capSetErr.Type != "NoSetsFound" {
		t.Errorf("Expected NoSetsFound error type, got %s", capSetErr.Type)
	}
}

func TestCapCubeFallbackScenario(t *testing.T) {
	// Test the exact scenario from the user's issue:
	// Provider: generic fallback (can handle any file type)
	// Plugin:   PDF-specific handler
	// Request:  PDF thumbnail
	// Expected: Plugin wins (more specific)

	providerRegistry := NewCapMatrix()
	pluginRegistry := NewCapMatrix()

	// Provider with generic fallback (can handle any file type)
	providerHost := &MockCapSetForRegistry{name: "provider_fallback"}
	providerCap := makeCap(
		`cap:in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`,
		"Generic Thumbnail Provider",
	)
	providerRegistry.RegisterCapSet("provider_fallback", providerHost, []*Cap{providerCap})

	// Plugin with PDF-specific handler
	pluginHost := &MockCapSetForRegistry{name: "pdf_plugin"}
	pluginCap := makeCap(
		`cap:ext=pdf;in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`,
		"PDF Thumbnail Plugin",
	)
	pluginRegistry.RegisterCapSet("pdf_plugin", pluginHost, []*Cap{pluginCap})

	// Providers first (would win on tie)
	composite := NewCapCube()
	composite.AddRegistry("providers", providerRegistry)
	composite.AddRegistry("plugins", pluginRegistry)

	// Request for PDF thumbnail
	request := `cap:ext=pdf;in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`
	best, err := composite.FindBestCapSet(request)
	if err != nil {
		t.Fatalf("Failed to find best cap set: %v", err)
	}

	// Plugin (specificity 4) should beat provider (specificity 3)
	if best.RegistryName != "plugins" {
		t.Errorf("Expected plugins to win, got %s", best.RegistryName)
	}
	if best.Cap.Title != "PDF Thumbnail Plugin" {
		t.Errorf("Expected PDF Thumbnail Plugin, got %s", best.Cap.Title)
	}
	if best.Specificity != 4 {
		t.Errorf("Expected specificity 4, got %d", best.Specificity)
	}

	// Also test that for a different file type, provider wins
	requestWav := `cap:ext=wav;in="media:type=binary;v=1";op=generate_thumbnail;out="media:type=binary;v=1"`
	bestWav, err := composite.FindBestCapSet(requestWav)
	if err != nil {
		t.Fatalf("Failed to find best cap set for wav: %v", err)
	}

	// Only provider matches (plugin doesn't match ext=wav)
	if bestWav.RegistryName != "providers" {
		t.Errorf("Expected providers for wav request, got %s", bestWav.RegistryName)
	}
	if bestWav.Cap.Title != "Generic Thumbnail Provider" {
		t.Errorf("Expected Generic Thumbnail Provider, got %s", bestWav.Cap.Title)
	}
}

func TestCapCubeCanMethod(t *testing.T) {
	// Test the can() method that returns a CapCaller

	providerRegistry := NewCapMatrix()

	providerHost := &MockCapSetForRegistry{name: "test_provider"}
	providerCap := makeCap(matrixTestUrn("ext=pdf;op=generate"), "Test Provider")
	providerRegistry.RegisterCapSet("test_provider", providerHost, []*Cap{providerCap})

	composite := NewCapCube()
	composite.AddRegistry("providers", providerRegistry)

	// Test can() returns a CapCaller
	caller, err := composite.Can(matrixTestUrn("ext=pdf;op=generate"))
	if err != nil {
		t.Fatalf("Can() failed: %v", err)
	}
	if caller == nil {
		t.Error("Expected CapCaller, got nil")
	}

	// Verify we got the right cap via CanHandle checks
	if !composite.CanHandle(matrixTestUrn("ext=pdf;op=generate")) {
		t.Error("Expected CanHandle to return true for matching cap")
	}
	if composite.CanHandle(matrixTestUrn("op=nonexistent")) {
		t.Error("Expected CanHandle to return false for non-matching cap")
	}
}

func TestCapCubeRegistryManagement(t *testing.T) {
	composite := NewCapCube()

	registry1 := NewCapMatrix()
	registry2 := NewCapMatrix()

	// Test AddRegistry
	composite.AddRegistry("r1", registry1)
	composite.AddRegistry("r2", registry2)

	names := composite.GetRegistryNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 registries, got %d", len(names))
	}

	// Test GetRegistry
	got := composite.GetRegistry("r1")
	if got != registry1 {
		t.Error("GetRegistry returned wrong registry")
	}

	// Test RemoveRegistry
	removed := composite.RemoveRegistry("r1")
	if removed != registry1 {
		t.Error("RemoveRegistry returned wrong registry")
	}

	names = composite.GetRegistryNames()
	if len(names) != 1 {
		t.Errorf("Expected 1 registry after removal, got %d", len(names))
	}

	// Test GetRegistry for non-existent
	got = composite.GetRegistry("nonexistent")
	if got != nil {
		t.Error("Expected nil for non-existent registry")
	}
}

// ============================================================================
// CapGraph Tests
// ============================================================================

// Helper to create caps with specific in/out specs for graph testing
func makeGraphCap(inSpec, outSpec, title string) *Cap {
	// Media URNs need to be quoted in cap URN strings
	urn := `cap:in="` + inSpec + `";op=convert;out="` + outSpec + `"`
	capUrn, _ := NewCapUrnFromString(urn)
	return &Cap{
		Urn:            capUrn,
		Title:          title,
		CapDescription: stringPtr(title),
		Metadata:       make(map[string]string),
		Command:        "convert",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}
}

func TestCapGraphBasicConstruction(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// Add caps that form a graph:
	// binary -> str -> obj
	cap1 := makeGraphCap(MediaBinary, MediaString, "Binary to String")
	cap2 := makeGraphCap(MediaString, MediaObject, "String to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapCube()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Check nodes
	nodes := graph.GetNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// Check edges
	edges := graph.GetEdges()
	if len(edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(edges))
	}

	// Check stats
	stats := graph.Stats()
	if stats.NodeCount != 3 {
		t.Errorf("Expected 3 nodes in stats, got %d", stats.NodeCount)
	}
	if stats.EdgeCount != 2 {
		t.Errorf("Expected 2 edges in stats, got %d", stats.EdgeCount)
	}
}

func TestCapGraphOutgoingIncoming(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str, binary -> obj
	cap1 := makeGraphCap(MediaBinary, MediaString, "Binary to String")
	cap2 := makeGraphCap(MediaBinary, MediaObject, "Binary to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapCube()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// binary has 2 outgoing edges
	outgoing := graph.GetOutgoing(MediaBinary)
	if len(outgoing) != 2 {
		t.Errorf("Expected 2 outgoing edges from binary, got %d", len(outgoing))
	}

	// str has 1 incoming edge
	incoming := graph.GetIncoming(MediaString)
	if len(incoming) != 1 {
		t.Errorf("Expected 1 incoming edge to str, got %d", len(incoming))
	}

	// obj has 1 incoming edge
	incoming = graph.GetIncoming(MediaObject)
	if len(incoming) != 1 {
		t.Errorf("Expected 1 incoming edge to obj, got %d", len(incoming))
	}
}

func TestCapGraphCanConvert(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str -> obj
	cap1 := makeGraphCap(MediaBinary, MediaString, "Binary to String")
	cap2 := makeGraphCap(MediaString, MediaObject, "String to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapCube()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Direct conversions
	if !graph.CanConvert(MediaBinary, MediaString) {
		t.Error("Should be able to convert binary to str")
	}
	if !graph.CanConvert(MediaString, MediaObject) {
		t.Error("Should be able to convert str to obj")
	}

	// Transitive conversion
	if !graph.CanConvert(MediaBinary, MediaObject) {
		t.Error("Should be able to convert binary to obj (transitively)")
	}

	// Same spec
	if !graph.CanConvert(MediaBinary, MediaBinary) {
		t.Error("Should be able to convert same spec to itself")
	}

	// Impossible conversions
	if graph.CanConvert(MediaObject, MediaBinary) {
		t.Error("Should not be able to convert obj to binary (no reverse edge)")
	}
	if graph.CanConvert("media:type=nonexistent;v=1", MediaString) {
		t.Error("Should not be able to convert non-existent spec")
	}
}

func TestCapGraphFindPath(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str -> obj
	cap1 := makeGraphCap(MediaBinary, MediaString, "Binary to String")
	cap2 := makeGraphCap(MediaString, MediaObject, "String to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapCube()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Direct path
	path := graph.FindPath(MediaBinary, MediaString)
	if path == nil {
		t.Fatal("Expected to find path from binary to str")
	}
	if len(path) != 1 {
		t.Errorf("Expected path length 1, got %d", len(path))
	}

	// Transitive path
	path = graph.FindPath(MediaBinary, MediaObject)
	if path == nil {
		t.Fatal("Expected to find path from binary to obj")
	}
	if len(path) != 2 {
		t.Errorf("Expected path length 2, got %d", len(path))
	}
	if path[0].Cap.Title != "Binary to String" {
		t.Errorf("First edge should be Binary to String, got %s", path[0].Cap.Title)
	}
	if path[1].Cap.Title != "String to Object" {
		t.Errorf("Second edge should be String to Object, got %s", path[1].Cap.Title)
	}

	// No path
	path = graph.FindPath(MediaObject, MediaBinary)
	if path != nil {
		t.Error("Expected nil for impossible path")
	}

	// Same spec
	path = graph.FindPath(MediaBinary, MediaBinary)
	if path == nil {
		t.Fatal("Expected empty path for same spec")
	}
	if len(path) != 0 {
		t.Errorf("Expected empty path for same spec, got length %d", len(path))
	}
}

func TestCapGraphFindAllPaths(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// Create a graph with multiple paths:
	// binary -> str -> obj
	// binary -> obj (direct)
	cap1 := makeGraphCap(MediaBinary, MediaString, "Binary to String")
	cap2 := makeGraphCap(MediaString, MediaObject, "String to Object")
	cap3 := makeGraphCap(MediaBinary, MediaObject, "Binary to Object (direct)")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2, cap3})

	composite := NewCapCube()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Find all paths from binary to obj
	paths := graph.FindAllPaths(MediaBinary, MediaObject, 3)

	if len(paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(paths))
	}

	// Paths should be sorted by length (shortest first)
	if len(paths[0]) != 1 {
		t.Errorf("First path should have length 1 (direct), got %d", len(paths[0]))
	}
	if len(paths[1]) != 2 {
		t.Errorf("Second path should have length 2 (via str), got %d", len(paths[1]))
	}
}

func TestCapGraphGetDirectEdges(t *testing.T) {
	registry1 := NewCapMatrix()
	registry2 := NewCapMatrix()

	host1 := &MockCapSetForRegistry{name: "converter1"}
	host2 := &MockCapSetForRegistry{name: "converter2"}

	// Two converters: binary -> str with different specificities
	cap1 := makeGraphCap(MediaBinary, MediaString, "Generic Binary to String")

	// More specific converter (with extra tag for higher specificity)
	capUrn2, _ := NewCapUrnFromString(`cap:ext=pdf;in="` + MediaBinary + `";op=convert;out="` + MediaString + `"`)
	cap2 := &Cap{
		Urn:            capUrn2,
		Title:          "PDF Binary to String",
		CapDescription: stringPtr("PDF Binary to String"),
		Metadata:       make(map[string]string),
		Command:        "convert",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}

	registry1.RegisterCapSet("converter1", host1, []*Cap{cap1})
	registry2.RegisterCapSet("converter2", host2, []*Cap{cap2})

	composite := NewCapCube()
	composite.AddRegistry("reg1", registry1)
	composite.AddRegistry("reg2", registry2)

	graph := composite.Graph()

	// Get direct edges (should be sorted by specificity)
	edges := graph.GetDirectEdges(MediaBinary, MediaString)

	if len(edges) != 2 {
		t.Errorf("Expected 2 direct edges, got %d", len(edges))
	}

	// First should be more specific (PDF converter)
	if edges[0].Cap.Title != "PDF Binary to String" {
		t.Errorf("First edge should be more specific, got %s", edges[0].Cap.Title)
	}
	if edges[0].Specificity <= edges[1].Specificity {
		t.Error("First edge should have higher specificity")
	}
}

func TestCapGraphStats(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str -> obj
	//         \-> json
	cap1 := makeGraphCap(MediaBinary, MediaString, "Binary to String")
	cap2 := makeGraphCap(MediaString, MediaObject, "String to Object")
	cap3 := makeGraphCap(MediaBinary, "media:type=json;v=1", "Binary to JSON")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2, cap3})

	composite := NewCapCube()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()
	stats := graph.Stats()

	// 4 unique nodes: binary, str, obj, json
	if stats.NodeCount != 4 {
		t.Errorf("Expected 4 nodes, got %d", stats.NodeCount)
	}

	// 3 edges
	if stats.EdgeCount != 3 {
		t.Errorf("Expected 3 edges, got %d", stats.EdgeCount)
	}

	// 2 input specs (binary, str)
	if stats.InputSpecCount != 2 {
		t.Errorf("Expected 2 input specs, got %d", stats.InputSpecCount)
	}

	// 3 output specs (str, obj, json)
	if stats.OutputSpecCount != 3 {
		t.Errorf("Expected 3 output specs, got %d", stats.OutputSpecCount)
	}
}

func TestCapGraphWithCapCube(t *testing.T) {
	// Integration test: build graph from CapCube
	providerRegistry := NewCapMatrix()
	pluginRegistry := NewCapMatrix()

	providerHost := &MockCapSetForRegistry{name: "provider"}
	pluginHost := &MockCapSetForRegistry{name: "plugin"}

	// Provider: binary -> str
	providerCap := makeGraphCap(MediaBinary, MediaString, "Provider Binary to String")
	providerRegistry.RegisterCapSet("provider", providerHost, []*Cap{providerCap})

	// Plugin: str -> obj
	pluginCap := makeGraphCap(MediaString, MediaObject, "Plugin String to Object")
	pluginRegistry.RegisterCapSet("plugin", pluginHost, []*Cap{pluginCap})

	cube := NewCapCube()
	cube.AddRegistry("providers", providerRegistry)
	cube.AddRegistry("plugins", pluginRegistry)

	graph := cube.Graph()

	// Should be able to convert binary -> obj through both registries
	if !graph.CanConvert(MediaBinary, MediaObject) {
		t.Error("Should be able to convert binary to obj across registries")
	}

	path := graph.FindPath(MediaBinary, MediaObject)
	if path == nil {
		t.Fatal("Expected to find path")
	}
	if len(path) != 2 {
		t.Errorf("Expected path length 2, got %d", len(path))
	}

	// Verify edges come from different registries
	if path[0].RegistryName != "providers" {
		t.Errorf("First edge should be from providers, got %s", path[0].RegistryName)
	}
	if path[1].RegistryName != "plugins" {
		t.Errorf("Second edge should be from plugins, got %s", path[1].RegistryName)
	}
}
