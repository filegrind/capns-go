package cap

import (
	"context"
	"fmt"
	"testing"

	"github.com/filegrind/capns-go/media"
	"github.com/filegrind/capns-go/standard"
	"github.com/filegrind/capns-go/urn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCapSet implements CapSet for testing
type MockCapSet struct {
	expectedCapUrn string
	returnResult   *HostResult
	returnError    error
}

func (m *MockCapSet) ExecuteCap(
	ctx context.Context,
	capUrn string,
	arguments []CapArgumentValue,
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
	capUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=test;out="media:string"`)
	require.NoError(t, err)

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	mockHost := &MockCapSet{}

	caller := NewCapCaller(`cap:in="media:void";op=test;out="media:string"`, mockHost, capDef)

	assert.NotNil(t, caller)
	assert.Equal(t, `cap:in="media:void";op=test;out="media:string"`, caller.cap)
	assert.Equal(t, capDef, caller.capDefinition)
	assert.Equal(t, mockHost, caller.capSet)
}

func TestCapCallerResolveOutputSpec(t *testing.T) {
	registry := testRegistry(t)
	mockHost := &MockCapSet{}

	// Common mediaSpecs for resolution
	mediaSpecs := []MediaSpecDef{
		{Urn: "media:bytes", MediaType: "application/octet-stream"},
		{Urn: "media:textable;form=scalar", MediaType: "text/plain", ProfileURI: ProfileStr},
		{Urn: MediaObject, MediaType: "application/json", ProfileURI: ProfileObj},
	}

	// Test binary cap using the 'out' tag with media URN - use proper binary tag
	binaryCapUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:bytes"`)
	require.NoError(t, err)

	capDef := NewCap(binaryCapUrn, "Test Capability", "test-command")
	capDef.SetMediaSpecs(mediaSpecs)
	caller := NewCapCaller(`cap:in="media:void";op=generate;out="media:bytes"`, mockHost, capDef)

	resolved, err := caller.resolveOutputSpec(registry)
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test non-binary cap (text output) - use proper textable tag
	textCapUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:textable;form=scalar"`)
	require.NoError(t, err)

	capDef2 := NewCap(textCapUrn, "Test Capability", "test-command")
	capDef2.SetMediaSpecs(mediaSpecs)
	caller2 := NewCapCaller(`cap:in="media:void";op=generate;out="media:textable;form=scalar"`, mockHost, capDef2)

	resolved2, err := caller2.resolveOutputSpec(registry)
	require.NoError(t, err)
	assert.False(t, resolved2.IsBinary())
	assert.True(t, resolved2.IsText())

	// Test map cap with object output - use proper form=map tag
	mapCapUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=generate;out="` + MediaObject + `"`)
	require.NoError(t, err)

	capDef3 := NewCap(mapCapUrn, "Test Capability", "test-command")
	capDef3.SetMediaSpecs(mediaSpecs)
	caller3 := NewCapCaller(`cap:in="media:void";op=generate;out="`+MediaObject+`"`, mockHost, capDef3)

	resolved3, err := caller3.resolveOutputSpec(registry)
	require.NoError(t, err)
	assert.True(t, resolved3.IsMap())

	// Test cap with unresolvable media URN - MUST FAIL (no mediaSpecs entry)
	badSpecCapUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:unknown"`)
	require.NoError(t, err)

	capDef5 := NewCap(badSpecCapUrn, "Test Capability", "test-command")
	capDef5.SetMediaSpecs(mediaSpecs) // mediaSpecs provided but doesn't contain "media:unknown"
	caller5 := NewCapCaller(`cap:in="media:void";op=generate;out="media:unknown"`, mockHost, capDef5)

	_, err = caller5.resolveOutputSpec(registry)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve output media URN")
}

func TestCapCallerCall(t *testing.T) {
	registry := testRegistry(t)
	// Setup test data - use MediaString constant for proper resolution
	capUrnStr := `cap:in="` + MediaVoid + `";op=test;out="` + MediaString + `"`
	capUrn, err := urn.NewCapUrnFromString(capUrnStr)
	require.NoError(t, err)

	// mediaSpecs for resolution
	mediaSpecs := []MediaSpecDef{
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
		{Urn: MediaVoid, MediaType: "application/x-void", ProfileURI: ProfileVoid},
	}

	capDef := NewCap(capUrn, "Test Capability", "test-command")
	capDef.SetOutput(NewCapOutput(MediaString, "Test output"))
	capDef.SetMediaSpecs(mediaSpecs)

	mockHost := &MockCapSet{
		expectedCapUrn: capUrnStr,
		returnResult: &HostResult{
			TextOutput: "test result",
		},
	}

	caller := NewCapCaller(capUrnStr, mockHost, capDef)

	// Test call with no arguments
	ctx := context.Background()
	result, err := caller.Call(ctx, []CapArgumentValue{}, registry)

	require.NoError(t, err)
	require.NotNil(t, result)

	resultStr, err := result.AsString()
	require.NoError(t, err)
	assert.Equal(t, "test result", resultStr)
}

func TestCapCallerWithArguments(t *testing.T) {
	registry := testRegistry(t)
	// Setup test data with arguments - use proper map tag for object
	capUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=process;out="media:form=map;textable"`)
	require.NoError(t, err)

	// mediaSpecs for resolution - MediaObject = "media:form=map;textable"
	mediaSpecs := []MediaSpecDef{
		{Urn: MediaObject, MediaType: "application/json", ProfileURI: ProfileObj},
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
	}

	capDef := NewCap(capUrn, "Process Capability", "process-command")
	capDef.SetMediaSpecs(mediaSpecs)
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

	// Test call with unified argument
	ctx := context.Background()
	result, err := caller.Call(ctx, []CapArgumentValue{
		NewCapArgumentValueFromStr(MediaString, "test.txt"),
	}, registry)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsJSON())
}

func TestCapCallerBinaryResponse(t *testing.T) {
	registry := testRegistry(t)
	// Setup binary cap - use raw type with binary tag
	capUrn, err := urn.NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:bytes"`)
	require.NoError(t, err)

	// mediaSpecs for resolution - MediaBinary = "media:bytes"
	mediaSpecs := []MediaSpecDef{
		{Urn: MediaBinary, MediaType: "application/octet-stream"},
	}

	capDef := NewCap(capUrn, "Generate Capability", "generate-command")
	capDef.SetMediaSpecs(mediaSpecs)
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
	result, err := caller.Call(ctx, []CapArgumentValue{}, registry)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsBinary())
	assert.Equal(t, pngHeader, result.AsBytes())
}

// TEST156: Test creating StdinSource Data variant with byte vector
func TestStdinSourceDataCreation(t *testing.T) {
	data := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f} // "Hello"
	source := NewStdinSourceFromData(data)

	assert.Equal(t, StdinSourceKindData, source.Kind)
	assert.Equal(t, data, source.Data)
	assert.True(t, source.IsData())
	assert.False(t, source.IsFileReference())
}

// TEST157: Test creating StdinSource FileReference variant with all required fields
func TestStdinSourceFileReferenceCreation(t *testing.T) {
	trackedFileID := "tracked-file-123"
	originalPath := "/path/to/original.pdf"
	securityBookmark := []byte{0x62, 0x6f, 0x6f, 0x6b} // "book"
	mediaUrn := "media:pdf;bytes"

	source := NewStdinSourceFromFileReference(
		trackedFileID,
		originalPath,
		securityBookmark,
		mediaUrn,
	)

	assert.Equal(t, StdinSourceKindFileReference, source.Kind)
	assert.Equal(t, trackedFileID, source.TrackedFileID)
	assert.Equal(t, originalPath, source.OriginalPath)
	assert.Equal(t, securityBookmark, source.SecurityBookmark)
	assert.Equal(t, mediaUrn, source.MediaUrn)
	assert.False(t, source.IsData())
	assert.True(t, source.IsFileReference())
}

// TEST158: Test StdinSource Data with empty vector stores and retrieves correctly
func TestStdinSourceEmptyData(t *testing.T) {
	source := NewStdinSourceFromData([]byte{})

	assert.Equal(t, StdinSourceKindData, source.Kind)
	assert.Empty(t, source.Data)
	assert.True(t, source.IsData())
}

// TEST159: Test StdinSource Data with binary content like PNG header bytes
func TestStdinSourceBinaryContent(t *testing.T) {
	// PNG header bytes
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	source := NewStdinSourceFromData(pngHeader)

	assert.Equal(t, StdinSourceKindData, source.Kind)
	assert.Equal(t, 8, len(source.Data))
	assert.Equal(t, byte(0x89), source.Data[0])
	assert.Equal(t, byte(0x50), source.Data[1]) // 'P'
	assert.Equal(t, pngHeader, source.Data)
}

// TEST160: Test StdinSource Data clone creates independent copy with same data
func TestStdinSourceDataClone(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	source := NewStdinSourceFromData(data)

	// Create a deep copy by copying the data slice
	dataCopy := make([]byte, len(source.Data))
	copy(dataCopy, source.Data)
	cloned := NewStdinSourceFromData(dataCopy)

	assert.Equal(t, source.Kind, cloned.Kind)
	assert.Equal(t, source.Data, cloned.Data)

	// Verify they're independent - modifying clone doesn't affect original
	cloned.Data[0] = 99
	assert.NotEqual(t, source.Data[0], cloned.Data[0])
}

// TEST161: Test StdinSource FileReference clone creates independent copy with same fields
func TestStdinSourceFileReferenceClone(t *testing.T) {
	source := NewStdinSourceFromFileReference("test-id", "/test/path.pdf", []byte{1, 2, 3}, "media:pdf")

	// Create a manual copy
	cloned := NewStdinSourceFromFileReference(
		source.TrackedFileID,
		source.OriginalPath,
		append([]byte{}, source.SecurityBookmark...),
		source.MediaUrn,
	)

	assert.Equal(t, source.Kind, cloned.Kind)
	assert.Equal(t, source.TrackedFileID, cloned.TrackedFileID)
	assert.Equal(t, source.OriginalPath, cloned.OriginalPath)
	assert.Equal(t, source.SecurityBookmark, cloned.SecurityBookmark)
	assert.Equal(t, source.MediaUrn, cloned.MediaUrn)
}

// TEST162: Test StdinSource Debug format displays variant type and relevant fields
func TestStdinSourceDebug(t *testing.T) {
	// Test Data variant
	dataSource := NewStdinSourceFromData([]byte{1, 2, 3})
	debugStr := fmt.Sprintf("%+v", dataSource)
	assert.Contains(t, debugStr, "Kind")
	assert.Contains(t, debugStr, "Data")

	// Test FileReference variant
	fileSource := NewStdinSourceFromFileReference("test-id", "/test/path.pdf", []byte{}, "media:pdf")
	debugStr = fmt.Sprintf("%+v", fileSource)
	assert.Contains(t, debugStr, "TrackedFileID")
	assert.Contains(t, debugStr, "OriginalPath")
	assert.Contains(t, debugStr, "MediaUrn")
}

// TestStdinSourceNilHandling tests that nil StdinSource is handled correctly
func TestStdinSourceNilHandling(t *testing.T) {
	var nilSource *StdinSource = nil

	// IsData and IsFileReference should return false for nil
	assert.False(t, nilSource.IsData())
	assert.False(t, nilSource.IsFileReference())
}

// TEST274: Test CapArgumentValue::new stores media_urn and raw byte value
func TestCapArgumentValueNew(t *testing.T) {
	arg := NewCapArgumentValue("media:model-spec;textable;form=scalar", []byte("gpt-4"))
	assert.Equal(t, "media:model-spec;textable;form=scalar", arg.MediaUrn)
	assert.Equal(t, []byte("gpt-4"), arg.Value)
}

// TEST275: Test CapArgumentValue::from_str converts string to UTF-8 bytes
func TestCapArgumentValueFromStr(t *testing.T) {
	arg := NewCapArgumentValueFromStr("media:string;textable", "hello world")
	assert.Equal(t, "media:string;textable", arg.MediaUrn)
	assert.Equal(t, []byte("hello world"), arg.Value)
}

// TEST276: Test CapArgumentValue::value_as_str succeeds for UTF-8 data
func TestCapArgumentValueAsStrValid(t *testing.T) {
	arg := NewCapArgumentValueFromStr("media:string", "test")
	val, err := arg.ValueAsStr()
	require.NoError(t, err)
	assert.Equal(t, "test", val)
}

// TEST277: Test CapArgumentValue::value_as_str fails for non-UTF-8 binary data
func TestCapArgumentValueAsStrInvalidUtf8(t *testing.T) {
	arg := NewCapArgumentValue("media:pdf;bytes", []byte{0xFF, 0xFE, 0x80})
	_, err := arg.ValueAsStr()
	require.Error(t, err, "non-UTF-8 data must fail")
}

// TEST278: Test CapArgumentValue::new with empty value stores empty vec
func TestCapArgumentValueEmpty(t *testing.T) {
	arg := NewCapArgumentValue("media:void", []byte{})
	assert.Empty(t, arg.Value)
	val, err := arg.ValueAsStr()
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

// TEST279: Test CapArgumentValue Clone produces independent copy with same data
func TestCapArgumentValueClone(t *testing.T) {
	arg := NewCapArgumentValue("media:test", []byte("data"))

	// In Go, we create a deep copy by copying the value slice
	valueCopy := make([]byte, len(arg.Value))
	copy(valueCopy, arg.Value)
	cloned := NewCapArgumentValue(arg.MediaUrn, valueCopy)

	assert.Equal(t, arg.MediaUrn, cloned.MediaUrn)
	assert.Equal(t, arg.Value, cloned.Value)

	// Verify they're independent - modifying clone doesn't affect original
	cloned.Value[0] = 'X'
	assert.NotEqual(t, arg.Value[0], cloned.Value[0])
}

// TEST280: Test CapArgumentValue Debug format includes media_urn and value
func TestCapArgumentValueDebug(t *testing.T) {
	arg := NewCapArgumentValueFromStr("media:test", "val")

	// In Go, we use String() method for debug representation
	str := arg.String()
	assert.Contains(t, str, "media:test", "string representation must include media_urn")
}

// TEST281: Test CapArgumentValue::new accepts Into<String> for media_urn (String and &str)
func TestCapArgumentValueStringTypes(t *testing.T) {
	s := "media:owned"
	arg1 := NewCapArgumentValue(s, []byte{})
	assert.Equal(t, "media:owned", arg1.MediaUrn)

	arg2 := NewCapArgumentValue("media:borrowed", []byte{})
	assert.Equal(t, "media:borrowed", arg2.MediaUrn)
}

// TEST282: Test CapArgumentValue from_str with Unicode string preserves all characters
func TestCapArgumentValueUnicode(t *testing.T) {
	arg := NewCapArgumentValueFromStr("media:string", "hello 世界 🌍")
	val, err := arg.ValueAsStr()
	require.NoError(t, err)
	assert.Equal(t, "hello 世界 🌍", val)
}

// TEST283: Test CapArgumentValue with large binary payload preserves all bytes
func TestCapArgumentValueLargeBinary(t *testing.T) {
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}
	arg := NewCapArgumentValue("media:pdf;bytes", data)
	assert.Equal(t, 10000, len(arg.Value))
	assert.Equal(t, data, arg.Value)
}
