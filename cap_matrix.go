// Package capns provides CapSet registry for unified capability host discovery
package capns

import (
	"context"
	"fmt"
	"sync"
)

// CapMatrixError represents errors that can occur during capability host operations
type CapMatrixError struct {
	Type    string
	Message string
}

func (e *CapMatrixError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewNoSetsFoundError creates a new error for when no sets are found
func NewNoSetsFoundError(capability string) *CapMatrixError {
	return &CapMatrixError{
		Type:    "NoSetsFound",
		Message: fmt.Sprintf("No cap sets found for capability: %s", capability),
	}
}

// NewInvalidUrnError creates a new error for invalid URNs
func NewInvalidUrnError(urn, reason string) *CapMatrixError {
	return &CapMatrixError{
		Type:    "InvalidUrn",
		Message: fmt.Sprintf("Invalid capability URN: %s: %s", urn, reason),
	}
}

// NewRegistryError creates a new general registry error
func NewRegistryError(message string) *CapMatrixError {
	return &CapMatrixError{
		Type:    "RegistryError",
		Message: message,
	}
}

// capSetEntry represents a registered capability host
type capSetEntry struct {
	name         string
	host         CapSet
	capabilities []*Cap
}

// CapMatrix provides unified registry for cap sets (providers and plugins)
type CapMatrix struct {
	sets map[string]*capSetEntry
}

// NewCapMatrix creates a new empty capability host registry
func NewCapMatrix() *CapMatrix {
	return &CapMatrix{
		sets: make(map[string]*capSetEntry),
	}
}

// RegisterCapSet registers a capability host with its supported capabilities
func (r *CapMatrix) RegisterCapSet(name string, host CapSet, capabilities []*Cap) error {
	entry := &capSetEntry{
		name:         name,
		host:         host,
		capabilities: capabilities,
	}
	
	r.sets[name] = entry
	return nil
}

// FindCapSets finds cap sets that can handle the requested capability
// Uses subset matching: host capabilities must be a subset of or match the request
func (r *CapMatrix) FindCapSets(requestUrn string) ([]CapSet, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, NewInvalidUrnError(requestUrn, err.Error())
	}
	
	var matchingHosts []CapSet
	
	for _, entry := range r.sets {
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
		return nil, NewNoSetsFoundError(requestUrn)
	}
	
	return matchingHosts, nil
}

// FindBestCapSet finds the best capability host and cap definition for the request using specificity ranking
func (r *CapMatrix) FindBestCapSet(requestUrn string) (CapSet, *Cap, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, nil, NewInvalidUrnError(requestUrn, err.Error())
	}
	
	var bestHost CapSet
	var bestCap *Cap
	var bestSpecificity int = -1
	
	for _, entry := range r.sets {
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
		return nil, nil, NewNoSetsFoundError(requestUrn)
	}
	
	return bestHost, bestCap, nil
}

// GetHostNames returns all registered capability host names
func (r *CapMatrix) GetHostNames() []string {
	names := make([]string, 0, len(r.sets))
	for name := range r.sets {
		names = append(names, name)
	}
	return names
}

// GetAllCapabilities returns all capabilities from all registered sets
func (r *CapMatrix) GetAllCapabilities() []*Cap {
	var capabilities []*Cap
	for _, entry := range r.sets {
		capabilities = append(capabilities, entry.capabilities...)
	}
	return capabilities
}

// CanHandle checks if any host can handle the specified capability
func (r *CapMatrix) CanHandle(requestUrn string) bool {
	_, err := r.FindCapSets(requestUrn)
	return err == nil
}

// UnregisterCapSet unregisters a capability host
func (r *CapMatrix) UnregisterCapSet(name string) bool {
	if _, exists := r.sets[name]; exists {
		delete(r.sets, name)
		return true
	}
	return false
}

// Clear removes all registered sets
func (r *CapMatrix) Clear() {
	r.sets = make(map[string]*capSetEntry)
}

// ============================================================================
// CapCube - Composite Registry
// ============================================================================

// BestCapSetMatch represents the result of finding the best match across registries
type BestCapSetMatch struct {
	Cap          *Cap   // The Cap definition that matched
	Specificity  int    // The specificity score of the match
	RegistryName string // The name of the registry that provided this match
}

// registryEntry holds a named registry for CapCube
type registryEntry struct {
	name     string
	registry *CapMatrix
}

// CapCube is a composite registry that wraps multiple CapMatrix instances
// and finds the best match across all of them by specificity.
// When multiple registries can handle a request, this registry compares
// specificity scores and returns the most specific match.
// On tie, defaults to the first registry that was added (priority order).
type CapCube struct {
	registries []registryEntry
	mu         sync.RWMutex
}

// CompositeCapSet wraps CapCube registries and implements CapSet interface
// for execution delegation to the best matching registry
type CompositeCapSet struct {
	registries []registryEntry
	mu         *sync.RWMutex
}

// NewCapCube creates a new empty composite registry
func NewCapCube() *CapCube {
	return &CapCube{
		registries: make([]registryEntry, 0),
	}
}

// AddRegistry adds a child registry with a name.
// Registries are checked in order of addition for tie-breaking.
func (c *CapCube) AddRegistry(name string, registry *CapMatrix) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registries = append(c.registries, registryEntry{name: name, registry: registry})
}

// RemoveRegistry removes a child registry by name and returns it
func (c *CapCube) RemoveRegistry(name string) *CapMatrix {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, entry := range c.registries {
		if entry.name == name {
			removed := entry.registry
			c.registries = append(c.registries[:i], c.registries[i+1:]...)
			return removed
		}
	}
	return nil
}

// GetRegistry returns a child registry by name
func (c *CapCube) GetRegistry(name string) *CapMatrix {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, entry := range c.registries {
		if entry.name == name {
			return entry.registry
		}
	}
	return nil
}

// GetRegistryNames returns the names of all child registries
func (c *CapCube) GetRegistryNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, len(c.registries))
	for i, entry := range c.registries {
		names[i] = entry.name
	}
	return names
}

// Can checks if a capability is available and returns a CapCaller.
// This is the main entry point for capability lookup - preserves the can().call() pattern.
// Finds the best (most specific) match across all child registries and returns
// a CapCaller ready to execute the capability.
func (c *CapCube) Can(capUrn string) (*CapCaller, error) {
	// Find the best match to get the cap definition
	bestMatch, err := c.FindBestCapSet(capUrn)
	if err != nil {
		return nil, err
	}

	// Create a CompositeCapSet that will delegate execution to the right registry
	c.mu.RLock()
	registriesCopy := make([]registryEntry, len(c.registries))
	copy(registriesCopy, c.registries)
	c.mu.RUnlock()

	compositeHost := &CompositeCapSet{
		registries: registriesCopy,
		mu:         &c.mu,
	}

	return NewCapCaller(capUrn, compositeHost, bestMatch.Cap), nil
}

// FindBestCapSet finds the best capability host across ALL child registries.
// This method polls all registries and compares their best matches by specificity.
// Returns the cap definition and specificity of the best match.
// On specificity tie, returns the match from the first registry (priority order).
func (c *CapCube) FindBestCapSet(requestUrn string) (*BestCapSetMatch, error) {
	request, err := NewCapUrnFromString(requestUrn)
	if err != nil {
		return nil, NewInvalidUrnError(requestUrn, err.Error())
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	var bestOverall *BestCapSetMatch

	for _, entry := range c.registries {
		// Find the best match within this registry
		cap, specificity := c.findBestInRegistry(entry.registry, request)
		if cap != nil {
			candidate := &BestCapSetMatch{
				Cap:          cap,
				Specificity:  specificity,
				RegistryName: entry.name,
			}

			if bestOverall == nil {
				bestOverall = candidate
			} else if specificity > bestOverall.Specificity {
				// Only replace if strictly more specific
				// On tie, keep the first one (priority order)
				bestOverall = candidate
			}
		}
	}

	if bestOverall == nil {
		return nil, NewNoSetsFoundError(requestUrn)
	}

	return bestOverall, nil
}

// CanHandle checks if any registry can handle the specified capability
func (c *CapCube) CanHandle(requestUrn string) bool {
	_, err := c.FindBestCapSet(requestUrn)
	return err == nil
}

// findBestInRegistry finds the best match within a single registry
// Returns (Cap, specificity) for the best match, or (nil, 0) if no match
func (c *CapCube) findBestInRegistry(registry *CapMatrix, request *CapUrn) (*Cap, int) {
	var bestCap *Cap
	bestSpecificity := -1

	for _, entry := range registry.sets {
		for _, cap := range entry.capabilities {
			if cap.Urn.Matches(request) {
				specificity := cap.Urn.Specificity()
				if bestSpecificity == -1 || specificity > bestSpecificity {
					bestCap = cap
					bestSpecificity = specificity
				}
				break // Found match for this entry, check next entry
			}
		}
	}

	if bestCap == nil {
		return nil, 0
	}
	return bestCap, bestSpecificity
}

// ExecuteCap implements the CapSet interface for CompositeCapSet.
// Finds the best match across all registries and delegates execution.
func (cs *CompositeCapSet) ExecuteCap(
	ctx context.Context,
	capUrn string,
	positionalArgs []string,
	namedArgs map[string]string,
	stdinData []byte,
) (*HostResult, error) {
	// Parse the request URN
	request, err := NewCapUrnFromString(capUrn)
	if err != nil {
		return nil, fmt.Errorf("invalid cap URN '%s': %w", capUrn, err)
	}

	// Find the best matching CapSet across all registries
	var bestHost CapSet
	bestSpecificity := -1

	for _, entry := range cs.registries {
		for _, setEntry := range entry.registry.sets {
			for _, cap := range setEntry.capabilities {
				if cap.Urn.Matches(request) {
					specificity := cap.Urn.Specificity()
					if bestSpecificity == -1 || specificity > bestSpecificity {
						bestHost = setEntry.host
						bestSpecificity = specificity
					}
					break // Found match for this entry, check next entry
				}
			}
		}
	}

	if bestHost == nil {
		return nil, fmt.Errorf("no capability host found for '%s'", capUrn)
	}

	// Delegate execution to the best matching host
	return bestHost.ExecuteCap(ctx, capUrn, positionalArgs, namedArgs, stdinData)
}

// Graph builds a directed graph from all capabilities in the registries.
// This enables discovering conversion paths between different media formats.
func (cs *CompositeCapSet) Graph() *CapGraph {
	return BuildCapGraphFromRegistries(cs.registries)
}

// ============================================================================
// CapGraph - Directed graph of capability conversions
// ============================================================================

// CapGraphEdge represents a conversion from one MediaSpec to another.
// Each edge corresponds to a capability that can transform data.
type CapGraphEdge struct {
	FromSpec     string // The input MediaSpec ID (e.g., "media:type=binary;v=1")
	ToSpec       string // The output MediaSpec ID (e.g., "media:type=string;v=1")
	Cap          *Cap   // The capability that performs this conversion
	RegistryName string // The registry that provided this capability
	Specificity  int    // Specificity score for ranking multiple paths
}

// CapGraphStats provides statistics about a capability graph.
type CapGraphStats struct {
	NodeCount       int // Number of unique MediaSpec nodes
	EdgeCount       int // Number of edges (capabilities)
	InputSpecCount  int // Number of specs that serve as inputs
	OutputSpecCount int // Number of specs that serve as outputs
}

// CapGraph is a directed graph where nodes are MediaSpec IDs and edges are capabilities.
// This graph enables discovering conversion paths between different media formats.
type CapGraph struct {
	edges    []CapGraphEdge
	outgoing map[string][]int // from_spec -> indices into edges
	incoming map[string][]int // to_spec -> indices into edges
	nodes    map[string]bool  // All unique spec IDs
}

// NewCapGraph creates a new empty capability graph.
func NewCapGraph() *CapGraph {
	return &CapGraph{
		edges:    make([]CapGraphEdge, 0),
		outgoing: make(map[string][]int),
		incoming: make(map[string][]int),
		nodes:    make(map[string]bool),
	}
}

// AddCap adds a capability as an edge in the graph.
// The cap's in_spec becomes the source node and out_spec becomes the target node.
func (g *CapGraph) AddCap(cap *Cap, registryName string) {
	fromSpec := cap.Urn.InSpec()
	toSpec := cap.Urn.OutSpec()
	specificity := cap.Urn.Specificity()

	// Add nodes
	g.nodes[fromSpec] = true
	g.nodes[toSpec] = true

	// Create edge
	edgeIndex := len(g.edges)
	edge := CapGraphEdge{
		FromSpec:     fromSpec,
		ToSpec:       toSpec,
		Cap:          cap,
		RegistryName: registryName,
		Specificity:  specificity,
	}
	g.edges = append(g.edges, edge)

	// Update indices
	g.outgoing[fromSpec] = append(g.outgoing[fromSpec], edgeIndex)
	g.incoming[toSpec] = append(g.incoming[toSpec], edgeIndex)
}

// BuildCapGraphFromRegistries builds a graph from multiple registries.
func BuildCapGraphFromRegistries(registries []registryEntry) *CapGraph {
	graph := NewCapGraph()

	for _, entry := range registries {
		for _, setEntry := range entry.registry.sets {
			for _, cap := range setEntry.capabilities {
				graph.AddCap(cap, entry.name)
			}
		}
	}

	return graph
}

// GetNodes returns all nodes (MediaSpec IDs) in the graph.
func (g *CapGraph) GetNodes() []string {
	nodes := make([]string, 0, len(g.nodes))
	for node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetEdges returns all edges in the graph.
func (g *CapGraph) GetEdges() []CapGraphEdge {
	return g.edges
}

// GetOutgoing returns all edges where the provided spec satisfies the edge's input requirement.
// Uses satisfies-based matching: checks if spec can satisfy each edge's FromSpec requirement.
func (g *CapGraph) GetOutgoing(spec string) []*CapGraphEdge {
	var edges []*CapGraphEdge

	// Iterate all edges and check which ones the provided spec satisfies
	for i := range g.edges {
		edge := &g.edges[i]
		if MediaUrnSatisfies(spec, edge.FromSpec) {
			edges = append(edges, edge)
		}
	}

	// Sort by specificity (highest first) for consistent ordering
	for i := 0; i < len(edges)-1; i++ {
		for j := i + 1; j < len(edges); j++ {
			if edges[j].Specificity > edges[i].Specificity {
				edges[i], edges[j] = edges[j], edges[i]
			}
		}
	}

	return edges
}

// GetIncoming returns all edges targeting a spec.
func (g *CapGraph) GetIncoming(spec string) []*CapGraphEdge {
	indices := g.incoming[spec]
	edges := make([]*CapGraphEdge, len(indices))
	for i, idx := range indices {
		edges[i] = &g.edges[idx]
	}
	return edges
}

// HasDirectEdge checks if there's any direct edge from one spec to another.
func (g *CapGraph) HasDirectEdge(fromSpec, toSpec string) bool {
	for _, edge := range g.GetOutgoing(fromSpec) {
		if edge.ToSpec == toSpec {
			return true
		}
	}
	return false
}

// GetDirectEdges returns all direct edges from one spec to another, sorted by specificity (highest first).
func (g *CapGraph) GetDirectEdges(fromSpec, toSpec string) []*CapGraphEdge {
	var edges []*CapGraphEdge
	for _, edge := range g.GetOutgoing(fromSpec) {
		if edge.ToSpec == toSpec {
			edges = append(edges, edge)
		}
	}

	// Sort by specificity (highest first)
	for i := 0; i < len(edges)-1; i++ {
		for j := i + 1; j < len(edges); j++ {
			if edges[j].Specificity > edges[i].Specificity {
				edges[i], edges[j] = edges[j], edges[i]
			}
		}
	}

	return edges
}

// CanConvert checks if a conversion path exists from one spec to another.
// Uses BFS to find if there's any path (direct or through intermediates).
func (g *CapGraph) CanConvert(fromSpec, toSpec string) bool {
	if fromSpec == toSpec {
		return true
	}

	if !g.nodes[fromSpec] || !g.nodes[toSpec] {
		return false
	}

	visited := make(map[string]bool)
	queue := []string{fromSpec}
	visited[fromSpec] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range g.GetOutgoing(current) {
			if edge.ToSpec == toSpec {
				return true
			}
			if !visited[edge.ToSpec] {
				visited[edge.ToSpec] = true
				queue = append(queue, edge.ToSpec)
			}
		}
	}

	return false
}

// FindPath finds the shortest conversion path from one spec to another.
// Returns a sequence of edges representing the conversion chain, or nil if no path exists.
func (g *CapGraph) FindPath(fromSpec, toSpec string) []*CapGraphEdge {
	if fromSpec == toSpec {
		return []*CapGraphEdge{}
	}

	if !g.nodes[fromSpec] || !g.nodes[toSpec] {
		return nil
	}

	// BFS to find shortest path
	// visited maps spec -> (previous spec, edge index used to reach it)
	type backtrackInfo struct {
		prevSpec string
		edgeIdx  int
	}
	visited := make(map[string]*backtrackInfo)
	queue := []string{fromSpec}
	visited[fromSpec] = nil

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		indices := g.outgoing[current]
		for _, edgeIdx := range indices {
			edge := &g.edges[edgeIdx]

			if edge.ToSpec == toSpec {
				// Found the target - reconstruct path
				path := []*CapGraphEdge{&g.edges[edgeIdx]}

				backtrack := current
				for visited[backtrack] != nil {
					info := visited[backtrack]
					path = append(path, &g.edges[info.edgeIdx])
					backtrack = info.prevSpec
				}

				// Reverse the path
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}

			if _, exists := visited[edge.ToSpec]; !exists {
				visited[edge.ToSpec] = &backtrackInfo{prevSpec: current, edgeIdx: edgeIdx}
				queue = append(queue, edge.ToSpec)
			}
		}
	}

	return nil
}

// FindAllPaths finds all conversion paths from one spec to another (up to a maximum depth).
// Returns all possible paths, sorted by total path length (shortest first).
func (g *CapGraph) FindAllPaths(fromSpec, toSpec string, maxDepth int) [][]*CapGraphEdge {
	if !g.nodes[fromSpec] || !g.nodes[toSpec] {
		return nil
	}

	var allPaths [][]int
	currentPath := []int{}
	visited := make(map[string]bool)

	g.dfsFindPaths(fromSpec, toSpec, maxDepth, currentPath, visited, &allPaths)

	// Sort by path length (shortest first)
	for i := 0; i < len(allPaths)-1; i++ {
		for j := i + 1; j < len(allPaths); j++ {
			if len(allPaths[j]) < len(allPaths[i]) {
				allPaths[i], allPaths[j] = allPaths[j], allPaths[i]
			}
		}
	}

	// Convert indices to edge references
	result := make([][]*CapGraphEdge, len(allPaths))
	for i, indices := range allPaths {
		path := make([]*CapGraphEdge, len(indices))
		for j, idx := range indices {
			path[j] = &g.edges[idx]
		}
		result[i] = path
	}

	return result
}

// dfsFindPaths is a DFS helper for finding all paths
func (g *CapGraph) dfsFindPaths(current, target string, remainingDepth int, currentPath []int, visited map[string]bool, allPaths *[][]int) {
	if remainingDepth == 0 {
		return
	}

	indices := g.outgoing[current]
	for _, edgeIdx := range indices {
		edge := &g.edges[edgeIdx]

		if edge.ToSpec == target {
			// Found a path
			path := make([]int, len(currentPath)+1)
			copy(path, currentPath)
			path[len(currentPath)] = edgeIdx
			*allPaths = append(*allPaths, path)
		} else if !visited[edge.ToSpec] {
			// Continue searching
			visited[edge.ToSpec] = true
			g.dfsFindPaths(edge.ToSpec, target, remainingDepth-1, append(currentPath, edgeIdx), visited, allPaths)
			delete(visited, edge.ToSpec)
		}
	}
}

// FindBestPath finds the best (highest specificity) conversion path from one spec to another.
// Unlike FindPath which finds the shortest path, this finds the path with
// the highest total specificity score (sum of all edge specificities).
func (g *CapGraph) FindBestPath(fromSpec, toSpec string, maxDepth int) []*CapGraphEdge {
	allPaths := g.FindAllPaths(fromSpec, toSpec, maxDepth)

	if len(allPaths) == 0 {
		return nil
	}

	var bestPath []*CapGraphEdge
	bestScore := -1

	for _, path := range allPaths {
		score := 0
		for _, edge := range path {
			score += edge.Specificity
		}
		if score > bestScore {
			bestScore = score
			bestPath = path
		}
	}

	return bestPath
}

// GetInputSpecs returns all specs that have at least one outgoing edge.
func (g *CapGraph) GetInputSpecs() []string {
	specs := make([]string, 0, len(g.outgoing))
	for spec := range g.outgoing {
		specs = append(specs, spec)
	}
	return specs
}

// GetOutputSpecs returns all specs that have at least one incoming edge.
func (g *CapGraph) GetOutputSpecs() []string {
	specs := make([]string, 0, len(g.incoming))
	for spec := range g.incoming {
		specs = append(specs, spec)
	}
	return specs
}

// Stats returns statistics about the graph.
func (g *CapGraph) Stats() CapGraphStats {
	return CapGraphStats{
		NodeCount:       len(g.nodes),
		EdgeCount:       len(g.edges),
		InputSpecCount:  len(g.outgoing),
		OutputSpecCount: len(g.incoming),
	}
}

// Graph builds a directed graph from all capabilities across all registries.
// This enables discovering conversion paths between different media formats.
func (c *CapCube) Graph() *CapGraph {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return BuildCapGraphFromRegistries(c.registries)
}