package goversion

// Version is the Go 1.x version which is currently
// in development and will eventually get released.
//
// It should be updated at the start of each development cycle to be
// the version of the next Go 1.x release. See go.dev/issue/40705.
const Version = 28

// GonVersion is the Gon release implemented by this toolchain.
// It is independent of the upstream development provenance in Version.
const GonVersion = "2.27.0"
