package segment

import (
	"flag"
	"math/rand"
)

// The format tests in internal/format have their own copies of these
// helpers; test files can't be shared across packages.

// updateGolden regenerates testdata/golden_seal.bin.
var updateGolden = flag.Bool("update", false, "regenerate golden testdata")

func randBytes(r *rand.Rand, n int) []byte {
	if n == 0 {
		return nil
	}
	b := make([]byte, n)
	r.Read(b)
	return b
}

// eventsEqual is reflect.DeepEqual with the one wrinkle that
// nil-vs-empty Payload should be treated as equal during property
// testing — though our decoder produces nil for zero-length, we
// guard against test-helper drift.
func eventsEqual(a, b Event) bool {
	if a.Seq != b.Seq || a.WitnessedAt != b.WitnessedAt ||
		a.IndexedAt != b.IndexedAt || a.Kind != b.Kind ||
		a.DID != b.DID || a.Collection != b.Collection ||
		a.Rkey != b.Rkey || a.Rev != b.Rev {
		return false
	}
	if len(a.Payload) != len(b.Payload) {
		return false
	}
	for i := range a.Payload {
		if a.Payload[i] != b.Payload[i] {
			return false
		}
	}
	return true
}
