# gofs

## Abstract

gofs is an implementation of a distributed file system in the design
tradition of the Google File System (GFS), as described by Ghemawat,
Gobioff, and Leung (2003). The system targets feature parity with the
original GFS design (v1) and is developed incrementally, with each
component — master, chunkserver, client library, and the protocol
binding them together — designed, implemented, and reviewed in
isolation before integration. The project is intended both as a
reference for study of distributed storage system design and as a
system suitable for direct use.

## Status

This repository currently contains only the project scaffold. No
system components have been implemented yet. Design documents for
individual components will be added to this repository as they are
produced, under `docs/`.

## Building and Testing

```
go build ./...
go test ./...
```

## License

No license has been assigned to this repository yet.
