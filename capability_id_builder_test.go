package capability_sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityIdBuilderBasicConstruction(t *testing.T) {
	capabilityId, err := NewCapabilityIdBuilder().
		AddSegment("data_processing").
		AddSegment("transform").
		AddSegment("json").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data_processing:transform:json", capabilityId.ToString())
}

func TestCapabilityIdBuilderFromString(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("extract:metadata:pdf")
	require.NoError(t, err)

	capabilityId, err := builder.Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:pdf", capabilityId.ToString())
}

func TestCapabilityIdBuilderMakeMoreGeneral(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("data_processing:transform:json")
	require.NoError(t, err)

	capabilityId, err := builder.MakeMoreGeneral().Build()
	require.NoError(t, err)
	assert.Equal(t, "data_processing:transform", capabilityId.ToString())
}

func TestCapabilityIdBuilderMakeWildcard(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("data_processing:transform:json")
	require.NoError(t, err)

	capabilityId, err := builder.MakeWildcard().Build()
	require.NoError(t, err)
	assert.Equal(t, "data_processing:transform:*", capabilityId.ToString())
}

func TestCapabilityIdBuilderAddWildcard(t *testing.T) {
	capabilityId, err := NewCapabilityIdBuilder().
		AddSegment("data_processing").
		AddWildcard().
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data_processing:*", capabilityId.ToString())
}

func TestCapabilityIdBuilderReplaceSegment(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("extract:metadata:pdf")
	require.NoError(t, err)

	capabilityId, err := builder.ReplaceSegment(2, "xml").Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:xml", capabilityId.ToString())
}

func TestCapabilityIdBuilderAddSegments(t *testing.T) {
	capabilityId, err := NewCapabilityIdBuilder().
		AddSegments("data", "processing").
		AddSegment("json").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data:processing:json", capabilityId.ToString())
}

func TestCapabilityIdBuilderAddSegmentsFromSlice(t *testing.T) {
	segments := []string{"data", "processing"}
	capabilityId, err := NewCapabilityIdBuilder().
		AddSegmentsFromSlice(segments).
		AddSegment("json").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data:processing:json", capabilityId.ToString())
}

func TestCapabilityIdBuilderMakeGeneralToLevel(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("a:b:c:d:e")
	require.NoError(t, err)

	capabilityId, err := builder.MakeGeneralToLevel(2).Build()
	require.NoError(t, err)
	assert.Equal(t, "a:b", capabilityId.ToString())
}

func TestCapabilityIdBuilderMakeWildcardFromLevel(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("data:processing:transform:json")
	require.NoError(t, err)

	capabilityId, err := builder.MakeWildcardFromLevel(2).Build()
	require.NoError(t, err)
	assert.Equal(t, "data:processing:*", capabilityId.ToString())
}

func TestCapabilityIdBuilderClear(t *testing.T) {
	builder, err := NewCapabilityIdBuilderFromString("data:processing:transform")
	require.NoError(t, err)

	assert.Equal(t, 3, builder.Len())
	assert.False(t, builder.IsEmpty())

	builder.Clear()
	assert.Equal(t, 0, builder.Len())
	assert.True(t, builder.IsEmpty())
}

func TestCapabilityIdBuilderClone(t *testing.T) {
	original, err := NewCapabilityIdBuilderFromString("data:processing:transform")
	require.NoError(t, err)

	clone := original.Clone()

	// Modify original
	original.AddSegment("json")

	// Clone should remain unchanged
	originalId, err := original.Build()
	require.NoError(t, err)
	assert.Equal(t, "data:processing:transform:json", originalId.ToString())

	cloneId, err := clone.Build()
	require.NoError(t, err)
	assert.Equal(t, "data:processing:transform", cloneId.ToString())
}

func TestCapabilityIdBuilderBuildString(t *testing.T) {
	builder := NewCapabilityIdBuilder().
		AddSegment("extract").
		AddSegment("metadata").
		AddWildcard()

	str, err := builder.BuildString()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:*", str)
}

func TestCapabilityIdBuilderHelperFunctions(t *testing.T) {
	// Test StringIntoBuilder
	builder1, err := StringIntoBuilder("extract:metadata:pdf")
	require.NoError(t, err)
	capId1, err := builder1.Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:pdf", capId1.ToString())

	// Test CapabilityIdIntoBuilder
	capId, err := NewCapabilityIdFromString("extract:metadata:pdf")
	require.NoError(t, err)
	builder2, err := CapabilityIdIntoBuilder(capId)
	require.NoError(t, err)
	capId2, err := builder2.Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:pdf", capId2.ToString())
}

func TestCapabilityIdBuilderEdgeCases(t *testing.T) {
	// Test replace segment with invalid index
	builder := NewCapabilityIdBuilder().AddSegment("test")
	builder.ReplaceSegment(5, "invalid") // Should not crash
	capId, err := builder.Build()
	require.NoError(t, err)
	assert.Equal(t, "test", capId.ToString())

	// Test make more general on empty builder
	emptyBuilder := NewCapabilityIdBuilder()
	emptyBuilder.MakeMoreGeneral() // Should not crash
	assert.True(t, emptyBuilder.IsEmpty())

	// Test make wildcard on empty builder
	emptyBuilder2 := NewCapabilityIdBuilder()
	emptyBuilder2.MakeWildcard() // Should not crash
	assert.True(t, emptyBuilder2.IsEmpty())
}