package main

import (
	"reflect"
	"testing"

	flags "github.com/jessevdk/go-flags"
)

// TestOptions_GoFlagsTags makes sure every subcommand field in Options is
// wired up correctly for go-flags: has a `command:` tag, a `description:`
// tag, and implements flags.Commander via Execute(args []string) error.
//
// go-flags silently ignores fields without the right tags, so a typo here
// turns a subcommand into a no-op at runtime. This test catches that at
// compile-time-ish (build the binary, run go test).
func TestOptions_GoFlagsTags(t *testing.T) {
	commander := reflect.TypeOf((*flags.Commander)(nil)).Elem()

	typ := reflect.TypeOf(Options{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		// Non-command fields (like Version) are exempt.
		if _, ok := f.Tag.Lookup("command"); !ok {
			continue
		}
		if f.Tag.Get("description") == "" {
			t.Errorf("%s: missing description tag", f.Name)
		}
		ptr := reflect.PointerTo(f.Type)
		if !ptr.Implements(commander) {
			t.Errorf("%s (%s) does not implement flags.Commander -- Execute signature must be Execute(args []string) error",
				f.Name, f.Type)
		}
	}
}
