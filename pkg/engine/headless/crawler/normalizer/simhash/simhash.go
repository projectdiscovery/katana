// Package simhash implements SimHash algorithm for near-duplicate detection.
//
// The original algorithm is taken from: https://github.com/yahoo/gryffin/blob/master/html-distance/feature.go
// Optimized implementation with performance improvements.
//
// Original Copyright 2015, Yahoo Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package simhash

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/mfonda/simhash"
	"golang.org/x/net/html"
)

// Constants for optimization
const (
	maxTokens       = 5000
	featuresBufSize = 1000
)

// Pre-allocated buffers for feature generation
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func fingerprintOptimized(r io.Reader, shingle int) uint64 {
	if shingle < 1 {
		shingle = 1
	}

	v := simhash.Vector{}
	z := html.NewTokenizer(r)

	features := make([]string, 0, featuresBufSize)
	window := make([][]byte, shingle)
	windowIndex := 0

	// Single-pass tokenization and feature extraction
	count := 0
	for count < maxTokens {
		if tt := z.Next(); tt == html.ErrorToken {
			break
		}
		t := z.Token()
		count++

		extractFeatures(&t, &features)
	}

	// Process features with shingling
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	for _, f := range features {
		window[windowIndex%shingle] = []byte(f)
		windowIndex++

		buf.Reset()
		for i := 0; i < shingle; i++ {
			if i > 0 {
				buf.WriteByte(' ')
			}
			buf.Write(window[(windowIndex-shingle+i+shingle)%shingle])
		}

		sum := simhash.NewFeature(buf.Bytes()).Sum()

		for i := uint8(0); i < 64; i++ {
			if (sum>>i)&1 == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}

	return simhash.Fingerprint(v)
}

// extractFeatures extracts features from HTML token and appends to slice
func extractFeatures(t *html.Token, features *[]string) {
	// Pre-allocate string builder for efficiency
	var s string

	switch t.Type {
	case html.StartTagToken:
		s = "A:" + t.DataAtom.String()
	case html.EndTagToken:
		s = "B:" + t.DataAtom.String()
	case html.SelfClosingTagToken:
		s = "C:" + t.DataAtom.String()
	case html.DoctypeToken:
		s = "D:" + string(t.Data)
	case html.CommentToken:
		s = "E:" + string(t.Data)
	case html.TextToken:
		s = "F:" + string(t.Data)
	case html.ErrorToken:
		s = "Z:" + string(t.Data)
	default:
		return
	}

	*features = append(*features, s)

	// Process attributes
	for _, attr := range t.Attr {
		switch attr.Key {
		case "class", "name", "rel":
			s = fmt.Sprintf("G:%s:%s:%s", t.DataAtom.String(), attr.Key, attr.Val)
		default:
			s = fmt.Sprintf("G:%s:%s", t.DataAtom.String(), attr.Key)
		}
		*features = append(*features, s)
	}
}

// Fingerprint is the original function signature for compatibility
func Fingerprint(r io.Reader, shingle int) uint64 {
	return fingerprintOptimized(r, shingle)
}

type Oracle struct {
	fingerprint uint64      // node value.
	nodes       [65]*Oracle // leaf nodes
}

// NewOracle return an oracle that could tell if the fingerprint has been seen or not.
func NewOracle() *Oracle {
	return newNode(0)
}

func newNode(f uint64) *Oracle {
	return &Oracle{fingerprint: f}
}

// Distance return the similarity distance between two fingerprint.
func Distance(a, b uint64) uint8 {
	return simhash.Compare(a, b)
}

// See asks the oracle to see the fingerprint.
func (n *Oracle) See(f uint64) *Oracle {
	d := Distance(n.fingerprint, f)

	if d == 0 {
		// current node with same fingerprint.
		return n
	}

	// the target node is already set,
	if c := n.nodes[d]; c != nil {
		return c.See(f)
	}

	n.nodes[d] = newNode(f)
	return n.nodes[d]
}

// Seen asks the oracle if anything closed to the fingerprint in a range (r) is seen before.
func (n *Oracle) Seen(f uint64, r uint8) bool {
	d := Distance(n.fingerprint, f)
	if d <= r {
		return true
	}

	// Check the direct child at distance d first
	if c := n.nodes[d]; c != nil && c.Seen(f, r) {
		return true
	}

	// Optimized search: start from closest distance and expand outward
	for offset := uint8(1); offset <= r; offset++ {
		// Check both directions
		if d >= offset {
			if c := n.nodes[d-offset]; c != nil && c.Seen(f, r) {
				return true
			}
		}
		if d+offset <= 64 {
			if c := n.nodes[d+offset]; c != nil && c.Seen(f, r) {
				return true
			}
		}
	}
	return false
}
