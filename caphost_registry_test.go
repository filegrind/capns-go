package capns

import (
	"context"
	"testing"
)

// MockCapHostForRegistry for testing (avoid conflict with existing MockCapHost)
type MockCapHostForRegistry struct {
	name string
}

func (m *MockCapHostForRegistry) ExecuteCap(
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

func TestRegisterAndFindCapHost(t *testing.T) {
	registry := NewCapHostRegistry()
	
	host := &MockCapHostForRegistry{name: "test-host"}
	
	capUrn, err := NewCapUrnFromString("cap:action=test;type=basic")
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
	
	err = registry.RegisterCapHost("test-host", host, []*Cap{cap})
	if err != nil {
		t.Fatalf("Failed to register cap host: %v", err)
	}
	
	// Test exact match
	hosts, err := registry.FindCapHosts("cap:action=test;type=basic")
	if err != nil {
		t.Fatalf("Failed to find cap hosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(hosts))
	}
	
	// Test subset match (request has more specific requirements)
	hosts, err = registry.FindCapHosts("cap:action=test;type=basic;model=gpt-4")
	if err != nil {
		t.Fatalf("Failed to find cap hosts for subset match: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host for subset match, got %d", len(hosts))
	}
	
	// Test no match
	_, err = registry.FindCapHosts("cap:action=different")
	if err == nil {
		t.Error("Expected error for non-matching capability, got nil")
	}
}

func TestBestCapHostSelection(t *testing.T) {
	registry := NewCapHostRegistry()
	
	// Register general host
	generalHost := &MockCapHostForRegistry{name: "general"}
	generalCapUrn, _ := NewCapUrnFromString("cap:action=generate")
	generalCap := &Cap{
		Urn:            generalCapUrn,
		CapDescription: stringPtr("General generation"),
		Metadata:       make(map[string]string),
		Command:        "generate",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}
	
	// Register specific host
	specificHost := &MockCapHostForRegistry{name: "specific"}
	specificCapUrn, _ := NewCapUrnFromString("cap:action=generate;type=text;model=gpt-4")
	specificCap := &Cap{
		Urn:            specificCapUrn,
		CapDescription: stringPtr("Specific text generation"),
		Metadata:       make(map[string]string),
		Command:        "generate",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}
	
	registry.RegisterCapHost("general", generalHost, []*Cap{generalCap})
	registry.RegisterCapHost("specific", specificHost, []*Cap{specificCap})
	
	// Request should match the more specific host
	bestHost, err := registry.FindBestCapHost("cap:action=generate;type=text;model=gpt-4;temperature=0.7")
	if err != nil {
		t.Fatalf("Failed to find best cap host: %v", err)
	}
	
	// Should get the specific host (though we can't directly compare interfaces)
	if bestHost == nil {
		t.Error("Expected a host, got nil")
	}
	
	// Both hosts should match
	allHosts, err := registry.FindCapHosts("cap:action=generate;type=text;model=gpt-4;temperature=0.7")
	if err != nil {
		t.Fatalf("Failed to find all matching hosts: %v", err)
	}
	if len(allHosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(allHosts))
	}
}

func TestInvalidUrnHandling(t *testing.T) {
	registry := NewCapHostRegistry()
	
	_, err := registry.FindCapHosts("invalid-urn")
	if err == nil {
		t.Error("Expected error for invalid URN, got nil")
	}
	
	capHostErr, ok := err.(*CapHostRegistryError)
	if !ok {
		t.Errorf("Expected CapHostRegistryError, got %T", err)
	} else if capHostErr.Type != "InvalidUrn" {
		t.Errorf("Expected InvalidUrn error type, got %s", capHostErr.Type)
	}
}

func TestCanHandle(t *testing.T) {
	registry := NewCapHostRegistry()
	
	// Empty registry
	if registry.CanHandle("cap:action=test") {
		t.Error("Empty registry should not handle any capability")
	}
	
	// After registration
	host := &MockCapHostForRegistry{name: "test"}
	capUrn, _ := NewCapUrnFromString("cap:action=test")
	cap := &Cap{
		Urn:            capUrn,
		CapDescription: stringPtr("Test"),
		Metadata:       make(map[string]string),
		Command:        "test",
		Arguments:      &CapArguments{Required: []CapArgument{}, Optional: []CapArgument{}},
		Output:         nil,
	}
	
	registry.RegisterCapHost("test", host, []*Cap{cap})
	
	if !registry.CanHandle("cap:action=test") {
		t.Error("Registry should handle registered capability")
	}
	if !registry.CanHandle("cap:action=test;extra=param") {
		t.Error("Registry should handle capability with extra parameters")
	}
	if registry.CanHandle("cap:action=different") {
		t.Error("Registry should not handle unregistered capability")
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}