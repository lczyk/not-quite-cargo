package unitgraph

import (
	"testing"

	"github.com/lczyk/assert"
)

func baseInputs() HashInputs {
	return HashInputs{
		PkgID:       "path+file:///proj/foo#0.1.0",
		TargetName:  "foo",
		Mode:        "build",
		ProfileName: "dev",
		Features:    []string{"default", "json"},
		Platform:    "",
		Host:        "aarch64-apple-darwin",
	}
}

func TestMetadataHash_Stable(t *testing.T) {
	h1 := MetadataHash(baseInputs())
	h2 := MetadataHash(baseInputs())
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 16)
}

func TestMetadataHash_FeatureOrderIndependent(t *testing.T) {
	a := baseInputs()
	b := baseInputs()
	b.Features = []string{"json", "default"}
	assert.Equal(t, MetadataHash(a), MetadataHash(b))
}

func TestMetadataHash_DifferentInputsDiffer(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*HashInputs)
	}{
		{"pkg id", func(in *HashInputs) { in.PkgID += "x" }},
		{"target name", func(in *HashInputs) { in.TargetName = "bar" }},
		{"mode", func(in *HashInputs) { in.Mode = "test" }},
		{"profile", func(in *HashInputs) { in.ProfileName = "release" }},
		{"add feature", func(in *HashInputs) { in.Features = append(in.Features, "extra") }},
		{"platform", func(in *HashInputs) { in.Platform = "x86_64-unknown-linux-gnu" }},
		{"host", func(in *HashInputs) { in.Host = "x86_64-unknown-linux-gnu" }},
	}
	base := MetadataHash(baseInputs())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			tc.mutate(&in)
			assert.NotEqual(t, MetadataHash(in), base)
		})
	}
}

func TestMetadataHash_NoCrossFieldCollision(t *testing.T) {
	// Field separation: moving a string from one field to another must
	// change the hash. Catches naive concat-then-hash impls that would
	// collide when, e.g., target_name="foo-bar" and target_name="foo"
	// with mode="bar".
	a := baseInputs()
	a.TargetName = "foo"
	a.Mode = "bar"

	b := baseInputs()
	b.TargetName = "foo-bar"
	b.Mode = ""

	assert.NotEqual(t, MetadataHash(a), MetadataHash(b))
}
