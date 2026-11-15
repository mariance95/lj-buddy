#!/usr/bin/env bash
# Compila LJ Buddy ArtNet Translator y genera los entregables en dist/:
#   LJ-Buddy-ArtNet-Translator-Setup-<versión>.exe      instalador de Windows (requiere makensis)
#   LJ-Buddy-ArtNet-Translator-<versión>-windows.zip    versión portable de Windows
#   LJ-Buddy-ArtNet-Translator-<versión>-linux-amd64.tar.gz
set -euo pipefail
cd "$(dirname "$0")"
VERSION="${VERSION:-1.0.0}"
LDFLAGS="-s -w -X main.version=${VERSION}"
export CGO_ENABLED=0

go vet ./...
go test ./...

rm -rf dist && mkdir -p dist/windows dist/linux

# Icono, manifiesto y datos de versión dentro del .exe.
go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 \
  --icon assets/icon.png --manifest gui \
  --product-name "LJ Buddy ArtNet Translator" --file-description "LJ Buddy ArtNet Translator" \
  --product-version "${VERSION}" --file-version "${VERSION}"

GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "${LDFLAGS} -H windowsgui" -o dist/windows/lj-buddy-artnet-translator.exe .
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "${LDFLAGS}" -o dist/linux/lj-buddy-artnet-translator .
rm -f rsrc_windows_*.syso

(cd dist/windows && zip -q "../LJ-Buddy-ArtNet-Translator-${VERSION}-windows.zip" lj-buddy-artnet-translator.exe && zip -qj "../LJ-Buddy-ArtNet-Translator-${VERSION}-windows.zip" ../../README.md ../../../LICENSE)
tar -czf "dist/LJ-Buddy-ArtNet-Translator-${VERSION}-linux-amd64.tar.gz" -C dist/linux lj-buddy-artnet-translator -C ../.. README.md -C .. LICENSE

if command -v makensis >/dev/null; then
  FTDI=()
  if [ -f installer/drivers/CDM-Setup.exe ]; then
    FTDI=(-DWITH_FTDI)
    echo "Incluyendo el driver FTDI en el instalador."
  fi
  makensis -V2 -DVERSION="${VERSION}" "${FTDI[@]}" installer/artnet-translator.nsi
else
  echo "makensis no está instalado: se omite el instalador de Windows." >&2
fi
ls -lh dist
