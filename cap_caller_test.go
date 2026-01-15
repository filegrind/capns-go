package capns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCapSet implements CapSet for testing
type MockCapSet struct {
	expectedCapUrn         string
	expectedPositionalArgs []string
	expectedNamedArgs      map[string]string
	expectedStdinData      []byte
	returnResult           *HostResult
	returnError            error
}

func (m *MockCapSet) ExecuteCap(
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
	// Setup test data - now with required in/out
	capUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapSet{}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`, mockHost, capDef)

	assert.NotNil(t, caller)
	assert.Equal(t, `cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`, caller.cap)
	assert.Equal(t, capDef, caller.capDefinition)
	assert.Equal(t, mockHost, caller.capSet)
}

func TestCapCallerConvertToString(t *testing.T) {
	capUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapSet{}
	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`, mockHost, capDef)

	// Test different type conversions
	assert.Equal(t, "hello", caller.convertToString("hello"))
	assert.Equal(t, "42", caller.convertToString(42))
	assert.Equal(t, "3.14", caller.convertToString(3.14))
	assert.Equal(t, "true", caller.convertToString(true))
	assert.Equal(t, "", caller.convertToString(nil))
}

func TestCapCallerResolveOutputSpec(t *testing.T) {
	mockHost := &MockCapSet{}

	// Test binary cap using the 'out' tag with media URN
	binaryCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=generate;out="media:type=binary;v=1"`)
	require.NoError(t, err)

	capDef := NewCap(binaryCapUrn, "Test Capability", "test-command")
	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=generate;out="media:type=binary;v=1"`, mockHost, capDef)

	resolved, err := caller.resolveOutputSpec()
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test non-binary cap (text output)
	textCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=generate;out="media:type=string;v=1"`)
	require.NoError(t, err)

	capDef2 := NewCap(textCapUrn, "Test Capability", "test-command")
	caller2 := NewCapCaller(`cap:in="media:type=void;v=1";op=generate;out="media:type=string;v=1"`, mockHost, capDef2)

	resolved2, err := caller2.resolveOutputSpec()
	require.NoError(t, err)
	assert.False(t, resolved2.IsBinary())
	assert.True(t, resolved2.IsText())

	// Test JSON cap with object output
	jsonCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=generate;out="media:type=object;v=1"`)
	require.NoError(t, err)

	capDef3 := NewCap(jsonCapUrn, "Test Capability", "test-command")
	caller3 := NewCapCaller(`cap:in="media:type=void;v=1";op=generate;out="media:type=object;v=1"`, mockHost, capDef3)

	resolved3, err := caller3.resolveOutputSpec()
	require.NoError(t, err)
	assert.True(t, resolved3.IsJSON())

	// Test cap with unresolvable media URN - MUST FAIL
	badSpecCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=generate;out="media:type=unknown;v=1"`)
	require.NoError(t, err)

	capDef5 := NewCap(badSpecCapUrn, "Test Capability", "test-command")
	caller5 := NewCapCaller(`cap:in="media:type=void;v=1";op=generate;out="media:type=unknown;v=1"`, mockHost, capDef5)

	_, err = caller5.resolveOutputSpec()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve output media URN")
}

func TestCapCallerCall(t *testing.T) {
	// Setup test data
	capUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	capDef.SetOutput(NewCapOutput(MediaString, "Test output"))

	mockHost := &MockCapSet{
		expectedCapUrn: `cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`,
		returnResult: &HostResult{
			TextOutput: "test result",
		},
	}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1"`, mockHost, capDef)

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
	capUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=process;out="media:type=object;v=1"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Process Capability", "process-command")
	capDef.AddRequiredArgument(NewCapArgument("input", MediaString, "Input file", "--input"))
	capDef.SetOutput(NewCapOutput(MediaObject, "Process output"))

	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"status": "ok"}`,
		},
	}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=process;out="media:type=object;v=1"`, mockHost, capDef)

	// Test call with positional argument
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{"test.txt"}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsJSON())
}

func TestCapCallerBinaryResponse(t *testing.T) {
	// Setup binary cap
	capUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=generate;out="media:type=binary;v=1"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Generate Capability", "generate-command")
	capDef.SetOutput(NewCapOutput(MediaBinary, "Binary output"))

	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller(`cap:in="media:type=void;v=1";op=generate;out="media:type=binary;v=1"`, mockHost, capDef)

	// Test call
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsBinary())
	assert.Equal(t, pngHeader, result.AsBytes())
}
