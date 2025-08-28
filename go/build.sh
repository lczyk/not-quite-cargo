#!/bin/bash
rm -rf ./not-quite-cargo || exit 1

go build -ldflags="-s -w" -o not-quite-cargo src/cmd/not-quite-cargo/main.go || exit 1

if command -v upx &> /dev/null; then
    upx --best not-quite-cargo
fi

echo "Built ./not-quite-cargo binary"
echo "Size: $(du -h ./not-quite-cargo | cut -f1)"