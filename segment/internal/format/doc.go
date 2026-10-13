// Package format is the segment block format: events, the block wire format,
// BlockBuilder, the sealed header, and the block index. It does no file I/O.
//
// Package segment builds its file storage on top of it, and package
// segment/format exposes it to clients that only decode blocks. Both
// re-export the same public names as aliases. The other exported names here
// (encoders, header and index codecs, limits, and the Builder* functions)
// exist only for package segment; being internal keeps them out of the
// public API. The Builder* functions are functions rather than BlockBuilder
// methods because an exported method would become public through the
// aliases.
package format
