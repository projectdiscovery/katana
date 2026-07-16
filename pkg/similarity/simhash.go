package similarity

import (
	"hash/fnv"
	"math/bits"
)

// SimHash64 computes a 64-bit Charikar simhash over weighted features.
func SimHash64(features []string) uint64 {
	if len(features) == 0 {
		return 0
	}
	var weights [64]int
	counts := make(map[string]int, len(features))
	for _, f := range features {
		counts[f]++
	}
	for feature, weight := range counts {
		h := hash64(feature)
		for bit := 0; bit < 64; bit++ {
			if (h>>uint(bit))&1 == 1 {
				weights[bit] += weight
			} else {
				weights[bit] -= weight
			}
		}
	}
	var fingerprint uint64
	for bit := 0; bit < 64; bit++ {
		if weights[bit] > 0 {
			fingerprint |= 1 << uint(bit)
		}
	}
	return fingerprint
}

// HammingDistance returns the number of differing bits between two fingerprints.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
