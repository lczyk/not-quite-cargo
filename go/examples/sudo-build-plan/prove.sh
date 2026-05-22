#!/usr/bin/env bash
# Prove stage of the demo. Runs inside ubuntu:24.04 (no rust toolchain,
# network off). Copies the built sudo into a stock distro env, sets the
# setuid bit, creates minimal /etc/sudoers + /etc/pam.d/sudo and then runs
# `sudo whoami`
set -ex

BIN=$(find /sudo-target -name "sudo-*" -type f -executable | head -n1)
[ -n "$BIN" ] || { echo "no sudo binary under /sudo-target" >&2; exit 1; }

install -m 4755 -o root -g root "$BIN" /usr/local/bin/sudo

echo "root ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers
chmod 440 /etc/sudoers

{
    echo "auth sufficient pam_permit.so"
    echo "account sufficient pam_permit.so"
    echo "session sufficient pam_permit.so"
} > /etc/pam.d/sudo

ldd /usr/local/bin/sudo
sudo whoami | grep -F root
