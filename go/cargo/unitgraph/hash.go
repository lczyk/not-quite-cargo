// Package unitgraph lowers a cargo unit-graph (`cargo build -Z
// unstable-options --unit-graph`) into the build-plan shape that the
// existing cargo.Run path already consumes.
//
// The package is experimental: cargo's --build-plan was removed in
// 1.93.0 and unit-graph is the closest surviving plan-export. unit-graph
// only carries the unit DAG + per-unit metadata, so this package
// reimplements the rustc-command-building logic cargo used to expose
// directly.
package unitgraph

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// HashInputs captures everything that distinguishes one compilation unit
// from another for the purpose of file-naming and `-C metadata=`.
//
// These fields are taken straight from a unit-graph entry; the hash is
// stable as long as the inputs are stable.
type HashInputs struct {
	PkgID       string
	TargetName  string
	Mode        string
	ProfileName string
	Features    []string
	Platform    string // empty string means host
	Host        string
}

// MetadataHash returns a 16-hex digest that identifies the unit.
//
// Used as the value of `-C metadata=`, `-C extra-filename=` and in the
// output file name so dependents can locate the produced rlib via
// `--extern <name>=<path>`.
//
// The hash is internally consistent within a single generated build plan;
// it does NOT match cargo's own fingerprint hash. The runner only ever
// sees our paths and cargo isn't there to disagree, so cargo-compat would
// buy nothing.
func MetadataHash(in HashInputs) string {
	h := sha256.New()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	write(in.PkgID)
	write(in.TargetName)
	write(in.Mode)
	write(in.ProfileName)
	features := append([]string(nil), in.Features...)
	sort.Strings(features)
	write(strings.Join(features, ","))
	write(in.Platform)
	write(in.Host)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
