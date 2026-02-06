package capns

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"

	cbor2 "github.com/fxamacker/cbor/v2"
	"github.com/filegrind/cap-sdk-go/cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper for integration tests - use proper media URNs with tags
func intTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:void";out="media:form=map;textable"`
	}
	return `cap:in="media:void";out="media:form=map;textable";` + tags
}

// TestIntegrationVersionlessCapCreation verifies caps can be created without version fields
func TestIntegrationVersionlessCapCreation(t *testing.T) {
	// Test case 1: Create cap without version parameter
	// Use type=data_processing key=value instead of flag
	urn, err := NewCapUrnFromString(intTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	cap := NewCap(urn, "Data Transformer", "transform-command")

	// Verify the cap has direction specs in canonical form
	assert.Contains(t, cap.UrnString(), `in=media:void`)
	assert.Contains(t, cap.UrnString(), `out="media:form=map;textable"`)
	assert.Equal(t, "transform-command", cap.Command)

	// Test case 2: Create cap with description but no version
	cap2 := NewCapWithDescription(urn, "Data Transformer", "transform-command", "Transforms data")
	assert.NotNil(t, cap2.CapDescription)
	assert.Equal(t, "Transforms data", *cap2.CapDescription)

	// Test case 3: Verify caps can be compared without version
	assert.True(t, cap.Equals(cap))

	// Different caps should not be equal
	urn2, _ := NewCapUrnFromString(intTestUrn("op=generate;format=pdf"))
	cap3 := NewCap(urn2, "PDF Generator", "generate-command")
	assert.False(t, cap.Equals(cap3))
}

// TestIntegrationCaseInsensitiveUrns verifies URNs are case-insensitive
func TestIntegrationCaseInsensitiveUrns(t *testing.T) {
	// Test case 1: Different case inputs should produce same URN
	urn1, err := NewCapUrnFromString(intTestUrn("OP=Transform;FORMAT=JSON;Type=Data_Processing"))
	require.NoError(t, err)

	urn2, err := NewCapUrnFromString(intTestUrn("op=transform;format=json;type=data_processing"))
	require.NoError(t, err)

	// URNs should be equal (case-insensitive keys and unquoted values)
	assert.True(t, urn1.Equals(urn2))
	assert.Equal(t, urn1.ToString(), urn2.ToString())

	// Test case 2: Case-insensitive tag operations
	op, exists := urn1.GetTag("OP")
	assert.True(t, exists)
	assert.Equal(t, "transform", op) // Should be normalized to lowercase

	op2, exists := urn1.GetTag("op")
	assert.True(t, exists)
	assert.Equal(t, "transform", op2)

	// Test case 3: HasTag - keys case-insensitive, values case-sensitive
	assert.True(t, urn1.HasTag("OP", "transform"))
	assert.True(t, urn1.HasTag("op", "transform"))
	assert.True(t, urn1.HasTag("Op", "transform"))
	assert.False(t, urn1.HasTag("op", "TRANSFORM"))

	// Test case 4: Builder preserves value case
	urn3, err := NewCapUrnBuilder().
		InSpec(MediaVoid).
		OutSpec(MediaObject).
		Tag("OP", "Transform").
		Tag("Format", "JSON").
		Build()
	require.NoError(t, err)

	assert.True(t, urn3.HasTag("op", "Transform"))
	assert.True(t, urn3.HasTag("format", "JSON"))
}

// TestIntegrationCallerAndResponseSystem verifies the caller and response system
func TestIntegrationCallerAndResponseSystem(t *testing.T) {
	registry := testRegistry(t)
	// Setup test cap definition with media URNs - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=extract;out="media:form=map;textable";target=metadata`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Metadata Extractor", "extract-metadata")
	capDef.SetOutput(NewCapOutput(MediaObject, "Extracted metadata"))

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs([]MediaSpecDef{
		{Urn: MediaObject, MediaType: "application/json", ProfileURI: ProfileObj},
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
	})

	// Add required argument using new architecture
	cliFlag := "--input"
	pos := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Input file path",
	})

	// Mock host that returns JSON
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"title": "Test Document", "pages": 10}`,
		},
	}

	// Create caller
	caller := NewCapCaller(`cap:in="media:void";op=extract;out="media:form=map;textable";target=metadata`, mockHost, capDef)

	// Test call with unified argument
	ctx := context.Background()
	response, err := caller.Call(ctx, []CapArgumentValue{
		NewCapArgumentValueFromStr(MediaString, "test.pdf"),
	}, registry)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response properties
	assert.True(t, response.IsJSON())
	assert.False(t, response.IsBinary())
	assert.False(t, response.IsEmpty())

	// Verify response can be parsed as JSON
	var metadata map[string]interface{}
	err = response.AsType(&metadata)
	require.NoError(t, err)

	assert.Equal(t, "Test Document", metadata["title"])
	assert.Equal(t, float64(10), metadata["pages"])

	// Verify response validation against cap
	err = response.ValidateAgainstCap(capDef, registry)
	assert.NoError(t, err)
}

// TestIntegrationBinaryCapHandling verifies binary cap handling
func TestIntegrationBinaryCapHandling(t *testing.T) {
	registry := testRegistry(t)
	// Setup binary cap - use raw type with binary tag
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=generate;out="media:bytes";target=thumbnail`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Thumbnail Generator", "generate-thumbnail")
	capDef.SetOutput(NewCapOutput(MediaBinary, "Generated thumbnail"))

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs([]MediaSpecDef{
		{Urn: MediaBinary, MediaType: "application/octet-stream"},
	})

	// Mock host that returns binary data
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			BinaryOutput: pngHeader,
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=generate;out="media:bytes";target=thumbnail`, mockHost, capDef)

	// Test binary response
	ctx := context.Background()
	response, err := caller.Call(ctx, []CapArgumentValue{}, registry)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response is binary
	assert.True(t, response.IsBinary())
	assert.False(t, response.IsJSON())
	assert.False(t, response.IsText())
	assert.Equal(t, pngHeader, response.AsBytes())

	// Binary to string should fail
	_, err = response.AsString()
	assert.Error(t, err)
}

// TestIntegrationTextCapHandling verifies text cap handling
func TestIntegrationTextCapHandling(t *testing.T) {
	registry := testRegistry(t)
	// Setup text cap - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=format;out="media:textable;form=scalar";target=text`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Text Formatter", "format-text")
	capDef.SetOutput(NewCapOutput(MediaString, "Formatted text"))

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs([]MediaSpecDef{
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
	})

	// Add required argument using new architecture
	cliFlag := "--input"
	pos := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag}, {Position: &pos}},
		ArgDescription: "Input text",
	})

	// Mock host that returns text
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: "Formatted output text",
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=format;out="media:textable;form=scalar";target=text`, mockHost, capDef)

	// Test text response
	ctx := context.Background()
	response, err := caller.Call(ctx, []CapArgumentValue{
		NewCapArgumentValueFromStr(MediaString, "input text"),
	}, registry)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response is text
	assert.True(t, response.IsText())
	assert.False(t, response.IsJSON())
	assert.False(t, response.IsBinary())

	text, err := response.AsString()
	require.NoError(t, err)
	assert.Equal(t, "Formatted output text", text)
}

// TestIntegrationCapWithMediaSpecs verifies caps with custom media specs
func TestIntegrationCapWithMediaSpecs(t *testing.T) {
	registry := testRegistry(t)
	// Setup cap with custom media spec - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=query;out="media:result;textable;form=map";target=data`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Query", "query-data")

	// Add custom media spec with schema
	capDef.AddMediaSpec(NewMediaSpecDefWithSchema(
		"media:result;textable;form=map",
		"application/json",
		"https://example.com/schema/result",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
				"count": map[string]interface{}{"type": "integer"},
			},
			"required": []interface{}{"items", "count"},
		},
	))

	capDef.SetOutput(NewCapOutput("media:result;textable;form=map", "Query result"))

	// Mock host
	mockHost := &MockCapSet{
		returnResult: &HostResult{
			TextOutput: `{"items": ["a", "b", "c"], "count": 3}`,
		},
	}

	caller := NewCapCaller(`cap:in="media:void";op=query;out="media:result;textable;form=map";target=data`, mockHost, capDef)

	// Test call
	ctx := context.Background()
	response, err := caller.Call(ctx, []CapArgumentValue{}, registry)
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify response
	assert.True(t, response.IsJSON())

	// Validate against cap
	err = response.ValidateAgainstCap(capDef, registry)
	assert.NoError(t, err)
}

// TestIntegrationCapValidation verifies cap schema validation
func TestIntegrationCapValidation(t *testing.T) {
	registry := testRegistry(t)
	coordinator := NewCapValidationCoordinator()

	// Create a cap with arguments - use proper tags
	urn, err := NewCapUrnFromString(`cap:in="media:void";op=process;out="media:form=map;textable";target=data`)
	require.NoError(t, err)

	capDef := NewCap(urn, "Data Processor", "process-data")

	// Add mediaSpecs for resolution
	capDef.SetMediaSpecs([]MediaSpecDef{
		{Urn: MediaObject, MediaType: "application/json", ProfileURI: ProfileObj},
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
	})

	// Add required string argument using new architecture
	cliFlag1 := "--input"
	pos1 := 0
	capDef.AddArg(CapArg{
		MediaUrn:       MediaString,
		Required:       true,
		Sources:        []ArgSource{{CliFlag: &cliFlag1}, {Position: &pos1}},
		ArgDescription: "Input path",
	})

	// Set output
	capDef.SetOutput(NewCapOutput(MediaObject, "Processing result"))

	// Register cap
	coordinator.RegisterCap(capDef)

	// Test valid inputs - string for MediaString
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{"test.txt"}, registry)
	assert.NoError(t, err)

	// Test missing required argument
	err = coordinator.ValidateInputs(capDef.UrnString(), []interface{}{}, registry)
	assert.Error(t, err)
}

// TestIntegrationMediaUrnResolution verifies media URN resolution
func TestIntegrationMediaUrnResolution(t *testing.T) {
	registry := testRegistry(t)

	// mediaSpecs for resolution - no built-in resolution, must provide specs
	mediaSpecs := []MediaSpecDef{
		{Urn: MediaString, MediaType: "text/plain", ProfileURI: ProfileStr},
		{Urn: MediaObject, MediaType: "application/json", ProfileURI: ProfileObj},
		{Urn: MediaBinary, MediaType: "application/octet-stream"},
	}

	// Test string media URN resolution
	resolved, err := ResolveMediaUrn(MediaString, mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "text/plain", resolved.MediaType)
	assert.Equal(t, ProfileStr, resolved.ProfileURI)
	assert.False(t, resolved.IsBinary())
	assert.False(t, resolved.IsJSON())
	assert.True(t, resolved.IsText())

	// Test object media URN
	resolved, err = ResolveMediaUrn(MediaObject, mediaSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "application/json", resolved.MediaType)
	assert.True(t, resolved.IsMap())
	assert.True(t, resolved.IsStructured())
	assert.False(t, resolved.IsJSON())

	// Test binary media URN
	resolved, err = ResolveMediaUrn(MediaBinary, mediaSpecs, registry)
	require.NoError(t, err)
	assert.True(t, resolved.IsBinary())

	// Test custom media URN resolution
	customSpecs := []MediaSpecDef{
		{Urn: "media:custom;textable", MediaType: "text/html", ProfileURI: "https://example.com/schema/html"},
	}

	resolved, err = ResolveMediaUrn("media:custom;textable", customSpecs, registry)
	require.NoError(t, err)
	assert.Equal(t, "text/html", resolved.MediaType)
	assert.Equal(t, "https://example.com/schema/html", resolved.ProfileURI)

	// Test unknown media URN fails
	_, err = ResolveMediaUrn("media:unknown", nil, registry)
	assert.Error(t, err)
}

// TestIntegrationMediaSpecDefConstruction verifies MediaSpecDef construction
func TestIntegrationMediaSpecDefConstruction(t *testing.T) {
	// Test basic construction
	def := NewMediaSpecDef("media:test;textable", "text/plain", "https://capns.org/schema/str")
	assert.Equal(t, "media:test;textable", def.Urn)
	assert.Equal(t, "text/plain", def.MediaType)
	assert.Equal(t, "https://capns.org/schema/str", def.ProfileURI)

	// Test with title
	defWithTitle := NewMediaSpecDefWithTitle("media:test;textable", "text/plain", "https://example.com/schema", "Test Title")
	assert.Equal(t, "Test Title", defWithTitle.Title)

	// Test object form with schema
	schema := map[string]interface{}{"type": "object"}
	schemaDef := NewMediaSpecDefWithSchema("media:test;json", "application/json", "https://example.com/schema", schema)
	assert.NotNil(t, schemaDef.Schema)
}

// CBOR Integration Tests (TEST284-303)
// These tests verify the CBOR plugin communication protocol between host and plugin

const testCBORManifest = `{"name":"TestPlugin","version":"1.0.0","description":"Test plugin","caps":[{"urn":"cap:in=\"media:void\";op=test;out=\"media:void\"","title":"Test","command":"test"}]}`

// createPipePair creates a pair of connected Unix socket streams for testing
func createPipePair(t *testing.T) (hostWrite, pluginRead, pluginWrite, hostRead net.Conn) {
	// Create two socket pairs
	hostWriteConn, pluginReadConn := createSocketPair(t)
	pluginWriteConn, hostReadConn := createSocketPair(t)
	return hostWriteConn, pluginReadConn, pluginWriteConn, hostReadConn
}

func createSocketPair(t *testing.T) (net.Conn, net.Conn) {
	// Use socketpair for bidirectional communication
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)

	file1 := os.NewFile(uintptr(fds[0]), "socket1")
	file2 := os.NewFile(uintptr(fds[1]), "socket2")

	conn1, err := net.FileConn(file1)
	require.NoError(t, err)
	conn2, err := net.FileConn(file2)
	require.NoError(t, err)

	file1.Close()
	file2.Close()

	return conn1, conn2
}

// TEST284: Test host-plugin handshake exchanges HELLO frames, negotiates limits, and transfers manifest
func TestHandshakeHostPlugin(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var pluginLimits cbor.Limits
	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		assert.True(t, limits.MaxFrame > 0)
		assert.True(t, limits.MaxChunk > 0)
		pluginLimits = limits
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	manifest, hostLimits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)

	// Verify manifest received
	assert.Equal(t, []byte(testCBORManifest), manifest)

	wg.Wait()

	// Both should have negotiated the same limits
	assert.Equal(t, hostLimits.MaxFrame, pluginLimits.MaxFrame)
	assert.Equal(t, hostLimits.MaxChunk, pluginLimits.MaxChunk)
}

// TEST285: Test simple request-response flow: host sends REQ, plugin sends END with payload
func TestRequestResponseSimple(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		// Handshake
		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeReq, frame.FrameType)
		assert.Equal(t, "cap:op=echo", frame.Cap)
		assert.Equal(t, []byte("hello"), frame.Payload)

		// Send response
		response := cbor.NewEnd(frame.Id, []byte("hello back"), "")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	manifest, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	assert.Equal(t, []byte(testCBORManifest), manifest)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=echo", []byte("hello"), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeEnd, response.FrameType)
	assert.Equal(t, []byte("hello back"), response.Payload)

	wg.Wait()
}

// TEST286: Test streaming response with multiple CHUNK frames collected by host
func TestStreamingChunks(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		requestID := frame.Id

		// Send 3 chunks
		chunks := [][]byte{[]byte("chunk1"), []byte("chunk2"), []byte("chunk3")}
		for i, chunk := range chunks {
			chunkFrame := cbor.NewChunk(requestID, uint64(i), chunk)
			if i == 0 {
				chunkFrame.Len = intPtr(18) // total length
			}
			if i == len(chunks)-1 {
				chunkFrame.Eof = boolPtr(true)
			}
			err = writer.WriteFrame(chunkFrame)
			require.NoError(t, err)
		}
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=stream", []byte("go"), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Collect chunks
	var chunks [][]byte
	for i := 0; i < 3; i++ {
		chunk, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeChunk, chunk.FrameType)
		chunks = append(chunks, chunk.Payload)
	}

	assert.Equal(t, 3, len(chunks))
	assert.Equal(t, []byte("chunk1"), chunks[0])
	assert.Equal(t, []byte("chunk2"), chunks[1])
	assert.Equal(t, []byte("chunk3"), chunks[2])

	wg.Wait()
}

// TEST287: Test host-initiated heartbeat is received and responded to by plugin
func TestHeartbeatFromHost(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	done := make(chan bool)

	// Plugin side
	go func() {
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read heartbeat
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeHeartbeat, frame.FrameType)

		// Respond with heartbeat
		response := cbor.NewHeartbeat(frame.Id)
		err = writer.WriteFrame(response)
		require.NoError(t, err)

		done <- true
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send heartbeat
	heartbeatID := cbor.NewMessageIdRandom()
	heartbeat := cbor.NewHeartbeat(heartbeatID)
	err = writer.WriteFrame(heartbeat)
	require.NoError(t, err)

	// Wait for plugin to finish
	<-done

	// Read heartbeat response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeHeartbeat, response.FrameType)
	assert.Equal(t, heartbeatID.ToString(), response.Id.ToString())
}

// TEST288: Test plugin ERR frame is received by host as error
func TestPluginErrorResponse(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)

		// Send error
		errFrame := cbor.NewErr(frame.Id, "NOT_FOUND", "Cap not found: cap:op=missing")
		err = writer.WriteFrame(errFrame)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=missing", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read error response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeErr, response.FrameType)
	assert.Equal(t, "NOT_FOUND", response.Code)
	assert.Contains(t, response.Message, "Cap not found")

	wg.Wait()
}

// TEST289: Test LOG frames sent during a request are transparently skipped by host
func TestLogFramesDuringRequest(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		requestID := frame.Id

		// Send log frames
		log1 := cbor.NewLog(requestID, "info", "Processing started")
		err = writer.WriteFrame(log1)
		require.NoError(t, err)

		log2 := cbor.NewLog(requestID, "debug", "Step 1 complete")
		err = writer.WriteFrame(log2)
		require.NoError(t, err)

		// Send final response
		response := cbor.NewEnd(requestID, []byte("done"), "")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read frames until END (skipping LOG frames)
	for {
		frame, err := reader.ReadFrame()
		require.NoError(t, err)

		if frame.FrameType == cbor.FrameTypeLog {
			// Skip log frames
			continue
		}

		if frame.FrameType == cbor.FrameTypeEnd {
			assert.Equal(t, []byte("done"), frame.Payload)
			break
		}
	}

	wg.Wait()
}

// TEST290: Test limit negotiation picks minimum of host and plugin max_frame and max_chunk
func TestLimitsNegotiation(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	smallLimits := cbor.Limits{
		MaxFrame: 500_000,
		MaxChunk: 50_000,
	}

	var pluginLimits cbor.Limits
	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side with custom limits
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		// Read host HELLO
		hostHello, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeHello, hostHello.FrameType)

		// Send HELLO with smaller limits (manifest only, limits negotiated separately)
		ourHello := cbor.NewHello([]byte(testCBORManifest))
		err = writer.WriteFrame(ourHello)
		require.NoError(t, err)

		// Negotiate
		var hostLimits cbor.Limits
		cbor.DecodeCBOR(hostHello.Payload, &hostLimits)
		pluginLimits = cbor.NegotiateLimits(smallLimits, hostLimits)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, hostLimits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)

	wg.Wait()

	// Host should have negotiated to smaller limits
	assert.Equal(t, 500_000, hostLimits.MaxFrame)
	assert.Equal(t, 50_000, hostLimits.MaxChunk)
	assert.Equal(t, hostLimits.MaxFrame, pluginLimits.MaxFrame)
	assert.Equal(t, hostLimits.MaxChunk, pluginLimits.MaxChunk)
}

// TEST291: Test binary payload with all 256 byte values roundtrips through host-plugin communication
func TestBinaryPayloadRoundtrip(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	// Create binary test data with all byte values
	binaryData := make([]byte, 256)
	for i := 0; i < 256; i++ {
		binaryData[i] = byte(i)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		payload := frame.Payload

		// Verify all bytes
		assert.Equal(t, 256, len(payload))
		for i := 0; i < 256; i++ {
			assert.Equal(t, byte(i), payload[i], "Byte mismatch at position %d", i)
		}

		// Echo back
		response := cbor.NewEnd(frame.Id, payload, "application/octet-stream")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send binary data
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=binary", binaryData, "application/octet-stream")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	result := response.Payload

	// Verify response
	assert.Equal(t, 256, len(result))
	for i := 0; i < 256; i++ {
		assert.Equal(t, byte(i), result[i], "Response byte mismatch at position %d", i)
	}

	wg.Wait()
}

// TEST292: Test three sequential requests get distinct MessageIds on the wire
func TestMessageIdUniqueness(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var receivedIDs []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read 3 requests
		for i := 0; i < 3; i++ {
			frame, err := reader.ReadFrame()
			require.NoError(t, err)

			mu.Lock()
			receivedIDs = append(receivedIDs, frame.Id.ToString())
			mu.Unlock()

			response := cbor.NewEnd(frame.Id, []byte("ok"), "")
			err = writer.WriteFrame(response)
			require.NoError(t, err)
		}
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send 3 requests
	for i := 0; i < 3; i++ {
		requestID := cbor.NewMessageIdRandom()
		request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "application/json")
		err = writer.WriteFrame(request)
		require.NoError(t, err)

		// Read response
		_, err = reader.ReadFrame()
		require.NoError(t, err)
	}

	wg.Wait()

	// Verify IDs are unique
	assert.Equal(t, 3, len(receivedIDs))
	for i := 0; i < len(receivedIDs); i++ {
		for j := i + 1; j < len(receivedIDs); j++ {
			assert.NotEqual(t, receivedIDs[i], receivedIDs[j], "IDs should be unique")
		}
	}
}

// TEST293: Test PluginRuntime handler registration and lookup by exact and non-existent cap URN
func TestPluginRuntimeHandlerRegistration(t *testing.T) {
	runtime, err := NewPluginRuntime([]byte(testCBORManifest))
	require.NoError(t, err)

	runtime.Register(`cap:in="media:void";op=echo;out="media:void"`,
		func(payload []byte, emitter StreamEmitter, peer PeerInvoker) ([]byte, error) {
			return payload, nil
		})

	runtime.Register(`cap:in="media:void";op=transform;out="media:void"`,
		func(payload []byte, emitter StreamEmitter, peer PeerInvoker) ([]byte, error) {
			return []byte("transformed"), nil
		})

	// Exact match
	assert.NotNil(t, runtime.FindHandler(`cap:in="media:void";op=echo;out="media:void"`))
	assert.NotNil(t, runtime.FindHandler(`cap:in="media:void";op=transform;out="media:void"`))

	// Non-existent
	assert.Nil(t, runtime.FindHandler(`cap:in="media:void";op=unknown;out="media:void"`))
}

// TEST294: Test plugin-initiated heartbeat mid-stream is handled transparently by host
func TestHeartbeatDuringStreaming(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		requestID := frame.Id

		// Send chunk 1
		chunk1 := cbor.NewChunk(requestID, 0, []byte("part1"))
		err = writer.WriteFrame(chunk1)
		require.NoError(t, err)

		// Send heartbeat
		heartbeatID := cbor.NewMessageIdRandom()
		heartbeat := cbor.NewHeartbeat(heartbeatID)
		err = writer.WriteFrame(heartbeat)
		require.NoError(t, err)

		// Wait for heartbeat response
		hbResponse, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeHeartbeat, hbResponse.FrameType)
		assert.Equal(t, heartbeatID.ToString(), hbResponse.Id.ToString())

		// Send final chunk
		chunk2 := cbor.NewChunk(requestID, 1, []byte("part2"))
		chunk2.Eof = boolPtr(true)
		err = writer.WriteFrame(chunk2)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=stream", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Collect chunks, handling heartbeat mid-stream
	var chunks [][]byte
	for {
		frame, err := reader.ReadFrame()
		require.NoError(t, err)

		if frame.FrameType == cbor.FrameTypeHeartbeat {
			// Respond to heartbeat
			hbResponse := cbor.NewHeartbeat(frame.Id)
			err = writer.WriteFrame(hbResponse)
			require.NoError(t, err)
			continue
		}

		if frame.FrameType == cbor.FrameTypeChunk {
			chunks = append(chunks, frame.Payload)
			if frame.Eof != nil && *frame.Eof {
				break
			}
		}
	}

	assert.Equal(t, 2, len(chunks))
	assert.Equal(t, []byte("part1"), chunks[0])
	assert.Equal(t, []byte("part2"), chunks[1])

	wg.Wait()
}

// TEST295: Test RES frame (not END) is received correctly as single complete response
func TestResFrameSingleResponse(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)

		// Send RES frame
		response := cbor.NewRes(frame.Id, []byte("single response"), "application/octet-stream")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=single", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeRes, response.FrameType)
	assert.Equal(t, []byte("single response"), response.Payload)

	wg.Wait()
}

// TEST296: Test host does not echo back plugin's heartbeat response (no infinite ping-pong)
func TestHostInitiatedHeartbeatNoPingPong(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	done := make(chan bool)

	// Plugin side
	go func() {
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		requestFrame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeReq, requestFrame.FrameType)
		requestID := requestFrame.Id

		// Read heartbeat from host
		heartbeatFrame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeHeartbeat, heartbeatFrame.FrameType)
		heartbeatID := heartbeatFrame.Id

		// Respond to heartbeat
		hbResponse := cbor.NewHeartbeat(heartbeatID)
		err = writer.WriteFrame(hbResponse)
		require.NoError(t, err)

		// Send request response
		response := cbor.NewRes(requestID, []byte("done"), "text/plain")
		err = writer.WriteFrame(response)
		require.NoError(t, err)

		done <- true
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Send heartbeat
	heartbeatID := cbor.NewMessageIdRandom()
	heartbeat := cbor.NewHeartbeat(heartbeatID)
	err = writer.WriteFrame(heartbeat)
	require.NoError(t, err)

	// Read heartbeat response
	hbResponse, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeHeartbeat, hbResponse.FrameType)

	// Read request response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeRes, response.FrameType)
	assert.Equal(t, []byte("done"), response.Payload)

	<-done
}

// TEST297: Test host call with unified CBOR arguments sends correct content_type and payload
func TestUnifiedArgumentsRoundtrip(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, "application/cbor", frame.ContentType, "unified arguments must use application/cbor")

		// Parse CBOR arguments
		var args []map[string]interface{}
		err = DecodeCBORValue(frame.Payload, &args)
		require.NoError(t, err)
		assert.Equal(t, 1, len(args), "should have exactly one argument")

		// Extract value from first argument
		value := args[0]["value"].([]byte)

		// Echo back
		response := cbor.NewEnd(frame.Id, value, "")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Create unified arguments
	args := []CapArgumentValue{
		NewCapArgumentValueFromStr("media:model-spec;textable", "gpt-4"),
	}

	// Encode arguments to CBOR
	argsData, err := EncodeCapArgumentValues(args)
	require.NoError(t, err)

	// Send request with CBOR arguments
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", argsData, "application/cbor")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, []byte("gpt-4"), response.Payload)

	wg.Wait()
}

// TEST298: Test host receives error when plugin closes connection unexpectedly
func TestPluginSuddenDisconnect(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request but don't respond - just close
		_, err = reader.ReadFrame()
		require.NoError(t, err)

		// Close connection
		pluginRead.Close()
		pluginWrite.Close()
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Try to read response - should fail with EOF
	_, err = reader.ReadFrame()
	assert.Error(t, err, "must fail when plugin disconnects")
	assert.Equal(t, io.EOF, err)

	wg.Wait()
}

// TEST299: Test empty payload request and response roundtrip through host-plugin communication
func TestEmptyPayloadRoundtrip(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Empty(t, frame.Payload, "empty payload must arrive empty")

		// Send empty response
		response := cbor.NewEnd(frame.Id, []byte{}, "")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send empty request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=empty", []byte{}, "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Empty(t, response.Payload)

	wg.Wait()
}

// TEST300: Test END frame without payload is handled as complete response with empty data
func TestEndFrameNoPayload(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)

		// Send END with nil payload
		response := cbor.NewEnd(frame.Id, nil, "")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, cbor.FrameTypeEnd, response.FrameType)
	// END with nil payload should be handled cleanly

	wg.Wait()
}

// TEST301: Test streaming response sequence numbers are contiguous and start from 0
func TestStreamingSequenceNumbers(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		requestID := frame.Id

		// Send 5 chunks with explicit sequence numbers
		for seq := uint64(0); seq < 5; seq++ {
			payload := []byte(string(rune('0' + seq)))
			chunk := cbor.NewChunk(requestID, seq, payload)
			if seq == 4 {
				chunk.Eof = boolPtr(true)
			}
			err = writer.WriteFrame(chunk)
			require.NoError(t, err)
		}
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "text/plain")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Collect chunks
	var chunks []*cbor.Frame
	for i := 0; i < 5; i++ {
		chunk, err := reader.ReadFrame()
		require.NoError(t, err)
		chunks = append(chunks, chunk)
	}

	// Verify sequence numbers
	assert.Equal(t, 5, len(chunks))
	for i, chunk := range chunks {
		assert.Equal(t, uint64(i), chunk.Seq, "chunk seq must be contiguous from 0")
	}
	assert.True(t, *chunks[4].Eof)

	wg.Wait()
}

// TEST302: Test host request on a closed host returns error
func TestRequestAfterShutdown(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		_, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)

		// Close immediately
		pluginRead.Close()
		pluginWrite.Close()
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	wg.Wait()

	// Close host connections
	hostWrite.Close()
	hostRead.Close()

	// Try to send request on closed connection - should fail
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", []byte(""), "application/json")
	err = writer.WriteFrame(request)
	assert.Error(t, err, "must fail on closed connection")
}

// TEST303: Test multiple unified arguments are correctly serialized in CBOR payload
func TestUnifiedArgumentsMultiple(t *testing.T) {
	hostWrite, pluginRead, pluginWrite, hostRead := createPipePair(t)
	defer hostWrite.Close()
	defer pluginRead.Close()
	defer pluginWrite.Close()
	defer hostRead.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	// Plugin side
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(pluginRead)
		writer := cbor.NewFrameWriter(pluginWrite)

		limits, err := cbor.HandshakeAccept(reader, writer, []byte(testCBORManifest))
		require.NoError(t, err)
		reader.SetLimits(limits)
		writer.SetLimits(limits)

		// Read request
		frame, err := reader.ReadFrame()
		require.NoError(t, err)

		// Parse CBOR arguments
		var args []map[string]interface{}
		err = DecodeCBORValue(frame.Payload, &args)
		require.NoError(t, err)
		assert.Equal(t, 2, len(args), "should have 2 arguments")

		// Send response
		responseMsg := []byte("got 2 args")
		response := cbor.NewEnd(frame.Id, responseMsg, "")
		err = writer.WriteFrame(response)
		require.NoError(t, err)
	}()

	// Host side
	reader := cbor.NewFrameReader(hostRead)
	writer := cbor.NewFrameWriter(hostWrite)

	_, limits, err := cbor.HandshakeInitiate(reader, writer)
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	// Create multiple unified arguments
	args := []CapArgumentValue{
		NewCapArgumentValueFromStr("media:model-spec;textable", "gpt-4"),
		NewCapArgumentValue("media:pdf;bytes", []byte{0x89, 0x50, 0x4E, 0x47}),
	}

	// Encode arguments to CBOR
	argsData, err := EncodeCapArgumentValues(args)
	require.NoError(t, err)

	// Send request
	requestID := cbor.NewMessageIdRandom()
	request := cbor.NewReq(requestID, "cap:op=test", argsData, "application/cbor")
	err = writer.WriteFrame(request)
	require.NoError(t, err)

	// Read response
	response, err := reader.ReadFrame()
	require.NoError(t, err)
	assert.Equal(t, []byte("got 2 args"), response.Payload)

	wg.Wait()
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

// DecodeCBORValue decodes CBOR bytes to any interface{}
func DecodeCBORValue(data []byte, v interface{}) error {
	return cbor2.Unmarshal(data, v)
}

// EncodeCapArgumentValues encodes CapArgumentValue slice to CBOR
func EncodeCapArgumentValues(args []CapArgumentValue) ([]byte, error) {
	// Convert to CBOR-friendly format
	var cborArgs []map[string]interface{}
	for _, arg := range args {
		argMap := map[string]interface{}{
			"media_urn": arg.MediaUrn,
			"value":     arg.Value,
		}
		cborArgs = append(cborArgs, argMap)
	}

	return cbor2.Marshal(cborArgs)
}
