package simhash

import (
	"strings"
	"testing"
)

var (
	htmlA = `<html><body><p>Hello World</p></body></html>`
	// htmlB differs by an exclamation mark. This should keep the documents fairly similar
	// while still resulting in a different SimHash fingerprint.
	htmlB = `<html><body><p>Hello World!</p></body></html>`
)

// TestFingerprintDeterministic ensures that hashing the same document twice produces
// identical fingerprints and that a shingle value of 0 gracefully falls back to 1.
func TestFingerprintDeterministic(t *testing.T) {
	in1 := strings.NewReader(htmlA)
	in2 := strings.NewReader(htmlA)

	fp1 := Fingerprint(in1, 0)
	fp2 := Fingerprint(in2, 1)

	if fp1 != fp2 {
		t.Fatalf("expected identical fingerprints, got %d and %d", fp1, fp2)
	}
}

// TestFingerprintSimilarity checks that two similar documents yield fingerprints with
// a small (non-zero) Hamming distance.
func TestFingerprintSimilarity(t *testing.T) {
	fpA := Fingerprint(strings.NewReader(htmlA), 3)
	fpB := Fingerprint(strings.NewReader(htmlB), 3)

	d := Distance(fpA, fpB)
	if d == 0 {
		t.Fatalf("expected different fingerprints, got distance 0")
	}

	const maxReasonableDistance = 20 // out of a maximum of 64
	if d > maxReasonableDistance {
		t.Fatalf("expected similar documents to have distance <= %d, got %d", maxReasonableDistance, d)
	}
}

// TestOracle validates the basic behaviour of the Oracle structure.
func TestOracle(t *testing.T) {
	fpA := Fingerprint(strings.NewReader(htmlA), 3)
	fpB := Fingerprint(strings.NewReader(htmlB), 3)

	o := NewOracle()

	if o.Seen(fpA, 0) {
		t.Fatalf("oracle should not have seen fingerprint yet")
	}

	// Teach the oracle about fpA.
	o.See(fpA)

	if !o.Seen(fpA, 0) {
		t.Fatalf("oracle should recognise an identical fingerprint once seen")
	}

	if o.Seen(fpB, 0) {
		t.Fatalf("oracle should not treat different fingerprint as identical when r=0")
	}

	r := Distance(fpA, fpB)
	if !o.Seen(fpB, r) {
		t.Fatalf("oracle should recognise fingerprint within distance %d", r)
	}
}
