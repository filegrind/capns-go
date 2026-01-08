package capns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCapHost implements CapHost for testing
type MockCapHost struct {
	expectedCapUrn         string
	expectedPositionalArgs []string
	expectedNamedArgs      map[string]string
	expectedStdinData      []byte
	returnResult           *HostResult
	returnError            error
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
	capUrn, err := NewCapUrnFromString("cap:op=test")
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapHost{}

	caller := NewCapCaller("cap:op=test", mockHost, capDef)

	assert.NotNil(t, caller)
	assert.Equal(t, "cap:op=test", caller.cap)
	assert.Equal(t, capDef, caller.capDefinition)
	assert.Equal(t, mockHost, caller.capHost)
}

func TestCapCallerConvertToString(t *testing.T) {
	capUrn, err := NewCapUrnFromString("cap:op=test")
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapHost{}
	caller := NewCapCaller("cap:op=test", mockHost, capDef)

	// Test different type conversions
	assert.Equal(t, "hello", caller.convertToString("hello"))
	assert.Equal(t, "42", caller.convertToString(42))
	assert.Equal(t, "3.14", caller.convertToString(3.14))
	assert.Equal(t, "true", caller.convertToString(true))
	assert.Equal(t, "", caller.convertToString(nil))
}

func TestCapCallerResolveOutputSpec(t *testing.T) {
	mockHost := &MockCapHost{}

	// Test binary cap using the 'out' tag with spec ID
	binaryCapUrn, err := NewCapUrnFromString("cap:op=generate;out=std:binary.v1")
	require.NoError(t, err)

	capDef := NewCap(binaryCapUrn, "Test Capability", "test-command")
	caller := NewCapCaller("cap:op=generate;out=std:binary.v1", mockHost, capDef)

	resolved, err := caller.resolveOutputSpec()
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test non-binary cap (text output)
	textCapUrn, err := NewCapUrnFromString("cap:op=generate;out=std:str.v1")
	require.NoError(t, err)

	capDef2 := NewCap(textCapUrn, "Test Capability", "test-command")
	caller2 := NewCapCaller("cap:op=generate;out=std:str.v1", mockHost, capDef2)

	resolved2, err := caller2.resolveOutputSpec()
	require.NoError(t, err)
	assert.False(t, resolved2.IsBinary())
	assert.True(t, resolved2.IsText())

	// Test JSON cap with object output
	jsonCapUrn, err := NewCapUrnFromString("cap:op=generate;out=std:obj.v1")
	require.NoError(t, err)

	capDef3 := NewCap(jsonCapUrn, "Test Capability", "test-command")
	caller3 := NewCapCaller("cap:op=generate;out=std:obj.v1", mockHost, capDef3)

	resolved3, err := caller3.resolveOutputSpec()
	require.NoError(t, err)
	assert.True(t, resolved3.IsJSON())

	// Test cap without output tag - MUST FAIL
	noOutCapUrn, err := NewCapUrnFromString("cap:op=generate")
	require.NoError(t, err)

	capDef4 := NewCap(noOutCapUrn, "Test Capability", "test-command")
	caller4 := NewCapCaller("cap:op=generate", mockHost, capDef4)

	_, err = caller4.resolveOutputSpec()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'out' tag")

	// Test cap with unresolvable spec ID - MUST FAIL
	badSpecCapUrn, err := NewCapUrnFromString("cap:op=generate;out=unknown:spec.v1")
	require.NoError(t, err)

	capDef5 := NewCap(badSpecCapUrn, "Test Capability", "test-command")
	caller5 := NewCapCaller("cap:op=generate;out=unknown:spec.v1", mockHost, capDef5)

	_, err = caller5.resolveOutputSpec()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve output spec ID")
}

func TestCapCallerCall(t *testing.T) {
	// Setup test data
	capUrn, err := NewCapUrnFromString("cap:op=test;out=std:str.v1")
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	capDef.SetOutput(NewCapOutput(SpecIDStr, "Test output"))

	mockHost := &MockCapHost{
		expectedCapUrn: "cap:op=test;out=std:str.v1",
		returnResult: &HostResult{
			TextOutput: "test result",
		},
	}

	caller := NewCapCaller("cap:op=test;out=std:str.v1", mockHost, capDef)

	// Test call with no arguments
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	resultStr, err := result.AsString()
	require.NoError(t, err)
	assert.Equal(t, "test result", resultStr)
}

func TestCapCallerWithArguments(t *testing.T) {
	// Setup test data with arguments
	capUrn, err := NewCapUrnFromString("cap:op=process;out=std:obj.v1")
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Process Capability", "process-command")
	capDef.AddRequiredArgument(NewCapArgument("input", SpecIDStr, "Input file", "--input"))
	capDef.SetOutput(NewCapOutput(SpecIDObj, "Process output"))

	mockHost := &MockCapHost{
		returnResult: &HostResult{
			TextOutput: `{"status": "ok"}`,
		},
	}

	caller := NewCapCaller("cap:op=process;out=std:obj.v1", mockHost, capDef)

	// Test call with positional argument
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{"test.txt"}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsJSON())
}

func TestCapCallerBinaryResponse(t *testing.T) {
	// Setup binary cap
	capUrn, err := NewCapUrnFromString("cap:op=generate;out=std:binary.v1")
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Generate Capability", "generate-command")
	capDef.SetOutput(NewCapOutput(SpecIDBinary, "Binary output"))

	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapHost{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller("cap:op=generate;out=std:binary.v1", mockHost, capDef)

	// Test call
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsBinary())
	assert.Equal(t, pngHeader, result.AsBytes())
}
