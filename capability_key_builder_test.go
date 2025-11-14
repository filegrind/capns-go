package capdef

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityKeyBuilderBasicConstruction(t *testing.T) {
	capabilityKey, err := NewCapabilityKeyBuilder().
		Sub("data_processing").
		Sub("transform").
		Sub("json").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data_processing:transform:json", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderFromString(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("extract:metadata:pdf")
	require.NoError(t, err)

	capabilityKey, err := builder.Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:pdf", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderMakeMoreGeneral(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("data_processing:transform:json")
	require.NoError(t, err)

	capabilityKey, err := builder.MakeMoreGeneral().Build()
	require.NoError(t, err)
	assert.Equal(t, "data_processing:transform", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderMakeWildcard(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("data_processing:transform:json")
	require.NoError(t, err)

	capabilityKey, err := builder.MakeWildcard().Build()
	require.NoError(t, err)
	assert.Equal(t, "data_processing:transform:*", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderAddWildcard(t *testing.T) {
	capabilityKey, err := NewCapabilityKeyBuilder().
		Sub("data_processing").
		AddWildcard().
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data_processing:*", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderReplaceSegment(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("extract:metadata:pdf")
	require.NoError(t, err)

	capabilityKey, err := builder.ReplaceSegment(2, "xml").Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:xml", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderSubs(t *testing.T) {
	capabilityKey, err := NewCapabilityKeyBuilder().
		Subs("data", "processing").
		Sub("json").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data:processing:json", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderSubsFromSlice(t *testing.T) {
	segments := []string{"data", "processing"}
	capabilityKey, err := NewCapabilityKeyBuilder().
		SubsFromSlice(segments).
		Sub("json").
		Build()

	require.NoError(t, err)
	assert.Equal(t, "data:processing:json", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderMakeGeneralToLevel(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("a:b:c:d:e")
	require.NoError(t, err)

	capabilityKey, err := builder.MakeGeneralToLevel(2).Build()
	require.NoError(t, err)
	assert.Equal(t, "a:b", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderMakeWildcardFromLevel(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("data:processing:transform:json")
	require.NoError(t, err)

	capabilityKey, err := builder.MakeWildcardFromLevel(2).Build()
	require.NoError(t, err)
	assert.Equal(t, "data:processing:*", capabilityKey.ToString())
}

func TestCapabilityKeyBuilderClear(t *testing.T) {
	builder, err := NewCapabilityKeyBuilderFromString("data:processing:transform")
	require.NoError(t, err)

	assert.Equal(t, 3, builder.Len())
	assert.False(t, builder.IsEmpty())

	builder.Clear()
	assert.Equal(t, 0, builder.Len())
	assert.True(t, builder.IsEmpty())
}

func TestCapabilityKeyBuilderClone(t *testing.T) {
	original, err := NewCapabilityKeyBuilderFromString("data:processing:transform")
	require.NoError(t, err)

	clone := original.Clone()

	// Modify original
	original.Sub("json")

	// Clone should remain unchanged
	originalId, err := original.Build()
	require.NoError(t, err)
	assert.Equal(t, "data:processing:transform:json", originalId.ToString())

	cloneId, err := clone.Build()
	require.NoError(t, err)
	assert.Equal(t, "data:processing:transform", cloneId.ToString())
}

func TestCapabilityKeyBuilderBuildString(t *testing.T) {
	builder := NewCapabilityKeyBuilder().
		Sub("extract").
		Sub("metadata").
		AddWildcard()

	str, err := builder.BuildString()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:*", str)
}

func TestCapabilityKeyBuilderHelperFunctions(t *testing.T) {
	// Test StringIntoBuilder
	builder1, err := StringIntoBuilder("extract:metadata:pdf")
	require.NoError(t, err)
	capId1, err := builder1.Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:pdf", capId1.ToString())

	// Test CapabilityKeyIntoBuilder
	capId, err := NewCapabilityKeyFromString("extract:metadata:pdf")
	require.NoError(t, err)
	builder2, err := CapabilityKeyIntoBuilder(capId)
	require.NoError(t, err)
	capId2, err := builder2.Build()
	require.NoError(t, err)
	assert.Equal(t, "extract:metadata:pdf", capId2.ToString())
}

func TestCapabilityKeyBuilderEdgeCases(t *testing.T) {
	// Test replace segment with invalid index
	builder := NewCapabilityKeyBuilder().Sub("test")
	builder.ReplaceSegment(5, "invalid") // Should not crash
	capId, err := builder.Build()
	require.NoError(t, err)
	assert.Equal(t, "test", capId.ToString())

	// Test make more general on empty builder
	emptyBuilder := NewCapabilityKeyBuilder()
	emptyBuilder.MakeMoreGeneral() // Should not crash
	assert.True(t, emptyBuilder.IsEmpty())

	// Test make wildcard on empty builder
	emptyBuilder2 := NewCapabilityKeyBuilder()
	emptyBuilder2.MakeWildcard() // Should not crash
	assert.True(t, emptyBuilder2.IsEmpty())
}