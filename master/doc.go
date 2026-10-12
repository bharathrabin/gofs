// Package master implements the namespace and chunk metadata core of
// the gofs master server, in the design tradition of the Google File
// System (Ghemawat, Gobioff, and Leung, 2003).
//
// This package is deliberately self-contained: it has no knowledge of
// RPC, chunkservers, replica placement, leases, or persistence. It
// provides two independent pieces of state that later components
// build on:
//
//   - Namespace, the master's in-memory mapping from full pathnames to
//     file and directory metadata, with the paper's prefix-locking
//     discipline (§4.1) for concurrent namespace mutations.
//   - ChunkAllocator, the source of globally unique, monotonically
//     increasing chunk handles and their version numbers.
package master
