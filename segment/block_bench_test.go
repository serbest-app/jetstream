package segment

import (
	"bytes"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/bluesky-social/jetstream/segment/internal/format"
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

// BenchmarkAppend measures the per-event amortized cost of Append.
// We isolate Append cost from Flush by resetting the pending buffer
// in-place when it reaches cap, rather than calling Flush (which
// would mix in zstd + write + fsync time). This lets the benchmark
// reflect the steady-state Append hot path across arbitrary b.N
// without bumping into the decoder cap on MaxEventsPerBlock.
func BenchmarkAppend(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "seg.jss")
	w, err := New(Config{Path: path, MaxEventsPerBlock: DefaultMaxEventsPerBlock})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	template := Event{
		Seq: 0, Kind: KindCreate,
		DID:        "did:plc:abcdefghijklmnopqrstuvwx",
		Collection: "app.bsky.feed.post",
		Rkey:       "3l3qo2vuowo2b",
		Rev:        "3l3qo2vutsw2b",
		Payload:    bytes.Repeat([]byte{0xAB}, 512),
	}

	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		if w.pending.Len() >= w.pending.Cap() {
			format.BuilderReset(&w.pending)
		}
		template.Seq = uint64(i)
		if _, err := w.Append(template); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSteadyFlush measures the production hot path: one
// already-open writer flushing block after block. This is what the
// firehose ingester actually does; BenchmarkFlushToTmpfs below
// includes file-open/teardown and gives a less useful picture of
// per-block cost.
func BenchmarkSteadyFlush(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "seg.jss")
	w, err := New(Config{Path: path})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	events := makeBenchEvents(b, DefaultMaxEventsPerBlock, benchOpts{})
	b.ReportAllocs()

	for b.Loop() {
		for j := range events {
			if _, err := w.Append(events[j]); err != nil {
				b.Fatal(err)
			}
		}
		if err := w.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFlushToTmpfs measures one full Append-batch + Flush
// cycle. On Linux CI runners t.TempDir() is tmpfs, so this measures
// CPU + zstd time, not real-disk fsync latency. Production fsync
// latency dominates and isn't what this benchmark is for.
func BenchmarkFlushToTmpfs(b *testing.B) {
	events := makeBenchEvents(b, 4096, benchOpts{})

	b.ReportAllocs()

	for b.Loop() {
		dir := b.TempDir()
		path := filepath.Join(dir, "seg.jss")
		w, err := New(Config{Path: path})
		if err != nil {
			b.Fatal(err)
		}

		for j := range events {
			if _, err := w.Append(events[j]); err != nil {
				b.Fatal(err)
			}
		}
		if err := w.Flush(); err != nil {
			b.Fatal(err)
		}
		if err := w.Close(); err != nil {
			b.Fatal(err)
		}
	}
}
