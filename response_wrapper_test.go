package capns

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	// Setup cap definitions with spec IDs
	stringCapUrn, _ := NewCapUrnFromString("cap:op=test")
	stringCap := NewCap(stringCapUrn, "String Test", "test")
	stringCap.SetOutput(NewCapOutput(SpecIDStr, "String output"))

	binaryCapUrn, _ := NewCapUrnFromString("cap:op=test")
	binaryCap := NewCap(binaryCapUrn, "Binary Test", "test")
	binaryCap.SetOutput(NewCapOutput(SpecIDBinary, "Binary output"))

	jsonCapUrn, _ := NewCapUrnFromString("cap:op=test")
	jsonCap := NewCap(jsonCapUrn, "JSON Test", "test")
	jsonCap.SetOutput(NewCapOutput(SpecIDObj, "JSON output"))

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
	noOutputCapUrn, _ := NewCapUrnFromString("cap:op=test")
	noOutputCap := NewCap(noOutputCapUrn, "No Output Test", "test")
	_, err = textResponse.MatchesOutputType(noOutputCap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no output definition")

	// Test cap with unresolvable spec ID - MUST FAIL
	badSpecCapUrn, _ := NewCapUrnFromString("cap:op=test")
	badSpecCap := NewCap(badSpecCapUrn, "Bad Spec Test", "test")
	badSpecCap.SetOutput(NewCapOutput("unknown:spec.v1", "Unknown output"))
	_, err = textResponse.MatchesOutputType(badSpecCap)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve output spec ID")
}

func TestResponseWrapperValidateAgainstCap(t *testing.T) {
	// Setup cap with output schema
	capUrn, _ := NewCapUrnFromString("cap:op=test")
	cap := NewCap(capUrn, "Test Cap", "test")

	// Add custom spec with schema
	cap.AddMediaSpec("my:result.v1", NewMediaSpecDefObjectWithSchema(
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

	cap.SetOutput(NewCapOutput("my:result.v1", "Result output"))

	// Valid JSON response
	validResponse := NewResponseWrapperFromJSON([]byte(`{"status": "ok"}`))
	err := validResponse.ValidateAgainstCap(cap)
	assert.NoError(t, err)

	// Invalid JSON response (missing required field)
	invalidResponse := NewResponseWrapperFromJSON([]byte(`{"other": "value"}`))
	err = invalidResponse.ValidateAgainstCap(cap)
	assert.Error(t, err)
}
