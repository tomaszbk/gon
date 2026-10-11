package gover

import (
	"internal/buildcfg"
	"internal/goversion"
	"strconv"
)

// TestVersion is initialized in the go command test binary
// to be $TESTGO_VERSION, to allow tests to override the
// go command's idea of its own version as returned by Local.
var TestVersion string

// Local returns the local Go version, the one implemented by this go command.
func Local() string {
	v, _ := local()
	return v
}

// LocalToolchain returns the local toolchain name, the one implemented by this go command.
func LocalToolchain() string {
	_, t := local()
	return t
}

func local() (goVers, toolVers string) {
	toolVers = buildcfg.Version
	if TestVersion != "" {
		toolVers = TestVersion
	}
	goVers = FromToolchain(toolVers)
	if goVers == "" {
		// Development branch. Use "Dev" version with just 1.N, no rc1 or .0 suffix.
		goVers = "1." + strconv.Itoa(goversion.Version)
		toolVers = "go" + goVers
	}
	return goVers, toolVers
}

// GonVersion is the Gon release implemented by this command.
const GonVersion = goversion.GonVersion

// Allows reports whether this toolchain satisfies a module's go directive.
func Allows(v string) bool {
	if IsGon(v) {
		return Compare(v, GonVersion) <= 0
	}
	return Compare(v, Local()) <= 0
}
