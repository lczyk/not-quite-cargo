# alpine:3.21 (cargo 1.83 + rustc 1.83, native musl). Single Dockerfile,
# two image tags via REMOVE_CARGO -- plan keeps cargo, demo deletes it.
#
# Alpine is musl-native: the default rustc target is the local
# <arch>-unknown-linux-musl, so we don't need rustup target add (the
# rustup wrapper isn't shipped at all in this image). cargo + rust come
# in as regular apk packages, picking up musl-dev + gcc transitively
# for the C linker.
FROM alpine:3.21

RUN apk add --no-cache cargo rust

ARG REMOVE_CARGO=1
RUN if [ "$REMOVE_CARGO" = "1" ]; then \
        rm -f /usr/bin/cargo; \
    fi
