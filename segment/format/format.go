// Package format is the segment block format without the file storage:
// events, building and decoding blocks, and the sealed segment header. It is
// what getBlock serves, so clients that decode archive blocks need it.
//
// Every name here is also exported by package segment, under the same name
// and as the same type, so values pass between the two unchanged. Import
// format instead of segment when you only decode blocks you fetched some
// other way: it does not link segment's file I/O or the pebble, cockroachdb
// and sentry-go packages that come with it.
package format

import (
	"io"

	impl "github.com/bluesky-social/jetstream/segment/internal/format"
)

// ValidateEvent checks that ev's fields fit the on-disk column widths
// and that Kind is in range. It performs no I/O and never panics.
//
// External-ingest callers use this before Append so malformed
// upstream data can be counted and skipped without touching the
// durable writer. Append still validates internally because segment
// is the final invariant boundary for on-disk data.
func ValidateEvent(ev Event) error {
	return impl.ValidateEvent(ev)
}

// BlockFrameSection returns a reader for the raw, stored zstd frame for block
// idx. It reads and validates only that block's 52-byte footer index entry;
// the returned section excludes the 8-byte length prefix and does not allocate
// space proportional to the compressed frame.
//
// r is the fd for the sealed segment file; hdr is its fixed header as returned
// by ReadSealedHeader. Returns ErrBlockOutOfRange when idx is out of range.
// All offsets are validated against hdr.FooterOffset before constructing the
// section, so a corrupt/hostile block-index entry cannot drive an out-of-bounds
// read.
func BlockFrameSection(r io.ReaderAt, hdr Header, idx int) (*io.SectionReader, error) {
	return impl.BlockFrameSection(r, hdr, idx)
}

// ReadBlockFrame reads the raw, stored zstd frame for block idx using only the
// already-read fixed header — no footer/bloom/collection parsing and no
// decompression. The returned bytes exclude the 8-byte length prefix, i.e. they
// are exactly the [block_len]byte frame the writer appended.
//
// r is the fd for the sealed segment file; hdr is its fixed header as returned
// by ReadSealedHeader. Returns ErrBlockOutOfRange when idx is out of range.
// All offsets are validated against hdr.FooterOffset before any read keyed off
// them, so a corrupt/hostile block-index entry cannot drive an out-of-bounds or
// oversized allocation/read.
func ReadBlockFrame(r io.ReaderAt, hdr Header, idx int) ([]byte, error) {
	return impl.ReadBlockFrame(r, hdr, idx)
}

// DecodeBlockFrame decompresses and decodes a single raw block frame into its
// events. frame is exactly the zstd frame returned by ReadBlockFrame or by the
// getBlock XRPC endpoint (no 8-byte length prefix). This is the standard block
// decoder remote clients use on the bytes named by a snapshot plan.
//
// The decoder bounds decompressed size to guard against zstd bombs and
// validates every column length, so a corrupt or hostile frame yields an error
// rather than a panic or oversized allocation.
//
// Buffer-aliasing contract: the returned events alias an internal decompressed
// buffer for their string and payload columns. The buffer is private to this
// call (not shared across frames), but callers that retain fields beyond the
// events' lifetime should clone them.
func DecodeBlockFrame(frame []byte) ([]Event, error) {
	return impl.DecodeBlockFrame(frame)
}

// BlockBuilder accumulates events into one in-memory columnar block
// (docs/README.md §3.2) and encodes it into a compressed block frame. It
// performs no I/O: Writer uses one to build the blocks it appends to the
// active file, and callers that persist blocks somewhere other than a
// local file use one directly.
//
// A BlockBuilder is not safe for concurrent use.
type BlockBuilder = impl.BlockBuilder

// NewBlockBuilder returns an empty builder that reports a block full at
// maxEventsPerBlock events. Zero selects DefaultMaxEventsPerBlock. The
// limit must not exceed what the block decoder accepts, so every block
// a builder produces can be read back.
func NewBlockBuilder(maxEventsPerBlock int) (*BlockBuilder, error) {
	return impl.NewBlockBuilder(maxEventsPerBlock)
}

// ErrInvalidConfig is returned by New when Config has unusable values.
var ErrInvalidConfig = impl.ErrInvalidConfig

// ErrCorruptSegment is returned by New when an existing segment
// file is smaller than the 256-byte reserved header region.
var ErrCorruptSegment = impl.ErrCorruptSegment

// ErrActiveSegment is returned by Open when the file at the given
// path has its fixed-header checksum field still zero — i.e. the
// segment is active (not yet sealed). Callers that intentionally
// scan a directory of mixed sealed and active segments (manifest
// startup) can errors.Is against this to skip the active files
// without conflating them with genuine corruption.
var ErrActiveSegment = impl.ErrActiveSegment

// ErrSegmentSealed will be returned by New when the file carries
// a sealed checksum trailer. The seal/unseal mechanism lands in a
// later slice; the sentinel is reserved here so callers can already
// switch on it.
var ErrSegmentSealed = impl.ErrSegmentSealed

// ErrFieldTooLong is returned by Append when a string or Payload
// field exceeds its on-disk column width.
var ErrFieldTooLong = impl.ErrFieldTooLong

// ErrInvalidKind is returned by Append when ev.Kind is outside [1, 6].
var ErrInvalidKind = impl.ErrInvalidKind

// ErrBufferFull is returned by Append when the pending block has
// already reached MaxEventsPerBlock and the caller did not Flush
// in response to an earlier "full" signal.
var ErrBufferFull = impl.ErrBufferFull

// ErrClosed is returned by Append, Flush, and (re-)Close after Close.
var ErrClosed = impl.ErrClosed

// ErrChecksumMismatch is returned by Reader.Open when the file's
// xxh3 checksum disagrees with the value in its fixed header. The
// likely contributing factors are bit rot, a partial CDN download,
// or replication corruption.
var ErrChecksumMismatch = impl.ErrChecksumMismatch

// ErrInvalidFooter is returned by Reader.Open when the variable-
// length footer fails structural validation: a section length
// would overrun the file, internal pointers don't agree, or a
// length-prefixed sub-region is truncated.
var ErrInvalidFooter = impl.ErrInvalidFooter

// ErrInvalidBlockIndex is returned by Reader.Open when a block
// index entry's offset/size pair doesn't fit within the file.
var ErrInvalidBlockIndex = impl.ErrInvalidBlockIndex

// ErrBlockOutOfRange is returned by Reader.DecodeBlock,
// Reader.BlockBloom, and Reader.BlockCollections when the requested
// block index is past BlockCount.
var ErrBlockOutOfRange = impl.ErrBlockOutOfRange

// Kind discriminates which firehose event type a row represents.
// Values are the on-disk wire format from docs/README.md §3.2.
type Kind = impl.Kind

const KindCreate = impl.KindCreate

const KindUpdate = impl.KindUpdate

const KindDelete = impl.KindDelete

const KindIdentity = impl.KindIdentity

const KindAccount = impl.KindAccount

const KindSync = impl.KindSync

const KindCreateResync = impl.KindCreateResync

// Event is one row inside a segment block.
//
// Variable-length fields are constrained to fit in their on-disk
// length columns:
//
//	DID:        up to 65535 bytes (uint16 column)
//	Collection: up to 255   bytes (uint8  column)
//	Rkey:       up to 255   bytes (uint8  column)
//	Rev:        up to 255   bytes (uint8  column)
//	Payload:    up to math.MaxUint32 bytes
//
// WitnessedAt and IndexedAt are unix microseconds. IndexedAt == 0
// means "no operator-supplied timestamp" (docs/README.md §3.2).
//
// For non-commit kinds (Identity, Account, Sync), Collection, Rkey,
// Rev, and Payload are typically empty / nil. The encoder accepts
// any combination; emptiness is not enforced as a per-Kind invariant
// at this layer.
type Event = impl.Event

// BlockInfo is one entry of the block index (docs/README.md §3.1.2). It
// describes a block's location and bounds within a sealed segment
// file.
type BlockInfo = impl.BlockInfo

// The default number of events to store in a single segment block. This
// value is just the default, and is configurable by the user.
const DefaultMaxEventsPerBlock = impl.DefaultMaxEventsPerBlock

// ReservedHeaderBytes is the size of the fixed header at the start
// of every segment file (docs/README.md §3.1.2). The first len(SegmentMagic)
// bytes are the magic; the remainder is zero-filled placeholder until
// Seal populates the finalized header. ReservedHeaderBytes is also
// the byte offset at which the framed-block region begins.
//
// Exported so callers that own the active-segment lifecycle (e.g.
// internal/ingest) can compute byte offsets without duplicating the
// constant.
const ReservedHeaderBytes = impl.ReservedHeaderBytes

// Header is the parsed form of the 256-byte fixed header at offset 0
// of every sealed segment file. See docs/README.md §3.1.2 for the wire
// layout, and the spec §5.1 for byte-offset details.
//
// All offsets are absolute file offsets.
type Header = impl.Header

// ReadSealedHeader reads and parses the 256-byte fixed header from r
// (an open segment file), with full magic/version validation. Returns
// an ErrActiveSegment-wrapped error when the checksum field is zero
// (active file) and an ErrCorruptSegment-wrapped error on bad magic.
// Cheap (one pread); used by serving paths that derive download
// validators from the file descriptor they actually opened.
func ReadSealedHeader(r io.ReaderAt) (Header, error) {
	return impl.ReadSealedHeader(r)
}

// WarmEncoder initializes the global zstd encoder's lazy worker channel
// outside any testing/synctest bubble. Otherwise the first in-bubble
// EncodeAll binds the channel to that bubble and later use outside it aborts
// the process.
//
// Call from TestMain before entering a bubble. Repeated calls are cheap and
// idempotent. The decoder needs no warmup because NewReader creates its
// channel at package initialization.
func WarmEncoder() {
	impl.WarmEncoder()
}
