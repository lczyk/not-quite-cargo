package driver

import (
	"reflect"
	"testing"

	"github.com/lczyk/assert"
)

func TestTranslate_PIE(t *testing.T) {
	got, err := Translate([]string{
		"foo.o",
		"-Wl,--as-needed",
		"-Wl,-z,relro,-z,now",
		"-nodefaultlibs",
		"-pie",
		"-lc",
		"-o", "/tmp/out",
	}, &Config{Triple: "aarch64-linux-gnu"})
	assert.NoError(t, err)

	want := []string{
		"-dynamic-linker", "/usr/lib/aarch64-linux-gnu/ld-linux-aarch64.so.1",
		"-L", "/usr/lib/aarch64-linux-gnu",
		"-L", "/usr/lib/gcc/aarch64-linux-gnu/14",
		// crt_pre (PIE)
		"/usr/lib/aarch64-linux-gnu/Scrt1.o",
		"/usr/lib/aarch64-linux-gnu/crti.o",
		"/usr/lib/gcc/aarch64-linux-gnu/14/crtbeginS.o",
		// user args, gcc-driver flags filtered out
		"foo.o",
		"--as-needed",
		"-z", "relro", "-z", "now",
		"-pie",
		"-lc",
		"-o", "/tmp/out",
		// crt_post
		"/usr/lib/gcc/aarch64-linux-gnu/14/crtendS.o",
		"/usr/lib/aarch64-linux-gnu/crtn.o",
	}
	assert.That(t, reflect.DeepEqual(got, want),
		"unexpected wild args\n got: %v\nwant: %v", got, want)
}

func TestTranslate_Shared(t *testing.T) {
	// -shared drops Scrt1 / crti / crtn (no main, no init/fini wrap)
	// and skips the PT_INTERP -dynamic-linker entry.
	got, err := Translate([]string{
		"foo.o",
		"-shared",
		"-o", "/tmp/libfoo.so",
	}, &Config{Triple: "x86_64-linux-gnu"})
	assert.NoError(t, err)

	want := []string{
		// no -dynamic-linker for -shared
		"-L", "/usr/lib/x86_64-linux-gnu",
		"-L", "/usr/lib/gcc/x86_64-linux-gnu/14",
		"/usr/lib/gcc/x86_64-linux-gnu/14/crtbeginS.o",
		"foo.o",
		"-shared",
		"-o", "/tmp/libfoo.so",
		"/usr/lib/gcc/x86_64-linux-gnu/14/crtendS.o",
	}
	assert.That(t, reflect.DeepEqual(got, want),
		"unexpected shared args\n got: %v\nwant: %v", got, want)
}

func TestTranslate_DropsLTOAndPluginFlags(t *testing.T) {
	got, err := Translate([]string{
		"foo.o",
		"-plugin",
		"-plugin-opt=-O3",
		"-fuse-ld=lld",
		"-flto=thin",
		"-pie",
	}, &Config{Triple: "aarch64-linux-gnu"})
	assert.NoError(t, err)

	// foo.o + -pie survive; LTO/plugin flags are gone.
	for _, want := range []string{"foo.o", "-pie"} {
		found := false
		for _, a := range got {
			if a == want {
				found = true
				break
			}
		}
		assert.That(t, found, "expected %q to survive translation", want)
	}
	for _, banned := range []string{"-plugin", "-plugin-opt=-O3", "-fuse-ld=lld", "-flto=thin"} {
		for _, a := range got {
			assert.That(t, a != banned, "expected %q to be stripped, found in output", banned)
		}
	}
}

func TestTranslate_DropsMArch(t *testing.T) {
	// rustc emits -m64 on x86_64 (and -m32 on i686) via the cc-driver
	// invocation; wild rejects -m 64 with "is not yet supported", so
	// the driver must strip both.
	got, err := Translate([]string{
		"foo.o",
		"-m64",
		"-m32",
		"-pie",
	}, &Config{Triple: "x86_64-linux-gnu"})
	assert.NoError(t, err)

	for _, banned := range []string{"-m64", "-m32"} {
		for _, a := range got {
			assert.That(t, a != banned, "expected %q to be stripped, found in output", banned)
		}
	}
}

func TestTranslate_UnsupportedArch(t *testing.T) {
	_, err := Translate([]string{"foo.o"}, &Config{Triple: "riscv64-linux-gnu"})
	assert.That(t, err != nil, "expected error for unsupported triple")
}
