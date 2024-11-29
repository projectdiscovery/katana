package utils

import (
	"testing"

	mapsutil "github.com/projectdiscovery/utils/maps"
	"github.com/stretchr/testify/require"
)

func TestMergeDataMap(t *testing.T) {
	// Create two data maps
	dataMap1 := mapsutil.NewOrderedMap[string, string]()
	dataMap1.Set("key1", "value1")
	dataMap1.Set("key2", "value2")

	dataMap2 := mapsutil.NewOrderedMap[string, string]()
	dataMap2.Set("key3", "value3")
	dataMap2.Set("key4", "value4")

	// Merge the data maps
	MergeDataMaps(&dataMap1, dataMap2)

	// Verify the merged map contains all the keys and values
	value1, _ := dataMap1.Get("key1")
	value2, _ := dataMap1.Get("key2")
	value3, _ := dataMap1.Get("key3")
	value4, _ := dataMap1.Get("key4")
	require.Equal(t, "value1", value1)
	require.Equal(t, "value2", value2)
	require.Equal(t, "value3", value3)
	require.Equal(t, "value4", value4)
}
