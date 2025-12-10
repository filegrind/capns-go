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
	// Setup cap definitions
	stringCapUrn, _ := NewCapUrnFromString("cap:action=test")
	stringCap := NewCap(stringCapUrn, "String Test", "test")
	stringCap.SetOutput(NewCapOutput(OutputTypeString, "String output"))
	
	binaryCapUrn, _ := NewCapUrnFromString("cap:action=test")
	binaryCap := NewCap(binaryCapUrn, "Binary Test", "test")
	binaryCap.SetOutput(NewCapOutput(OutputTypeBinary, "Binary output"))
	
	// Test text response with string output type
	textResponse := NewResponseWrapperFromText([]byte("test"))
	assert.True(t, textResponse.MatchesOutputType(stringCap))
	assert.False(t, textResponse.MatchesOutputType(binaryCap))
	
	// Test binary response with binary output type
	binaryResponse := NewResponseWrapperFromBinary([]byte{1, 2, 3})
	assert.False(t, binaryResponse.MatchesOutputType(stringCap))
	assert.True(t, binaryResponse.MatchesOutputType(binaryCap))
	
	// Test JSON response (should match multiple types)
	jsonResponse := NewResponseWrapperFromJSON([]byte("\"test\""))
	assert.True(t, jsonResponse.MatchesOutputType(stringCap))
	assert.False(t, jsonResponse.MatchesOutputType(binaryCap))
}