package capns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCapHost implements CapHost for testing
type MockCapHost struct {
	expectedCapUrn        string
	expectedPositionalArgs []string
	expectedNamedArgs     map[string]string
	expectedStdinData     []byte
	returnResult          *HostResult
	returnError           error
}

func (m *MockCapHost) ExecuteCap(
	ctx context.Context,
	capUrn string,
	positionalArgs []string,
	namedArgs map[string]string,
	stdinData []byte,
) (*HostResult, error) {
	if m.expectedCapUrn != "" {
		if capUrn != m.expectedCapUrn {
			return nil, assert.AnError
		}
	}
	return m.returnResult, m.returnError
}

func TestCapCallerCreation(t *testing.T) {
	// Setup test data
	capUrn, err := NewCapUrnFromString("cap:action=test")
	require.NoError(t, err)
	
	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapHost{}
	
	caller := NewCapCaller("cap:action=test", mockHost, capDef)
	
	assert.NotNil(t, caller)
	assert.Equal(t, "cap:action=test", caller.cap)
	assert.Equal(t, capDef, caller.capDefinition)
	assert.Equal(t, mockHost, caller.capHost)
}

func TestCapCallerConvertToString(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:action=test")
	require.NoError(t, err)
	
	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapHost{}
	caller := NewCapCaller("cap:action=test", mockHost, capDef)
	
	// Test different type conversions
	assert.Equal(t, "hello", caller.convertToString("hello"))
	assert.Equal(t, "42", caller.convertToString(42))
	assert.Equal(t, "3.14", caller.convertToString(3.14))
	assert.Equal(t, "true", caller.convertToString(true))
	assert.Equal(t, "", caller.convertToString(nil))
}

func TestCapCallerIsBinaryCap(t *testing.T) {
	// Test binary cap
	binaryCapUrn, err := NewCapUrnFromString("cap:action=generate;output=binary")
	require.NoError(t, err)
	
	capDef := NewCap(binaryCapUrn, "Test Capability", "test-command")
	mockHost := &MockCapHost{}
	caller := NewCapCaller("cap:action=generate;output=binary", mockHost, capDef)
	
	assert.True(t, caller.isBinaryCap())
	
	// Test non-binary cap
	textCapUrn, err := NewCapUrnFromString("cap:action=generate;output=text")
	require.NoError(t, err)
	
	capDef2 := NewCap(textCapUrn, "Test Capability", "test-command")
	caller2 := NewCapCaller("cap:action=generate;output=text", mockHost, capDef2)
	
	assert.False(t, caller2.isBinaryCap())
}

func TestCapCallerIsJsonCap(t *testing.T) {
	// Test JSON cap (default when no output specified)
	jsonCapUrn, err := NewCapUrnFromString("cap:action=generate")
	require.NoError(t, err)
	
	capDef := NewCap(jsonCapUrn, "Test Capability", "test-command")
	mockHost := &MockCapHost{}
	caller := NewCapCaller("cap:action=generate", mockHost, capDef)
	
	assert.True(t, caller.isJsonCap())
	
	// Test binary cap (not JSON)
	binaryCapUrn, err := NewCapUrnFromString("cap:action=generate;output=binary")
	require.NoError(t, err)
	
	capDef2 := NewCap(binaryCapUrn, "Test Capability", "test-command")
	caller2 := NewCapCaller("cap:action=generate;output=binary", mockHost, capDef2)
	
	assert.False(t, caller2.isJsonCap())
}

func TestCapCallerCall(t *testing.T) {
	// Setup test data
	capUrn, err := NewCapUrnFromString("cap:action=test")
	require.NoError(t, err)
	
	capDef := NewCap(capUrn, "Test Capability", "test-command")
	capDef.SetOutput(NewCapOutput(OutputTypeString, "Test output"))
	
	mockHost := &MockCapHost{
		expectedCapUrn: "cap:action=test",
		returnResult: &HostResult{
			TextOutput: "\"test result\"", // Valid JSON string
		},
	}
	
	caller := NewCapCaller("cap:action=test", mockHost, capDef)
	
	// Test call with no arguments
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	
	resultStr, err := result.AsString()
	require.NoError(t, err)
	assert.Equal(t, "\"test result\"", resultStr)
}