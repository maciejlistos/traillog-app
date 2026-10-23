// Package gpx parses GPX 1.1 track files and calculates route summaries.
//
// The package is independant from TrailLog's persistance and HTTP layers.
// It accepts io.Reader values so callers can parse uploads, files tests,
// or other streams without first loading the complete GPX document into
// memory.
//
// TrailLog currently imports one GPX track per document. A track may contain
// multiple segments.
package gpx
