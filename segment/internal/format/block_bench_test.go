package format

import (
	"bytes"
	"math/rand"
	"testing"
)

// makeBenchEvents builds a deterministic block of events sized for
// the requested benchmark scenario. Using a fixed seed gives stable
// numbers across runs.
func makeBenchEvents(b *testing.B, n int, opts benchOpts) []Event {
	b.Helper()
	r := rand.New(rand.NewSource(42))
	events := make([]Event, n)
	for i := range events {
		ev := Event{
			Seq: uint64(i), WitnessedAt: int64(i), Kind: KindCreate,
			DID: "did:plc:abcdefghijklmnopqrstuvwx",
		}
		switch {
		case opts.zeroPayload:
			ev.Payload = nil
		case opts.identicalPayload:
			ev.Payload = bytes.Repeat([]byte{0xAB}, 512)
		default:
			ev.Payload = randBytes(r, 512)
		}
		ev.Collection = "app.bsky.feed.post"
		ev.Rkey = "3l3qo2vuowo2b"
		ev.Rev = "3l3qo2vutsw2b"
		events[i] = ev
	}
	return events
}

type benchOpts struct {
	zeroPayload      bool
	identicalPayload bool
}

// BenchmarkEncodeColumns isolates the columnar encoder from zstd
// so we can see the per-byte rate of just the layout step. The
// compressed benchmarks below mix in zstd time, which on random
// payloads dominates wall-clock — but in production the writer
// reuses a scratch buffer and the columnar step is what we have
// the most control over.
func BenchmarkEncodeColumns(b *testing.B) {
	cases := []struct {
		name string
		n    int
		opts benchOpts
	}{
		{"256_events_random", 256, benchOpts{}},
		{"4096_events_random", 4096, benchOpts{}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			events := makeBenchEvents(b, tc.n, tc.opts)
			scratch := make([]byte, 0, 8<<20)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				scratch = encodeBlockInto(scratch[:0], eventColumns(events))
				b.SetBytes(int64(len(scratch)))
			}
		})
	}
}

// BenchmarkEncodeColumnsPending exercises the writer's hot path:
// the pendingBlock columns implementation backed by parallel slices.
// This is what production Append/Flush actually feeds the encoder.
func BenchmarkEncodeColumnsPending(b *testing.B) {
	cases := []struct {
		name string
		n    int
		opts benchOpts
	}{
		{"4096_events_random", 4096, benchOpts{}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			events := makeBenchEvents(b, tc.n, tc.opts)
			var p pendingBlock
			p.preallocate(tc.n)
			for _, ev := range events {
				p.seq = append(p.seq, ev.Seq)
				p.witnessedAt = append(p.witnessedAt, ev.WitnessedAt)
				p.indexedAt = append(p.indexedAt, ev.IndexedAt)
				p.kind = append(p.kind, uint8(ev.Kind))
				p.collLen = append(p.collLen, uint8(len(ev.Collection)))
				p.didLen = append(p.didLen, uint16(len(ev.DID)))
				p.rkeyLen = append(p.rkeyLen, uint8(len(ev.Rkey)))
				p.revLen = append(p.revLen, uint8(len(ev.Rev)))
				p.eventLen = append(p.eventLen, uint32(len(ev.Payload)))
				p.collections = append(p.collections, ev.Collection...)
				p.dids = append(p.dids, ev.DID...)
				p.rkeys = append(p.rkeys, ev.Rkey...)
				p.revs = append(p.revs, ev.Rev...)
				p.payloads = append(p.payloads, ev.Payload...)
			}
			scratch := make([]byte, 0, 8<<20)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				scratch = encodeBlockInto(scratch[:0], &p)
				b.SetBytes(int64(len(scratch)))
			}
		})
	}
}

// BenchmarkDecodeColumns isolates the uncompressed-body decoder from
// zstd so we can see the cost of the parsing step alone.
func BenchmarkDecodeColumns(b *testing.B) {
	cases := []struct {
		name string
		n    int
		opts benchOpts
	}{
		{"256_events_random", 256, benchOpts{}},
		{"4096_events_random", 4096, benchOpts{}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			events := makeBenchEvents(b, tc.n, tc.opts)
			body, err := encodeBlock(events)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(body)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := decodeBlock(body); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEncodeBlock(b *testing.B) {
	cases := []struct {
		name string
		n    int
		opts benchOpts
	}{
		{"256_events_random", 256, benchOpts{}},
		{"4096_events_random", 4096, benchOpts{}},
		{"4096_events_zero_payload", 4096, benchOpts{zeroPayload: true}},
		{"4096_events_identical", 4096, benchOpts{identicalPayload: true}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			events := makeBenchEvents(b, tc.n, tc.opts)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				out, err := encodeBlockCompressed(events)
				if err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(out)))
			}
		})
	}
}

func BenchmarkDecodeBlock(b *testing.B) {
	cases := []struct {
		name string
		n    int
		opts benchOpts
	}{
		{"256_events_random", 256, benchOpts{}},
		{"4096_events_random", 4096, benchOpts{}},
		{"4096_events_identical", 4096, benchOpts{identicalPayload: true}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			events := makeBenchEvents(b, tc.n, tc.opts)
			frame, err := encodeBlockCompressed(events)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(frame)))
			b.ResetTimer()
			for b.Loop() {
				if _, err := DecodeBlockCompressed(frame); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
