#!/usr/bin/env bash
# Genera la demo estática en _site/ a partir del panel real, para que nunca
# se desincronicen: solo se le antepone el motor simulado.
set -euo pipefail
cd "$(dirname "$0")/.."
rm -rf _site && mkdir -p _site
sed '0,/<script>/s//<script src="mock.js"><\/script>\n<script>/' internal/web/index.html > _site/index.html
grep -q 'src="mock.js"' _site/index.html
cp web-example/mock.js assets/icon.png _site/
