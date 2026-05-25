#!/bin/sh
# Install zig + wire it up as the system c++ / cc compiler. Runs inside
# the Dockerfile build (or anywhere else that needs the same toolchain).
#
# Steps:
#   1. side-install curl + its runtime deps via apt-get download +
#      dpkg-deb -x (stripped base image has no _apt user, so a normal
#      apt-get install fails in postinst).
#   2. download the pinned zig tarball, extract to /opt/zig.
#   3. drop shim scripts at /usr/bin/{c++,cc} that exec
#      `zig c++` / `zig cc`, so cc_build / cpp_build pick them up via
#      default tool detection.
#
# Env:
#   ZIG_VERSION  pinned zig release (default 0.13.0)
set -eux

ZIG_VERSION="${ZIG_VERSION:-0.13.0}"

apt-get update
# Resolve full dep closure via apt's solver, download only (no postinst,
# which fails on the stripped base image -- no _apt user). Then unpack
# every .deb with dpkg-deb -x, which skips maintainer scripts entirely.
apt-get install -y --download-only --no-install-recommends curl ca-certificates xz-utils
for f in /var/cache/apt/archives/*.deb; do dpkg-deb -x "$f" /; done
rm -rf /var/cache/apt/archives/*.deb /var/lib/apt/lists/*

case "$(uname -m)" in
    aarch64) ZIG_ARCH=aarch64 ;;
    x86_64)  ZIG_ARCH=x86_64 ;;
    *) echo "unsupported arch $(uname -m)" >&2; exit 1 ;;
esac

curl -fsSL "https://ziglang.org/download/${ZIG_VERSION}/zig-linux-${ZIG_ARCH}-${ZIG_VERSION}.tar.xz" \
    -o /tmp/zig.tar.xz
mkdir -p /opt
tar -C /opt -xJf /tmp/zig.tar.xz
mv "/opt/zig-linux-${ZIG_ARCH}-${ZIG_VERSION}" /opt/zig
rm /tmp/zig.tar.xz

# Wipe any pre-existing entries first -- the base image ships
# /usr/bin/cc as an alternatives-managed symlink; writing through it
# would corrupt the chain and create a symlink loop on the next ln -sf.
rm -f /usr/bin/c++ /usr/bin/cc /usr/bin/g++ /usr/bin/gcc

# Shims also rewrite rust-style target triples (e.g.
# `aarch64-unknown-linux-gnu`) into zig's (`aarch64-linux-gnu`) -- zig
# doesn't accept the `unknown` vendor token cc-rs / cpp_build forward.
# POSIX positional rebuild, no sed / awk needed at run time.
cat > /usr/bin/c++ <<'SHIM'
#!/bin/sh
n=$#; i=0
while [ $i -lt $n ]; do
    a=$1; shift
    case "$a" in
        --target=*-unknown-linux-*)
            a="${a%%-unknown-linux-*}-linux-${a#*-unknown-linux-}" ;;
    esac
    set -- "$@" "$a"
    i=$((i+1))
done
exec /opt/zig/zig c++ "$@"
SHIM

cat > /usr/bin/cc <<'SHIM'
#!/bin/sh
n=$#; i=0
while [ $i -lt $n ]; do
    a=$1; shift
    case "$a" in
        --target=*-unknown-linux-*)
            a="${a%%-unknown-linux-*}-linux-${a#*-unknown-linux-}" ;;
    esac
    set -- "$@" "$a"
    i=$((i+1))
done
exec /opt/zig/zig cc "$@"
SHIM

cp /usr/bin/c++ /usr/bin/g++
cp /usr/bin/cc  /usr/bin/gcc
chmod +x /usr/bin/c++ /usr/bin/g++ /usr/bin/cc /usr/bin/gcc

/usr/bin/c++ --version
/usr/bin/cc --version
