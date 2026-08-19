#!/usr/bin/env bash
# 交叉编译三平台静态二进制并打包，供 GitHub Release 使用。
# 用法: VERSION=v0.1.1 ./scripts/release.sh   （不传则用 git describe）
set -euo pipefail

VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"
LDFLAGS="-X github.com/caiqianzhang/gitcn/internal/cli.Version=${VERSION}"
OUTDIR="dist"

rm -rf "${OUTDIR}"
mkdir -p "${OUTDIR}"

build() {
  local os="$1" arch="$2" exe=""
  local name="gitcn-${VERSION}-${os}-${arch}"
  if [ "${os}" = "windows" ]; then exe=".exe"; fi
  GOOS="${os}" GOARCH="${arch}" CGO_ENABLED=0 \
    go build -trimpath -ldflags "${LDFLAGS}" -o "${OUTDIR}/gitcn${exe}" ./cmd/gitcn
  if [ "${os}" = "windows" ]; then
    (cd "${OUTDIR}" && zip -q "${name}.zip" "gitcn.exe" && rm -f "gitcn.exe")
  else
    (cd "${OUTDIR}" && tar -czf "${name}.tar.gz" "gitcn" && rm -f "gitcn")
  fi
  echo "built ${OUTDIR}/${name}"
}

build linux amd64
build linux arm64
build darwin amd64
build darwin arm64
build windows amd64

# 校验和，供用户核对下载完整性
(cd "${OUTDIR}" && shasum -a 256 *.tar.gz *.zip > SHA256SUMS)

ls -lh "${OUTDIR}"
