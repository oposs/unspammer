// Package unspammer holds what the program and its release share.
package unspammer

import (
	_ "embed"
	"strings"
)

// The release bumps VERSION, the one place the version is kept; embedding it
// means the binary reports the version without a second copy in Go source.
//
//go:embed VERSION
var version string

// Version returns the version of this build.
func Version() string {
	return strings.TrimSpace(version)
}
