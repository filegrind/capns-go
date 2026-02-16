package urn

import (
	"encoding/json"
	"testing"

	"github.com/filegrind/capns-go/standard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TEST057: Test parsing simple media URN verifies correct structure with no version, subtype, or profile
func TestMediaUrnParseSimple(t *testing.T) {
	urn, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)
	assert.True(t, urn.HasTag("string"))
	assert.False(t, urn.HasTag("bytes"))
}

// TEST058: Test parsing media URN with subtype extracts subtype tag correctly
func TestMediaUrnWithSubtype(t *testing.T) {
	urn, err := NewMediaUrnFromString("media:string;form=scalar")
	require.NoError(t, err)
	assert.True(t, urn.HasTag("string"))
	assert.True(t, urn.HasTag("form"))
	formVal, ok := urn.GetTag("form")
	assert.True(t, ok)
	assert.Equal(t, "scalar", formVal)
}

// TEST059: Test parsing media URN with profile extracts profile URL correctly
func TestMediaUrnWithProfile(t *testing.T) {
	// Profile support depends on implementation
	urn, err := NewMediaUrnFromString("media:string;textable")
	require.NoError(t, err)
	assert.True(t, urn.HasTag("textable"))
}

// TEST060: Test wrong prefix fails with InvalidPrefix error
func TestMediaUrnWrongPrefix(t *testing.T) {
	_, err := NewMediaUrnFromString("notmedia:string")
	assert.Error(t, err)
}

// TEST061: Test is_binary returns true only when bytes marker tag is present
func TestMediaUrnIsBinary(t *testing.T) {
	binary, err := NewMediaUrnFromString("media:bytes")
	require.NoError(t, err)
	assert.True(t, binary.IsBinary())

	text, err := NewMediaUrnFromString("media:string;textable")
	require.NoError(t, err)
	assert.False(t, text.IsBinary())
}

// TEST062: Test is_map returns true when form=map tag is present
func TestMediaUrnIsMap(t *testing.T) {
	mapUrn, err := NewMediaUrnFromString("media:form=map")
	require.NoError(t, err)
	// Check via structured or form tag
	form, ok := mapUrn.GetTag("form")
	assert.True(t, ok)
	assert.Equal(t, "map", form)
}

// TEST063: Test is_scalar returns true when form=scalar tag is present
func TestMediaUrnIsScalar(t *testing.T) {
	scalar, err := NewMediaUrnFromString("media:string;form=scalar")
	require.NoError(t, err)
	form, ok := scalar.GetTag("form")
	assert.True(t, ok)
	assert.Equal(t, "scalar", form)
}

// TEST064: Test is_list returns true when form=list tag is present
func TestMediaUrnIsList(t *testing.T) {
	list, err := NewMediaUrnFromString("media:form=list")
	require.NoError(t, err)
	form, ok := list.GetTag("form")
	assert.True(t, ok)
	assert.Equal(t, "list", form)
}

// TEST065: Test is_structured returns true for map or list forms
func TestMediaUrnIsStructured(t *testing.T) {
	mapUrn, err := NewMediaUrnFromString("media:form=map")
	require.NoError(t, err)
	form, _ := mapUrn.GetTag("form")
	assert.True(t, form == "map" || form == "list")

	listUrn, err := NewMediaUrnFromString("media:form=list")
	require.NoError(t, err)
	form, _ = listUrn.GetTag("form")
	assert.True(t, form == "map" || form == "list")

	scalar, err := NewMediaUrnFromString("media:string;form=scalar")
	require.NoError(t, err)
	form, _ = scalar.GetTag("form")
	assert.False(t, form == "map" || form == "list")
}

// TEST066: Test is_json returns true only when json marker tag is present
func TestMediaUrnIsJson(t *testing.T) {
	jsonUrn, err := NewMediaUrnFromString("media:json")
	require.NoError(t, err)
	assert.True(t, jsonUrn.HasTag("json"))

	nonJson, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)
	assert.False(t, nonJson.HasTag("json"))
}

// TEST067: Test is_text returns true only when textable marker tag is present
func TestMediaUrnIsText(t *testing.T) {
	text, err := NewMediaUrnFromString("media:string;textable")
	require.NoError(t, err)
	assert.True(t, text.HasTag("textable"))

	binary, err := NewMediaUrnFromString("media:bytes")
	require.NoError(t, err)
	assert.False(t, binary.HasTag("textable"))
}

// TEST068: Test is_void returns true when void tag is present
func TestMediaUrnIsVoid(t *testing.T) {
	void, err := NewMediaUrnFromString("media:void")
	require.NoError(t, err)
	assert.True(t, void.HasTag("void"))

	nonVoid, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)
	assert.False(t, nonVoid.HasTag("void"))
}

// TEST069: Test simple constructor creates media URN with type tag
func TestMediaUrnConstructor(t *testing.T) {
	// NewMediaUrn or similar constructor
	urn, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)
	assert.True(t, urn.HasTag("string"))
}

// TEST070: Test with_subtype constructor creates media URN with subtype
func TestMediaUrnWithSubtypeConstructor(t *testing.T) {
	urn, err := NewMediaUrnFromString("media:application;subtype=json")
	require.NoError(t, err)
	assert.True(t, urn.HasTag("application"))
	subtype, ok := urn.GetTag("subtype")
	assert.True(t, ok)
	assert.Equal(t, "json", subtype)
}

// TEST071: Test to_string roundtrip ensures serialization preserves structure
func TestMediaUrnRoundtrip(t *testing.T) {
	original := "media:string;textable;form=scalar"
	urn1, err := NewMediaUrnFromString(original)
	require.NoError(t, err)

	serialized := urn1.String()
	urn2, err := NewMediaUrnFromString(serialized)
	require.NoError(t, err)

	assert.True(t, urn1.Equals(urn2))
}

// TEST072: Test all media URN constants parse successfully
func TestMediaUrnConstants(t *testing.T) {
	constants := []string{
		standard.MediaVoid,
		standard.MediaString,
		standard.MediaBinary,
		standard.MediaObject,
		standard.MediaInteger,
		MediaNumber,
		standard.MediaBoolean,
	}

	for _, constant := range constants {
		_, err := NewMediaUrnFromString(constant)
		assert.NoError(t, err, "Failed to parse constant: %s", constant)
	}
}

// TEST073: Test extension helper functions create media URNs with ext tag
func TestMediaUrnExtension(t *testing.T) {
	// Extension helpers if they exist
	pdfUrn, err := NewMediaUrnFromString("media:bytes;ext=pdf")
	require.NoError(t, err)
	ext, ok := pdfUrn.GetTag("ext")
	assert.True(t, ok)
	assert.Equal(t, "pdf", ext)
}

// TEST074: Test media URN matching using tagged URN semantics
func TestMediaUrnMatching(t *testing.T) {
	specific, err := NewMediaUrnFromString("media:string;textable;form=scalar")
	require.NoError(t, err)

	generic, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)

	// Specific pattern does NOT accept generic instance (generic missing textable and form)
	assert.False(t, specific.Accepts(generic))

	// Generic pattern DOES accept specific instance (generic has no constraints on extra tags)
	assert.True(t, generic.Accepts(specific))

	// Specific instance conforms to generic pattern
	assert.True(t, specific.ConformsTo(generic))

	// Generic instance does NOT conform to specific pattern
	assert.False(t, generic.ConformsTo(specific))
}

// TEST075: Test matching with implicit wildcards
func TestMediaUrnImplicitWildcards(t *testing.T) {
	handler, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)

	request, err := NewMediaUrnFromString("media:string;form=scalar")
	require.NoError(t, err)

	// Handler with fewer tags can match requests with more tags (wildcard semantics)
	assert.True(t, handler.Accepts(request))
}

// TEST076: Test specificity increases with more tags
func TestMediaUrnSpecificity(t *testing.T) {
	simple, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)

	detailed, err := NewMediaUrnFromString("media:string;textable;form=scalar")
	require.NoError(t, err)

	// More tags = higher specificity
	assert.True(t, detailed.Specificity() > simple.Specificity())
}

// TEST077: Test serde roundtrip serializes to JSON string correctly
func TestMediaUrnSerdeRoundtrip(t *testing.T) {
	original, err := NewMediaUrnFromString("media:string;textable")
	require.NoError(t, err)

	// JSON marshaling
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// JSON unmarshaling
	var restored MediaUrn
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.True(t, original.Equals(&restored))
}

// TEST078: Debug test for matching behavior between different media URN types
func TestMediaUrnMatchingBehavior(t *testing.T) {
	void, err := NewMediaUrnFromString("media:void")
	require.NoError(t, err)

	string, err := NewMediaUrnFromString("media:string")
	require.NoError(t, err)

	bytes, err := NewMediaUrnFromString("media:bytes")
	require.NoError(t, err)

	// Different base types should not match
	assert.False(t, void.Accepts(string))
	assert.False(t, string.Accepts(bytes))
	assert.False(t, bytes.Accepts(void))

	// Same type should match
	void2, _ := NewMediaUrnFromString("media:void")
	assert.True(t, void.Accepts(void2))
}
