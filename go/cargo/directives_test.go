package cargo

import (
	"reflect"
	"testing"
)

type captureLogger struct{ warnings []string }

func (c *captureLogger) Infof(string, ...any)             {}
func (c *captureLogger) Warnf(format string, args ...any) { c.warnings = append(c.warnings, format) }

func TestParseBuildScriptOutput(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantFlags []string
		wantEnv   map[string]string
		wantWarns int
	}{
		{
			name:      "rustc-cfg",
			input:     "cargo:rustc-cfg=feature=\"x\"",
			wantFlags: []string{"--cfg", "feature=\"x\""},
			wantEnv:   map[string]string{},
		},
		{
			name:      "rustc-check-cfg",
			input:     "cargo:rustc-check-cfg=cfg(foo)",
			wantFlags: []string{"--check-cfg", "cfg(foo)"},
			wantEnv:   map[string]string{},
		},
		{
			name:      "rustc-link-lib",
			input:     "cargo:rustc-link-lib=ssl",
			wantFlags: []string{"-l", "ssl"},
			wantEnv:   map[string]string{},
		},
		{
			name:      "rustc-link-arg",
			input:     "cargo:rustc-link-arg=-Wl,--gc-sections",
			wantFlags: []string{"-C", "link-arg=-Wl,--gc-sections"},
			wantEnv:   map[string]string{},
		},
		{
			name:      "rustc-link-search bare",
			input:     "cargo:rustc-link-search=/opt/lib",
			wantFlags: []string{"-L", "/opt/lib"},
			wantEnv:   map[string]string{},
		},
		{
			name:      "rustc-link-search kind",
			input:     "cargo:rustc-link-search=native=/opt/lib",
			wantFlags: []string{"-L", "/opt/lib"},
			wantEnv:   map[string]string{},
		},
		{
			name:      "rustc-env",
			input:     "cargo:rustc-env=FOO=bar",
			wantFlags: []string{},
			wantEnv:   map[string]string{"FOO": "bar"},
		},
		{
			name:      "ignored keys",
			input:     "cargo:rerun-if-changed=build.rs\ncargo:rerun-if-env-changed=PATH",
			wantFlags: []string{},
			wantEnv:   map[string]string{},
		},
		{
			name:      "skip non-cargo prefix",
			input:     "warning: something\nplain text\ncargo:rustc-cfg=ok",
			wantFlags: []string{"--cfg", "ok"},
			wantEnv:   map[string]string{},
		},
		{
			name:      "malformed without equals",
			input:     "cargo:rustc-cfg-no-equals",
			wantFlags: []string{},
			wantEnv:   map[string]string{},
			wantWarns: 1,
		},
		{
			name:      "unknown key warns",
			input:     "cargo:something-new=value",
			wantFlags: []string{},
			wantEnv:   map[string]string{},
			wantWarns: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &captureLogger{}
			got := ParseBuildScriptOutput(tc.input, log)
			if !reflect.DeepEqual(got.RustcFlags, tc.wantFlags) {
				t.Errorf("flags mismatch\n got: %v\nwant: %v", got.RustcFlags, tc.wantFlags)
			}
			if !reflect.DeepEqual(got.EnvVars, tc.wantEnv) {
				t.Errorf("env mismatch\n got: %v\nwant: %v", got.EnvVars, tc.wantEnv)
			}
			if len(log.warnings) != tc.wantWarns {
				t.Errorf("warnings: got %d, want %d (%v)", len(log.warnings), tc.wantWarns, log.warnings)
			}
		})
	}
}
