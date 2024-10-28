package utils

import mapsutil "github.com/projectdiscovery/utils/maps"

func MergeDataMaps(dataMap1 *mapsutil.OrderedMap[string, string], dataMap2 mapsutil.OrderedMap[string, string]) {
	dataMap2.Iterate(func(key, value string) bool {
		dataMap1.Set(key, value)
		return true
	})
}
