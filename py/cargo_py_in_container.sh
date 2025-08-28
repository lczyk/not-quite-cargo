#!/bin/bash
# spellchecker: words PYTHONPATH userns

DOCKER="docker"
# DOCKER="podman"

# Set to 1 to get a shell inside the container instead of running the script
SHELL=0

# IMAGE="oci-archive:rust-rock_1.84_arm64.rock"
# IMAGE="rust-rock:1.84"
IMAGE="rust:latest"

function main() {
    local cargo_home=${CARGO_HOME:-$HOME/.cargo}
    # echo "CARGO_HOME: '$cargo_home'"

    rm -rf ./target || exit 1

    local it=""
    local entrypoint='/work/python'
    local args=("-S" "not-quite-cargo.py" "run" "build_plan.json")
    
    if [[ $SHELL -eq 1 ]]; then
        it="-it"
        entrypoint="/usr/bin/sh"
        args=()
    elif [[ $SHELL -eq 2 ]]; then
        it="-it"
        entrypoint="/work/python"
        args=("-S")
    fi

    local host_arch="$(uname -m)"
    [[ "$host_arch" == "aarch64" ]] && host_arch="arm64"
    [[ "$host_arch" == "x86_64" ]] && host_arch="amd64"

    local common=(
        "$it"
        --rm
        --platform=linux/"$host_arch"
        --network=none
        -e CARGO_HOME=/cargo-home
        -e PYTHONPATH=/work/python-lib
    )
    
    if [[ "$DOCKER" == "podman" ]]; then

        # get the uid/gid of the user outside the container
        local uid=$(podman image inspect "$IMAGE" | jq -r '.[0].User')
        local gid=$(podman image inspect "$IMAGE" | jq -r '.[0].Config.Labels["io.containers.user-groups"]' | cut -d: -f2 | cut -d, -f1)
        [[ -z "$gid" || "$gid" == "null" ]] && gid=$uid

        podman run \
        ${common[@]} \
        --volume "$PWD":/work:z \
        --volume "$cargo_home":/cargo-home:z \
        --workdir /work \
        --userns=keep-id:uid="$uid",gid="$uid" \
        --entrypoint "$entrypoint" \
        "$IMAGE" \
        ${args[@]}
    else
        docker run \
        ${common[@]} \
        --volume "$PWD":/work \
        --volume "$cargo_home":/cargo-home \
        --workdir /work \
        --user "$(id -u):$(id -g)" \
        --entrypoint "$entrypoint" \
        "$IMAGE" \
        ${args[@]}
    fi
}

main "$@"