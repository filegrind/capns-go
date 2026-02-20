package capns

import (
	"context"
	"testing"

	"github.com/filegrind/capns-go/standard"
)

// MockCapSetForRegistry for testing (avoid conflict with existing MockCapSet)
type MockCapSetForRegistry struct {
	name string
}

func (m *MockCapSetForRegistry) ExecuteCap(
	ctx context.Context,
	capUrn string,
	arguments []CapArgumentValue,
) (*HostResult, error) {
	return &HostResult{
		TextOutput: "Mock response from " + m.name,
	}, nil
}

// Test helper for matrix tests
func matrixTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:void";out="media:object"`
	}
	return `cap:in="media:void";out="media:object";` + tags
}

// TEST117: Test registering cap set and finding by exact and subset matching
func Test117_register_and_find_cap_set(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "test-host"}

	capUrn, err := NewCapUrnFromString(matrixTestUrn("op=test;basic"))
	if err != nil {
		t.Fatalf("Failed to create CapUrn: %v", err)
	}

	cap := &Cap{
		Urn:            capUrn,
		CapDescription: stringPtr("Test capability"),
		Metadata:       make(map[string]string),
		Command:        "test",
		Args:           []CapArg{},
		Output:         nil,
	}

	err = registry.RegisterCapSet("test-host", host, []*Cap{cap})
	if err != nil {
		t.Fatalf("Failed to register cap host: %v", err)
	}

	// Test exact match
	sets, err := registry.FindCapSets(matrixTestUrn("op=test;basic"))
	if err != nil {
		t.Fatalf("Failed to find cap sets: %v", err)
	}
	if len(sets) != 1 {
		t.Errorf("Expected 1 host, got %d", len(sets))
	}

	// Test subset match: request has MORE tags than the cap
	// Cap registered: op=test;basic
	// Request: model=gpt-4;op=test;basic
	// Cap missing model tag → implicit wildcard → SHOULD MATCH (Rust semantics)
	sets, err = registry.FindCapSets(matrixTestUrn("model=gpt-4;op=test;basic"))
	if err != nil {
		t.Errorf("Expected match for request with extra tags (cap missing tag = implicit wildcard): %v", err)
	}
	if len(sets) != 1 {
		t.Errorf("Expected 1 match for request with extra tags, got %d", len(sets))
	}

	// Test no match (different direction specs)
	_, err = registry.FindCapSets(`cap:in="media:binary";op=different;out="media:object"`)
	if err == nil {
		t.Error("Expected error for non-matching capability, got nil")
	}
}

// TEST118: Test selecting best cap set based on specificity ranking
func Test118_best_cap_set_selection(t *testing.T) {
	registry := NewCapMatrix()

	// Register general host with explicit wildcards for flexibility
	generalHost := &MockCapSetForRegistry{name: "general"}
	generalCapUrn, _ := NewCapUrnFromString(matrixTestUrn("model=*;op=generate;text=*"))
	generalCap := &Cap{
		Urn:            generalCapUrn,
		CapDescription: stringPtr("General generation"),
		Metadata:       make(map[string]string),
		Command:        "generate",
		Args:           []CapArg{},
		Output:         nil,
	}

	// Register specific host
	specificHost := &MockCapSetForRegistry{name: "specific"}
	specificCapUrn, _ := NewCapUrnFromString(matrixTestUrn("model=gpt-4;op=generate;text"))
	specificCap := &Cap{
		Urn:            specificCapUrn,
		CapDescription: stringPtr("Specific text generation"),
		Metadata:       make(map[string]string),
		Command:        "generate",
		Args:           []CapArg{},
		Output:         nil,
	}

	registry.RegisterCapSet("general", generalHost, []*Cap{generalCap})
	registry.RegisterCapSet("specific", specificHost, []*Cap{specificCap})

	// Request for specific model - both match but specific wins due to higher specificity
	// General: in=void(1) + out=object(1) + model=*(2) + op=generate(3) + text=*(2) = 9
	// Specific: in=void(1) + out=object(1) + model=gpt-4(3) + op=generate(3) + text(2) = 10
	bestHost, bestCap, err := registry.FindBestCapSet(matrixTestUrn("model=gpt-4;op=generate;text"))
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

	// Both sets should match the request
	allHosts, err := registry.FindCapSets(matrixTestUrn("model=gpt-4;op=generate;text"))
	if err != nil {
		t.Fatalf("Failed to find all matching sets: %v", err)
	}
	if len(allHosts) != 2 {
		t.Errorf("Expected 2 sets, got %d", len(allHosts))
	}
}

// TEST119: Test invalid URN returns InvalidUrn error
func Test119_invalid_urn_handling(t *testing.T) {
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

// TEST120: Test accepts_request checks if registry can handle a capability request
func Test120_accepts_request(t *testing.T) {
	registry := NewCapMatrix()

	// Empty registry
	if registry.AcceptsRequest(matrixTestUrn("op=test")) {
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
		Args:           []CapArg{},
		Output:         nil,
	}

	registry.RegisterCapSet("test", host, []*Cap{cap})

	if !registry.AcceptsRequest(matrixTestUrn("op=test")) {
		t.Error("Registry should handle registered capability")
	}
	// Cap registered: op=test
	// Request: extra=param;op=test
	// Cap missing extra tag → implicit wildcard → SHOULD MATCH (Rust semantics)
	if !registry.AcceptsRequest(matrixTestUrn("extra=param;op=test")) {
		t.Error("Registry should handle capability (cap missing extra tag = implicit wildcard)")
	}
	if registry.AcceptsRequest(matrixTestUrn("op=different")) {
		t.Error("Registry should not handle unregistered capability")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}

// ============================================================================
// CapBlock Tests
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
		Args:           []CapArg{},
		Output:         nil,
	}
}

// TEST121: Test CapBlock selects more specific cap over less specific regardless of registry order
func Test121_cap_block_more_specific_wins(t *testing.T) {
	// This is the key test: provider has less specific cap, plugin has more specific
	// The more specific one should win regardless of registry order

	providerRegistry := NewCapMatrix()
	pluginRegistry := NewCapMatrix()

	// Provider: less specific cap (no ext tag)
	providerHost := &MockCapSetForRegistry{name: "provider"}
	providerCap := makeCap(
		`cap:in="media:binary";op=generate_thumbnail;out="media:binary"`,
		"Provider Thumbnail Generator (generic)",
	)
	providerRegistry.RegisterCapSet("provider", providerHost, []*Cap{providerCap})

	// Plugin: more specific cap (has ext=pdf)
	pluginHost := &MockCapSetForRegistry{name: "plugin"}
	pluginCap := makeCap(
		`cap:ext=pdf;in="media:binary";op=generate_thumbnail;out="media:binary"`,
		"Plugin PDF Thumbnail Generator (specific)",
	)
	pluginRegistry.RegisterCapSet("plugin", pluginHost, []*Cap{pluginCap})

	// Create composite with provider first (normally would have priority on ties)
	composite := NewCapBlock()
	composite.AddRegistry("providers", providerRegistry)
	composite.AddRegistry("plugins", pluginRegistry)

	// Request for PDF thumbnails - plugin's more specific cap should win
	request := `cap:ext=pdf;in="media:binary";op=generate_thumbnail;out="media:binary"`
	best, err := composite.FindBestCapSet(request)
	if err != nil {
		t.Fatalf("Failed to find best cap set: %v", err)
	}

	// Plugin: in=bytes(1) + out=bytes(1) + ext=pdf(3) + op=generate_thumbnail(3) = 8
	// Provider: in=bytes(1) + out=bytes(1) + op=generate_thumbnail(3) = 5
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

// TEST122: Test CapBlock breaks specificity ties by first registered registry
func Test122_cap_block_tie_goes_to_first(t *testing.T) {
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

	composite := NewCapBlock()
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

// TEST123: Test CapBlock polls all registries to find most specific match
func Test123_cap_block_polls_all(t *testing.T) {
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

	composite := NewCapBlock()
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

// TEST124: Test CapBlock returns error when no registries match the request
func Test124_cap_block_no_match(t *testing.T) {
	registry := NewCapMatrix()

	composite := NewCapBlock()
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

// TEST125: Test CapBlock prefers specific plugin over generic provider fallback
func Test125_cap_block_fallback_scenario(t *testing.T) {
	// Test the exact scenario from the user's issue:
	// Provider: generic fallback with ext=* (can handle any file type)
	// Plugin:   PDF-specific handler
	// Request:  PDF thumbnail
	// Expected: Plugin wins (more specific)

	providerRegistry := NewCapMatrix()
	pluginRegistry := NewCapMatrix()

	// Provider with generic fallback - uses ext=* to accept any extension
	providerHost := &MockCapSetForRegistry{name: "provider_fallback"}
	providerCap := makeCap(
		`cap:ext=*;in="media:binary";op=generate_thumbnail;out="media:binary"`,
		"Generic Thumbnail Provider",
	)
	providerRegistry.RegisterCapSet("provider_fallback", providerHost, []*Cap{providerCap})

	// Plugin with PDF-specific handler
	pluginHost := &MockCapSetForRegistry{name: "pdf_plugin"}
	pluginCap := makeCap(
		`cap:ext=pdf;in="media:binary";op=generate_thumbnail;out="media:binary"`,
		"PDF Thumbnail Plugin",
	)
	pluginRegistry.RegisterCapSet("pdf_plugin", pluginHost, []*Cap{pluginCap})

	// Providers first (would win on tie)
	composite := NewCapBlock()
	composite.AddRegistry("providers", providerRegistry)
	composite.AddRegistry("plugins", pluginRegistry)

	// Request for PDF thumbnail
	request := `cap:ext=pdf;in="media:binary";op=generate_thumbnail;out="media:binary"`
	best, err := composite.FindBestCapSet(request)
	if err != nil {
		t.Fatalf("Failed to find best cap set: %v", err)
	}

	// Plugin: in=bytes(1) + out=bytes(1) + ext=pdf(3) + op=generate_thumbnail(3) = 8
	// Provider: in=bytes(1) + out=bytes(1) + ext=*(2) + op=generate_thumbnail(3) = 7
	if best.RegistryName != "plugins" {
		t.Errorf("Expected plugins to win, got %s", best.RegistryName)
	}
	if best.Cap.Title != "PDF Thumbnail Plugin" {
		t.Errorf("Expected PDF Thumbnail Plugin, got %s", best.Cap.Title)
	}
	if best.Specificity != 4 {
		t.Errorf("Expected specificity 4, got %d", best.Specificity)
	}

	// Also test that for a different file type, provider wins (since plugin doesn't match ext=wav)
	requestWav := `cap:ext=wav;in="media:binary";op=generate_thumbnail;out="media:binary"`
	bestWav, err := composite.FindBestCapSet(requestWav)
	if err != nil {
		t.Fatalf("Failed to find best cap set for wav: %v", err)
	}

	// Only provider matches (plugin has ext=pdf which doesn't match ext=wav)
	// Provider has ext=* which matches any ext value
	if bestWav.RegistryName != "providers" {
		t.Errorf("Expected providers for wav request, got %s", bestWav.RegistryName)
	}
	if bestWav.Cap.Title != "Generic Thumbnail Provider" {
		t.Errorf("Expected Generic Thumbnail Provider, got %s", bestWav.Cap.Title)
	}
}

// TEST126: Test composite can method returns CapCaller for capability execution
func Test126_cap_block_can_method(t *testing.T) {
	// Test the can() method that returns a CapCaller

	providerRegistry := NewCapMatrix()

	providerHost := &MockCapSetForRegistry{name: "test_provider"}
	providerCap := makeCap(matrixTestUrn("ext=pdf;op=generate"), "Test Provider")
	providerRegistry.RegisterCapSet("test_provider", providerHost, []*Cap{providerCap})

	composite := NewCapBlock()
	composite.AddRegistry("providers", providerRegistry)

	// Test can() returns a CapCaller
	caller, err := composite.Can(matrixTestUrn("ext=pdf;op=generate"))
	if err != nil {
		t.Fatalf("Can() failed: %v", err)
	}
	if caller == nil {
		t.Error("Expected CapCaller, got nil")
	}

	// Verify we got the right cap via AcceptsRequest checks
	if !composite.AcceptsRequest(matrixTestUrn("ext=pdf;op=generate")) {
		t.Error("Expected AcceptsRequest to return true for matching cap")
	}
	if composite.AcceptsRequest(matrixTestUrn("op=nonexistent")) {
		t.Error("Expected AcceptsRequest to return false for non-matching cap")
	}
}

func TestCapBlockRegistryManagement(t *testing.T) {
	composite := NewCapBlock()

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
		Args:           []CapArg{},
		Output:         nil,
	}
}

// TEST127: Test CapGraph adds nodes and edges from capability definitions
func Test127_cap_graph_basic_construction(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// Add caps that form a graph:
	// binary -> str -> obj
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Binary to String")
	cap2 := makeGraphCap(standard.MediaString, standard.MediaObject, "String to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapBlock()
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

// TEST128: Test CapGraph tracks outgoing and incoming edges for spec conversions
func Test128_cap_graph_outgoing_incoming(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str, binary -> obj
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Binary to String")
	cap2 := makeGraphCap(standard.MediaBinary, standard.MediaObject, "Binary to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapBlock()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// binary has 2 outgoing edges
	outgoing := graph.GetOutgoing(standard.MediaBinary)
	if len(outgoing) != 2 {
		t.Errorf("Expected 2 outgoing edges from binary, got %d", len(outgoing))
	}

	// str has 1 incoming edge
	incoming := graph.GetIncoming(standard.MediaString)
	if len(incoming) != 1 {
		t.Errorf("Expected 1 incoming edge to str, got %d", len(incoming))
	}

	// obj has 1 incoming edge
	incoming = graph.GetIncoming(standard.MediaObject)
	if len(incoming) != 1 {
		t.Errorf("Expected 1 incoming edge to obj, got %d", len(incoming))
	}
}

// TEST129: Test CapGraph detects direct and indirect conversion paths between specs
func Test129_cap_graph_can_convert(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str -> obj
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Binary to String")
	cap2 := makeGraphCap(standard.MediaString, standard.MediaObject, "String to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapBlock()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Direct conversions
	if !graph.CanConvert(standard.MediaBinary, standard.MediaString) {
		t.Error("Should be able to convert binary to str")
	}
	if !graph.CanConvert(standard.MediaString, standard.MediaObject) {
		t.Error("Should be able to convert str to obj")
	}

	// Transitive conversion
	if !graph.CanConvert(standard.MediaBinary, standard.MediaObject) {
		t.Error("Should be able to convert binary to obj (transitively)")
	}

	// Same spec
	if !graph.CanConvert(standard.MediaBinary, standard.MediaBinary) {
		t.Error("Should be able to convert same spec to itself")
	}

	// Impossible conversions
	if graph.CanConvert(standard.MediaObject, standard.MediaBinary) {
		t.Error("Should not be able to convert obj to binary (no reverse edge)")
	}
	if graph.CanConvert("media:nonexistent", standard.MediaString) {
		t.Error("Should not be able to convert non-existent spec")
	}
}

// TEST130: Test CapGraph finds shortest path for spec conversion chain
func Test130_cap_graph_find_path(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str -> obj
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Binary to String")
	cap2 := makeGraphCap(standard.MediaString, standard.MediaObject, "String to Object")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2})

	composite := NewCapBlock()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Direct path
	path := graph.FindPath(standard.MediaBinary, standard.MediaString)
	if path == nil {
		t.Fatal("Expected to find path from binary to str")
	}
	if len(path) != 1 {
		t.Errorf("Expected path length 1, got %d", len(path))
	}

	// Transitive path
	path = graph.FindPath(standard.MediaBinary, standard.MediaObject)
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
	path = graph.FindPath(standard.MediaObject, standard.MediaBinary)
	if path != nil {
		t.Error("Expected nil for impossible path")
	}

	// Same spec
	path = graph.FindPath(standard.MediaBinary, standard.MediaBinary)
	if path == nil {
		t.Fatal("Expected empty path for same spec")
	}
	if len(path) != 0 {
		t.Errorf("Expected empty path for same spec, got length %d", len(path))
	}
}

// TEST131: Test CapGraph finds all conversion paths sorted by length
func Test131_cap_graph_find_all_paths(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// Create a graph with multiple paths:
	// binary -> str -> obj
	// binary -> obj (direct)
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Binary to String")
	cap2 := makeGraphCap(standard.MediaString, standard.MediaObject, "String to Object")
	cap3 := makeGraphCap(standard.MediaBinary, standard.MediaObject, "Binary to Object (direct)")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2, cap3})

	composite := NewCapBlock()
	composite.AddRegistry("converters", registry)

	graph := composite.Graph()

	// Find all paths from binary to obj
	paths := graph.FindAllPaths(standard.MediaBinary, standard.MediaObject, 3)

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

// TEST132: Test CapGraph returns direct edges sorted by specificity
func Test132_cap_graph_get_direct_edges(t *testing.T) {
	registry1 := NewCapMatrix()
	registry2 := NewCapMatrix()

	host1 := &MockCapSetForRegistry{name: "converter1"}
	host2 := &MockCapSetForRegistry{name: "converter2"}

	// Two converters: binary -> str with different specificities
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Generic Binary to String")

	// More specific converter (with extra tag for higher specificity)
	capUrn2, _ := NewCapUrnFromString(`cap:ext=pdf;in="` + standard.MediaBinary + `";op=convert;out="` + standard.MediaString + `"`)
	cap2 := &Cap{
		Urn:            capUrn2,
		Title:          "PDF Binary to String",
		CapDescription: stringPtr("PDF Binary to String"),
		Metadata:       make(map[string]string),
		Command:        "convert",
		Args:           []CapArg{},
		Output:         nil,
	}

	registry1.RegisterCapSet("converter1", host1, []*Cap{cap1})
	registry2.RegisterCapSet("converter2", host2, []*Cap{cap2})

	composite := NewCapBlock()
	composite.AddRegistry("reg1", registry1)
	composite.AddRegistry("reg2", registry2)

	graph := composite.Graph()

	// Get direct edges (should be sorted by specificity)
	edges := graph.GetDirectEdges(standard.MediaBinary, standard.MediaString)

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

// TEST134: Test CapGraph stats provides counts of nodes and edges
func Test134_cap_graph_stats(t *testing.T) {
	registry := NewCapMatrix()

	host := &MockCapSetForRegistry{name: "converter"}

	// binary -> str -> obj
	//         \-> json
	cap1 := makeGraphCap(standard.MediaBinary, standard.MediaString, "Binary to String")
	cap2 := makeGraphCap(standard.MediaString, standard.MediaObject, "String to Object")
	cap3 := makeGraphCap(standard.MediaBinary, "media:json", "Binary to JSON")

	registry.RegisterCapSet("converter", host, []*Cap{cap1, cap2, cap3})

	composite := NewCapBlock()
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

// TEST133: Test CapBlock graph integration with multiple registries and conversion paths
func Test133_cap_graph_with_cap_block(t *testing.T) {
	// Integration test: build graph from CapBlock
	providerRegistry := NewCapMatrix()
	pluginRegistry := NewCapMatrix()

	providerHost := &MockCapSetForRegistry{name: "provider"}
	pluginHost := &MockCapSetForRegistry{name: "plugin"}

	// Provider: binary -> str
	providerCap := makeGraphCap(standard.MediaBinary, standard.MediaString, "Provider Binary to String")
	providerRegistry.RegisterCapSet("provider", providerHost, []*Cap{providerCap})

	// Plugin: str -> obj
	pluginCap := makeGraphCap(standard.MediaString, standard.MediaObject, "Plugin String to Object")
	pluginRegistry.RegisterCapSet("plugin", pluginHost, []*Cap{pluginCap})

	cube := NewCapBlock()
	cube.AddRegistry("providers", providerRegistry)
	cube.AddRegistry("plugins", pluginRegistry)

	graph := cube.Graph()

	// Should be able to convert binary -> obj through both registries
	if !graph.CanConvert(standard.MediaBinary, standard.MediaObject) {
		t.Error("Should be able to convert binary to obj across registries")
	}

	path := graph.FindPath(standard.MediaBinary, standard.MediaObject)
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
