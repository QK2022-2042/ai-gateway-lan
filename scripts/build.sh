#!/usr/bin/env bash
# 构建离线发布包（无需 Docker，产物可直接拷贝到内网机器）。
# 用法：VERSION=1.0.0 ./scripts/build.sh
set -euo pipefail

cd "$(dirname "$0")/.."

GO="${GO:-go}"
VERSION="${VERSION:-1.0.0}"
OUT="${OUT:-dist}"
SKIP_TESTS="${SKIP_TESTS:-0}"

if ! command -v "$GO" >/dev/null 2>&1; then
  echo "找不到 go 命令。请安装 Go 1.27.4+，或用 GO=/path/to/go 指定。" >&2
  exit 1
fi

if [[ "$SKIP_TESTS" != "1" ]]; then
  echo "==> 运行测试"
  "$GO" test ./...
fi

rm -rf "$OUT"
mkdir -p "$OUT"

LDFLAGS="-s -w -X main.version=${VERSION}"

# Windows PowerShell 5.1 只有看到 UTF-8 BOM 才会按 UTF-8 读取 .ps1；
# 否则中文提示会变成乱码。
ensure_bom() {
  local file="$1"
  [[ -f "$file" ]] || return 0
  local magic
  magic="$(head -c 3 "$file" | od -An -t x1 | tr -d ' \n')"
  if [[ "$magic" != "efbbbf" ]]; then
    { printf '\xEF\xBB\xBF'; cat "$file"; } > "$file.tmp"
    mv "$file.tmp" "$file"
  fi
}

package_linux() {
  local arch="$1"
  local stage="$OUT/stage-linux-$arch"
  local name="ai-gateway_${VERSION}_linux_${arch}"
  mkdir -p "$stage"

  echo "==> 编译 linux/$arch"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" \
    "$GO" build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$stage/ai-gateway" .

  cp README.md "$stage/README.md"
  cp deploy/install.sh "$stage/install.sh"
  cp deploy/ai-gateway.service "$stage/ai-gateway.service"
  cp deploy/config.example.json "$stage/config.example.json"
  cp deploy/nginx.conf.example "$stage/nginx.conf.example"
  chmod 0755 "$stage/install.sh"

  (cd "$stage" && tar -czf "../${name}.tar.gz" .)
  rm -rf "$stage"
  echo "    -> $OUT/${name}.tar.gz"
}

package_windows() {
  local stage="$OUT/stage-windows-amd64"
  local name="ai-gateway_${VERSION}_windows_amd64"
  mkdir -p "$stage"

  echo "==> 编译 windows/amd64"
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
    "$GO" build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$stage/ai-gateway.exe" .

  cp README.md "$stage/README.md"
  cp WINDOWS.md "$stage/WINDOWS.md"
  cp deploy/config.windows.example.json "$stage/config.example.json"
  cp deploy/install-windows.ps1 "$stage/install-windows.ps1"
  cp deploy/uninstall-windows.ps1 "$stage/uninstall-windows.ps1"
  ensure_bom "$stage/install-windows.ps1"
  ensure_bom "$stage/uninstall-windows.ps1"
  # 批处理文件必须是 CRLF 换行，否则部分 Windows 版本解析异常。
  printf '@echo off\r\nchcp 65001 >nul\r\ncd /d "%%~dp0"\r\nif not exist data mkdir data\r\nai-gateway.exe -listen :8080 -data data -log data\\gateway.log\r\npause\r\n' \
    > "$stage/run-windows.bat"

  if command -v zip >/dev/null 2>&1; then
    (cd "$stage" && zip -q -r "../${name}.zip" .)
    echo "    -> $OUT/${name}.zip"
  else
    echo "    ! 未找到 zip 命令，保留未压缩目录 $stage" >&2
  fi
  rm -rf "$stage"
}

package_linux amd64
package_linux arm64
package_windows

# 可选的 macOS 本地试用二进制（不打进服务器安装包）
if [[ "${BUILD_DARWIN:-0}" == "1" ]]; then
  echo "==> 编译 darwin/arm64（本地试用）"
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
    "$GO" build -trimpath -buildvcs=false -ldflags "$LDFLAGS" \
    -o "$OUT/ai-gateway_${VERSION}_darwin_arm64" .
fi

echo "==> 生成 SHA256SUMS"
(
  cd "$OUT"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz ./*.zip > SHA256SUMS 2>/dev/null || true
  else
    shasum -a 256 ./*.tar.gz ./*.zip > SHA256SUMS 2>/dev/null || true
  fi
)

echo
echo "构建完成，产物位于 $OUT/："
ls -lh "$OUT" | sed 's/^/  /'
