package source

import (
	"math/rand"
	"strings"

	"github.com/projectdiscovery/gologger"
)

const MultipleKeyPartsLength = 2

func PickRandom[T any](v []T, sourceName string) T {
	var result T
	length := len(v)
	if length == 0 {
		gologger.Debug().Msgf("Cannot use the %s source because there was no API key/secret defined for it.", sourceName)
		return result
	}
	return v[rand.Intn(length)]
}

func CreateApiKeys[T any](keys []string, provider func(k, v string) T) []T {
	var result []T
	for _, key := range keys {
		if keyPartA, keyPartB, ok := createMultiPartKey(key); ok {
			result = append(result, provider(keyPartA, keyPartB))
		}
	}
	return result
}

func createMultiPartKey(key string) (keyPartA, keyPartB string, ok bool) {
	parts := strings.Split(key, ":")
	ok = len(parts) == MultipleKeyPartsLength

	if ok {
		keyPartA = parts[0]
		keyPartB = parts[1]
	}

	return
}
