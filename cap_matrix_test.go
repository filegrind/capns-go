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
	stdinData []byte,
) (*HostResult, error) {
	return &HostResult{
		TextOutput: "Mock response from " + m.name,
	}, nil
}

func TestRegisterAndFindCapSet(t *testing.T) {
	registry := NewCapMatrix()
	
	host := &MockCapSetForRegistry{name: "test-host"}
	
	capUrn, err := NewCapUrnFromString("cap:op=test;type=basic")
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
	hosts, err := registry.FindCapSets("cap:op=test;type=basic")
	if err != nil {
		t.Fatalf("Failed to find cap hosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(hosts))
	}
	
	// Test subset match (request has more specific requirements)
	hosts, err = registry.FindCapSets("cap:op=test;type=basic;model=gpt-4")
	if err != nil {
		t.Fatalf("Failed to find cap hosts for subset match: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host for subset match, got %d", len(hosts))
	}
	
	// Test no match
	_, err = registry.FindCapSets("cap:op=different")
	if err == nil {
		t.Error("Expected error for non-matching capability, got nil")
	}
}

func TestBestCapSetSelection(t *testing.T) {
	registry := NewCapMatrix()
	
	// Register general host
	generalHost := &MockCapSetForRegistry{name: "general"}
	generalCapUrn, _ := NewCapUrnFromString("cap:op=generate")
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
	specificCapUrn, _ := NewCapUrnFromString("cap:op=generate;type=text;model=gpt-4")
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
	bestHost, bestCap, err := registry.FindBestCapSet("cap:op=generate;type=text;model=gpt-4;temperature=0.7")
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
	
	// Both hosts should match
	allHosts, err := registry.FindCapSets("cap:op=generate;type=text;model=gpt-4;temperature=0.7")
	if err != nil {
		t.Fatalf("Failed to find all matching hosts: %v", err)
	}
	if len(allHosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(allHosts))
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
	if registry.CanHandle("cap:op=test") {
		t.Error("Empty registry should not handle any capability")
	}
	
	// After registration
	host := &MockCapSetForRegistry{name: "test"}
	capUrn, _ := NewCapUrnFromString("cap:op=test")
	cap := &Cap{
		Urn:            capUrn,
		CapDescription: stringPtr("Test"),
		Metadata:       make(map[string]string),
		Command:        "test",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}
	
	registry.RegisterCapSet("test", host, []*Cap{cap})
	
	if !registry.CanHandle("cap:op=test") {
		t.Error("Registry should handle registered capability")
	}
	if !registry.CanHandle("cap:op=test;extra=param") {
		t.Error("Registry should handle capability with extra parameters")
	}
	if registry.CanHandle("cap:op=different") {
		t.Error("Registry should not handle unregistered capability")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}