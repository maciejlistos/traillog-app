// Package buildinfo contains metadata injected into the TrailLog binary at build time.
package buildinfo

// Version in TrailLog's release or development version.
//
// Production builds replace this value using Go linker -X flags.
var Version = "dev"

// Revision is the source-control revision used to build TrailLog.
var Revision = "unknown"

// BuildTime is the UTC timestamp at which TrailLog was build.
var BuildTime = "unknown"

// Info describes on TrailLog application build.
type Info struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	BuildTime string `json:"build_time"`
}

// Current returns metadata describing the running TrailLog binary.
func Current() Info {
	return Info{
		Version:   Version,
		Revision:  Revision,
		BuildTime: BuildTime,
	}
}
