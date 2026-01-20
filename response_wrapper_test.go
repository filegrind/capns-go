package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper for response wrapper tests - use proper media URNs with tags
func respTestUrn(tags string) string {
	if tags == "" {
		return `cap:in="media:type=void;v=1";out="media:type=object;v=1;textable;keyed"`
	}
	return `cap:in="media:type=void;v=1";out="media:type=object;v=1;textable;keyed";` + tags
}

func TestResponseWrapperFromJSON(t *testing.T) {
	testData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}
	jsonBytes, err := json.Marshal(testData)
	require.NoError(t, err)

	response := NewResponseWrapperFromJSON(jsonBytes)

	assert.True(t, response.IsJSON())
	assert.False(t, response.IsText())
	assert.False(t, response.IsBinary())
	assert.Equal(t, len(jsonBytes), response.Size())

	var parsed map[string]interface{}
	err = response.AsType(&parsed)
	assert.NoError(t, err)
	assert.Equal(t, "test", parsed["name"])
	assert.Equal(t, float64(42), parsed["value"]) // JSON numbers are float64
}

func TestResponseWrapperFromText(t *testing.T) {
	testText := "Hello, World!"
	response := NewResponseWrapperFromText([]byte(testText))

	assert.False(t, response.IsJSON())
	assert.True(t, response.IsText())
	assert.False(t, response.IsBinary())

	result, err := response.AsString()
	assert.NoError(t, err)
	assert.Equal(t, testText, result)
}

func TestResponseWrapperFromBinary(t *testing.T) {
	testData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	response := NewResponseWrapperFromBinary(testData)

	assert.False(t, response.IsJSON())
	assert.False(t, response.IsText())
	assert.True(t, response.IsBinary())

	assert.Equal(t, testData, response.AsBytes())
	assert.Equal(t, len(testData), response.Size())

	// Should fail to convert to string
	_, err := response.AsString()
	assert.Error(t, err)
}

func TestResponseWrapperAsInt(t *testing.T) {
	// Test from text
	response := NewResponseWrapperFromText([]byte("42"))
	result, err := response.AsInt()
	assert.NoError(t, err)
	assert.Equal(t, int64(42), result)

	// Test from JSON
	response2 := NewResponseWrapperFromJSON([]byte("123"))
	result2, err := response2.AsInt()
	assert.NoError(t, err)
	assert.Equal(t, int64(123), result2)

	// Test invalid conversion
	response3 := NewResponseWrapperFromText([]byte("not_a_number"))
	_, err = response3.AsInt()
	assert.Error(t, err)
}

func TestResponseWrapperAsFloat(t *testing.T) {
	// Test from text
	response := NewResponseWrapperFromText([]byte("3.14"))
	result, err := response.AsFloat()
	assert.NoError(t, err)
	assert.Equal(t, 3.14, result)

	// Test from JSON
	response2 := NewResponseWrapperFromJSON([]byte("2.71"))
	result2, err := response2.AsFloat()
	assert.NoError(t, err)
	assert.Equal(t, 2.71, result2)
}

func TestResponseWrapperAsBool(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
		hasError bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"1", true, false},
		{"0", false, false},
		{"yes", true, false},
		{"no", false, false},
		{"invalid", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			response := NewResponseWrapperFromText([]byte(tc.input))
			result, err := response.AsBool()

			if tc.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestResponseWrapperIsEmpty(t *testing.T) {
	// Empty response
	response := NewResponseWrapperFromText([]byte{})
	assert.True(t, response.IsEmpty())

	// Non-empty response
	response2 := NewResponseWrapperFromText([]byte("test"))
	assert.False(t, response2.IsEmpty())
}

func TestResponseWrapperGetContentType(t *testing.T) {
	jsonResponse := NewResponseWrapperFromJSON([]byte("{}"))
	assert.Equal(t, "application/json", jsonResponse.GetContentType())

	textResponse := NewResponseWrapperFromText([]byte("test"))
	assert.Equal(t, "text/plain", textResponse.GetContentType())

	binaryResponse := NewResponseWrapperFromBinary([]byte{1, 2, 3})
	assert.Equal(t, "application/octet-stream", binaryResponse.GetContentType())
}

func TestResponseWrapperMatchesOutputType(t *testing.T) {
	// Setup cap definitions with media URNs - all need in/out with proper tags
	stringCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=string;v=1;textable;scalar"`)
	require.NoError(t, err)
	stringCap := NewCap(stringCapUrn, "String Test", "test")
	stringCap.SetOutput(NewCapOutput(MediaString, "String output"))

	binaryCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=raw;v=1;binary"`)
	require.NoError(t, err)
	binaryCap := NewCap(binaryCapUrn, "Binary Test", "test")
	binaryCap.SetOutput(NewCapOutput(MediaBinary, "Binary output"))

	jsonCapUrn, err := NewCapUrnFromString(`cap:in="media:type=void;v=1";op=test;out="media:type=object;v=1;textable;keyed"`)
	require.NoError(t, err)
	jsonCap := NewCap(jsonCapUrn, "JSON Test", "test")
	jsonCap.SetOutput(NewCapOutput(MediaObject, "JSON output"))

	// Test text response with string output type
	textResponse := NewResponseWrapperFromText([]byte("test"))
	matchStr, err := textResponse.MatchesOutputType(stringCap)
	assert.NoError(t, err)
	assert.True(t, matchStr)
	matchBin, err := textResponse.MatchesOutputType(binaryCap)
	assert.NoError(t, err)
	assert.False(t, matchBin)
	matchJson, err := textResponse.MatchesOutputType(jsonCap)
	assert.NoError(t, err)
	assert.False(t, matchJson)

	// Test binary response with binary output type
	binaryResponse := NewResponseWrapperFromBinary([]byte{1, 2, 3})
	matchStr, err = binaryResponse.MatchesOutputType(stringCap)
	assert.NoError(t, err)
	assert.False(t, matchStr)
	matchBin, err = binaryResponse.MatchesOutputType(binaryCap)
	assert.NoError(t, err)
	assert.True(t, matchBin)
	matchJson, err = binaryResponse.MatchesOutputType(jsonCap)
	assert.NoError(t, err)
	assert.False(t, matchJson)

	// Test JSON response (should match JSON types)
	jsonResponse := NewResponseWrapperFromJSON([]byte(`{"test": "value"}`))
	matchStr, err = jsonResponse.MatchesOutputType(stringCap)
	assert.NoError(t, err)
	assert.False(t, matchStr)
	matchBin, err = jsonResponse.MatchesOutputType(binaryCap)
	assert.NoError(t, err)
	assert.False(t, matchBin)
	matchJson, err = jsonResponse.MatchesOutputType(jsonCap)
	assert.NoError(t, err)
	assert.True(t, matchJson)

	// Test cap with no output definition - MUST FAIL
	noOutputCapUrn, err := NewCapUrnFromString(respTestUrn("op=test"))
	require.NoError(t, err)
	noOutputCap := NewCap(noOutputCapUrn, "No Output Test", "test")
	_, err = textResponse.MatchesOutputType(noOutputCap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no output definition")

	// Test cap with unresolvable media URN - MUST FAIL
	badSpecCapUrn, err := NewCapUrnFromString(respTestUrn("op=test"))
	require.NoError(t, err)
	badSpecCap := NewCap(badSpecCapUrn, "Bad Spec Test", "test")
	badSpecCap.SetOutput(NewCapOutput("media:type=unknown;v=1", "Unknown output"))
	_, err = textResponse.MatchesOutputType(badSpecCap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve output media URN")
}

func TestResponseWrapperValidateAgainstCap(t *testing.T) {
	// Setup cap with output schema
	capUrn, err := NewCapUrnFromString(respTestUrn("op=test"))
	require.NoError(t, err)
	cap := NewCap(capUrn, "Test Cap", "test")

	// Add custom spec with schema - needs keyed tag for JSON
	cap.AddMediaSpec("media:type=result;v=1;textable;keyed", NewMediaSpecDefObjectWithSchema(
		"application/json",
		"https://example.com/schema/result",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"status"},
		},
	))

	cap.SetOutput(NewCapOutput("media:type=result;v=1;textable;keyed", "Result output"))

	// Valid JSON response
	validResponse := NewResponseWrapperFromJSON([]byte(`{"status": "ok"}`))
	err = validResponse.ValidateAgainstCap(cap)
	assert.NoError(t, err)

	// Invalid JSON response (missing required field)
	invalidResponse := NewResponseWrapperFromJSON([]byte(`{"other": "value"}`))
	err = invalidResponse.ValidateAgainstCap(cap)
	assert.Error(t, err)
}
