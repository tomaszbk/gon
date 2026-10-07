#!/bin/bash
set -euo pipefail
cd /opt/gon
rm -rf tools
cp -R /source/tools tools
while IFS= read -r path; do rm -f "$path"; done < /evidence/deleted-files.txt
while IFS= read -r path; do mkdir -p "$(dirname "$path")"; cp "/source/$path" "$path"; done < /evidence/source-files.txt
cp -R /source/test/. /opt/gon/test
export GOENV=off GOTOOLCHAIN=local GOFLAGS=-p=2 GOMAXPROCS=2
./bin/go install cmd/compile cmd/go cmd/gofmt cmd/vet cmd/cgo cmd/cover
python3 misc/gon/build.py
apt-get update >/evidence/apt-update.log
apt-get install -y --no-install-recommends qemu-user >/evidence/apt-qemu.log
mkdir -p /tmp/gon-exec
printf '#!/bin/sh\nexec qemu-x86_64 "$@"\n' > /tmp/gon-exec/go_linux_amd64_exec
printf '#!/bin/sh\nexec qemu-riscv64 "$@"\n' > /tmp/gon-exec/go_linux_riscv64_exec
chmod +x /tmp/gon-exec/*
export PATH=/tmp/gon-exec:$PATH GON_BASELINE_GO=/usr/local/go/bin/go
./gon/bin/gon version > /evidence/gon-version.txt
$GON_BASELINE_GO version > /evidence/baseline-version.txt
qemu-x86_64 --version > /evidence/qemu-amd64-version.txt
qemu-riscv64 --version > /evidence/qemu-riscv64-version.txt
for arch in arm64 amd64 riscv64; do
    misc/gon/cross_pair.sh linux "$arch" > "/evidence/linux-$arch.log" 2>&1
    echo "PASS: linux/$arch (arm64 native; amd64/riscv64 QEMU)"
done
