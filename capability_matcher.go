package capdef

import (
	"sort"
)

// CapabilityMatcher provides utilities for finding the best capability match from a collection
// based on specificity and compatibility rules.
type CapabilityMatcher struct{}

// FindBestMatch finds the most specific capability that can handle a request
func (m *CapabilityMatcher) FindBestMatch(capabilities []*CapabilityKey, request *CapabilityKey) *CapabilityKey {
	matches := m.FindAllMatches(capabilities, request)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

// FindAllMatches finds all capabilities that can handle a request
// Returns capabilities sorted by specificity (most specific first)
func (m *CapabilityMatcher) FindAllMatches(capabilities []*CapabilityKey, request *CapabilityKey) []*CapabilityKey {
	var matches []*CapabilityKey

	for _, capability := range capabilities {
		if capability.CanHandle(request) {
			matches = append(matches, capability)
		}
	}

	return m.SortBySpecificity(matches)
}

// SortBySpecificity sorts capabilities by specificity (most specific first)
func (m *CapabilityMatcher) SortBySpecificity(capabilities []*CapabilityKey) []*CapabilityKey {
	sorted := make([]*CapabilityKey, len(capabilities))
	copy(sorted, capabilities)

	sort.Slice(sorted, func(i, j int) bool {
		cap1 := sorted[i]
		cap2 := sorted[j]

		// Sort by specificity level first (higher specificity first)
		spec1 := cap1.SpecificityLevel()
		spec2 := cap2.SpecificityLevel()

		if spec1 != spec2 {
			return spec1 > spec2
		}

		// If same specificity level, sort by segment count (more segments first)
		count1 := len(cap1.segments)
		count2 := len(cap2.segments)

		if count1 != count2 {
			return count1 > count2
		}

		// If same segment count, sort alphabetically for deterministic ordering
		return cap1.ToString() < cap2.ToString()
	})

	return sorted
}

// CanHandleWithContext checks if a capability can handle a request with additional context
func (m *CapabilityMatcher) CanHandleWithContext(capability *CapabilityKey, request *CapabilityKey, context map[string]interface{}) bool {
	// Basic capability matching
	if !capability.CanHandle(request) {
		return false
	}

	// If no context provided, basic matching is sufficient
	if context == nil {
		return true
	}

	// Context-based filtering could be implemented here
	// For example, checking file type compatibility, version requirements, etc.
	// This is extensible for future use cases

	return true
}

// Static methods for convenience

// FindBestMatchStatic is a convenience function for finding the best match without creating a matcher instance
func FindBestMatchStatic(capabilities []*CapabilityKey, request *CapabilityKey) *CapabilityKey {
	matcher := &CapabilityMatcher{}
	return matcher.FindBestMatch(capabilities, request)
}

// FindAllMatchesStatic is a convenience function for finding all matches without creating a matcher instance
func FindAllMatchesStatic(capabilities []*CapabilityKey, request *CapabilityKey) []*CapabilityKey {
	matcher := &CapabilityMatcher{}
	return matcher.FindAllMatches(capabilities, request)
}

// SortBySpecificityStatic is a convenience function for sorting by specificity without creating a matcher instance
func SortBySpecificityStatic(capabilities []*CapabilityKey) []*CapabilityKey {
	matcher := &CapabilityMatcher{}
	return matcher.SortBySpecificity(capabilities)
}