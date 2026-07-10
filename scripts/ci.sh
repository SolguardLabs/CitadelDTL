#!/usr/bin/env bash
set -euo pipefail

gofmt -l cmd src >/tmp/citadeldtl_gofmt_check.txt
if [ -s /tmp/citadeldtl_gofmt_check.txt ]; then
  cat /tmp/citadeldtl_gofmt_check.txt
  exit 1
fi

go test ./...
npm run check
npm test
npm run loc
