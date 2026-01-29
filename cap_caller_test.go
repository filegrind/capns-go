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
	expectedStdinSource    *StdinSource
	returnResult           *HostResult
	returnError            error
}

func (m *MockCapSet) ExecuteCap(
	ctx context.Context,
	capUrn string,
	positionalArgs []string,
	namedArgs map[string]string,
	stdinSource *StdinSource,
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
	capUrn, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:string"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapSet{}

	caller := NewCapCaller(`cap:in="media:void";op=test;out="media:string"`, mockHost, capDef)

	assert.NotNil(t, caller)
	assert.Equal(t, `cap:in="media:void";op=test;out="media:string"`, caller.cap)
	assert.Equal(t, capDef, caller.capDefinition)
	assert.Equal(t, mockHost, caller.capSet)
}

func TestCapCallerConvertToString(t *testing.T) {
	capUrn, err := NewCapUrnFromString(`cap:in="media:void";op=test;out="media:string"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapSet{}
	caller := NewCapCaller(`cap:in="media:void";op=test;out="media:string"`, mockHost, capDef)

	// Test different type conversions
	assert.Equal(t, "hello", caller.convertToString("hello"))
	assert.Equal(t, "42", caller.convertToString(42))
	assert.Equal(t, "3.14", caller.convertToString(3.14))
	assert.Equal(t, "true", caller.convertToString(true))
	assert.Equal(t, "", caller.convertToString(nil))
}

func TestCapCallerResolveOutputSpec(t *testing.T) {
	mockHost := &MockCapSet{}

	// Test binary cap using the 'out' tag with media URN - use proper binary tag
	binaryCapUrn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:bytes"`)
	require.NoError(t, err)

	capDef := NewCap(binaryCapUrn, "Test Capability", "test-command")
	caller := NewCapCaller(`cap:in="media:void";op=generate;out="media:bytes"`, mockHost, capDef)

	resolved, err := caller.resolveOutputSpec()
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test non-binary cap (text output) - use proper textable tag
	textCapUrn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:textable;form=scalar"`)
	require.NoError(t, err)

	capDef2 := NewCap(textCapUrn, "Test Capability", "test-command")
	caller2 := NewCapCaller(`cap:in="media:void";op=generate;out="media:textable;form=scalar"`, mockHost, capDef2)

	resolved2, err := caller2.resolveOutputSpec()
	require.NoError(t, err)
	assert.False(t, resolved2.IsBinary())
	assert.True(t, resolved2.IsText())

	// Test map cap with object output - use proper form=map tag
	mapCapUrn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="` + MediaObject + `"`)
	require.NoError(t, err)

	capDef3 := NewCap(mapCapUrn, "Test Capability", "test-command")
	caller3 := NewCapCaller(`cap:in="media:void";op=generate;out="`+MediaObject+`"`, mockHost, capDef3)

	resolved3, err := caller3.resolveOutputSpec()
	require.NoError(t, err)
	assert.True(t, resolved3.IsMap())

	// Test cap with unresolvable media URN - MUST FAIL
	badSpecCapUrn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:unknown"`)
	require.NoError(t, err)

	capDef5 := NewCap(badSpecCapUrn, "Test Capability", "test-command")
	caller5 := NewCapCaller(`cap:in="media:void";op=generate;out="media:unknown"`, mockHost, capDef5)

	_, err = caller5.resolveOutputSpec()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve output media URN")
}

func TestCapCallerCall(t *testing.T) {
	// Setup test data - use MediaString constant for proper resolution
	capUrnStr := `cap:in="` + MediaVoid + `";op=test;out="` + MediaString + `"`
	capUrn, err := NewCapUrnFromString(capUrnStr)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	capDef.SetOutput(NewCapOutput(MediaString, "Test output"))

	mockHost := &MockCapSet{
		expectedCapUrn: capUrnStr,
		returnResult: &HostResult{
			TextOutput: "test result",
		},
	}

	caller := NewCapCaller(capUrnStr, mockHost, capDef)

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
	// Setup test data with arguments - use proper map tag for object
	capUrn, err := NewCapUrnFromString(`cap:in="media:void";op=process;out="media:form=map;textable"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Process Capability", "process-command")
	cliFlag := "--input"
	pos := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Input file",
	})
	capDef.SetOutput(NewCapOutput(MediaObject, "Process output"))

	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"status": "ok"}`,
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=process;out="media:form=map;textable"`, mockHost, capDef)

	// Test call with positional argument
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{"test.txt"}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsJSON())
}

func TestCapCallerBinaryResponse(t *testing.T) {
	// Setup binary cap - use raw type with binary tag
	capUrn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:bytes"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Generate Capability", "generate-command")
	capDef.SetOutput(NewCapOutput(MediaBinary, "Binary output"))

	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=generate;out="media:bytes"`, mockHost, capDef)

	// Test call
	ctx := context.Background()
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, nil)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsBinary())
	assert.Equal(t, pngHeader, result.AsBytes())
}

// TestStdinSourceCreation tests the creation of StdinSource types
func TestStdinSourceCreation(t *testing.T) {
	// Test Data source creation
	data := []byte("test data")
	dataSource := NewStdinSourceFromData(data)

	assert.NotNil(t, dataSource)
	assert.Equal(t, StdinSourceKindData, dataSource.Kind)
	assert.True(t, dataSource.IsData())
	assert.False(t, dataSource.IsFileReference())
	assert.Equal(t, data, dataSource.Data)

	// Test FileReference source creation
	fileSource := NewStdinSourceFromFileReference(
		"tracked-123",
		"/path/to/original.pdf",
		[]byte("security-bookmark-data"),
		"media:pdf;bytes",
	)

	assert.NotNil(t, fileSource)
	assert.Equal(t, StdinSourceKindFileReference, fileSource.Kind)
	assert.False(t, fileSource.IsData())
	assert.True(t, fileSource.IsFileReference())
	assert.Equal(t, "tracked-123", fileSource.TrackedFileID)
	assert.Equal(t, "/path/to/original.pdf", fileSource.OriginalPath)
	assert.Equal(t, []byte("security-bookmark-data"), fileSource.SecurityBookmark)
	assert.Equal(t, "media:pdf;bytes", fileSource.MediaUrn)
}

// TestStdinSourceNilHandling tests that nil StdinSource is handled correctly
func TestStdinSourceNilHandling(t *testing.T) {
	var nilSource *StdinSource = nil

	// IsData and IsFileReference should return false for nil
	assert.False(t, nilSource.IsData())
	assert.False(t, nilSource.IsFileReference())
}

// MockCapSetWithStdinVerification implements CapSet and verifies stdin source
type MockCapSetWithStdinVerification struct {
	t                 *testing.T
	expectedStdinKind StdinSourceKind
	expectedStdinData []byte
	expectedFileID    string
}

func (m *MockCapSetWithStdinVerification) ExecuteCap(
	ctx context.Context,
	capUrn string,
	positionalArgs []string,
	namedArgs map[string]string,
	stdinSource *StdinSource,
) (*HostResult, error) {
	if stdinSource == nil {
		m.t.Fatal("Expected StdinSource but got nil")
	}

	assert.Equal(m.t, m.expectedStdinKind, stdinSource.Kind)

	if stdinSource.IsData() {
		assert.Equal(m.t, m.expectedStdinData, stdinSource.Data)
	} else if stdinSource.IsFileReference() {
		assert.Equal(m.t, m.expectedFileID, stdinSource.TrackedFileID)
	}

	return &HostResult{TextOutput: "ok"}, nil
}

// TestCapCallerWithStdinSourceData tests calling a cap with StdinSource Data
func TestCapCallerWithStdinSourceData(t *testing.T) {
	capUrnStr := `cap:in="` + MediaVoid + `";op=process;out="` + MediaString + `"`
	capUrn, err := NewCapUrnFromString(capUrnStr)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Process Capability", "process-command")
	capDef.SetOutput(NewCapOutput(MediaString, "Process output"))

	stdinData := []byte("test stdin content")
	mockHost := &MockCapSetWithStdinVerification{
		t:                 t,
		expectedStdinKind: StdinSourceKindData,
		expectedStdinData: stdinData,
	}

	caller := NewCapCaller(capUrnStr, mockHost, capDef)

	ctx := context.Background()
	stdinSource := NewStdinSourceFromData(stdinData)
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, stdinSource)

	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestCapCallerWithStdinSourceFileReference tests calling a cap with StdinSource FileReference
func TestCapCallerWithStdinSourceFileReference(t *testing.T) {
	capUrnStr := `cap:in="` + MediaVoid + `";op=process;out="` + MediaString + `"`
	capUrn, err := NewCapUrnFromString(capUrnStr)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Process Capability", "process-command")
	capDef.SetOutput(NewCapOutput(MediaString, "Process output"))

	mockHost := &MockCapSetWithStdinVerification{
		t:                 t,
		expectedStdinKind: StdinSourceKindFileReference,
		expectedFileID:    "tracked-file-123",
	}

	caller := NewCapCaller(capUrnStr, mockHost, capDef)

	ctx := context.Background()
	stdinSource := NewStdinSourceFromFileReference(
		"tracked-file-123",
		"/path/to/file.pdf",
		[]byte("bookmark"),
		"media:pdf;bytes",
	)
	result, err := caller.Call(ctx, []interface{}{}, []interface{}{}, stdinSource)

	require.NoError(t, err)
	require.NotNil(t, result)
}
