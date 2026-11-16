#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"

cd "$ROOT_DIR"

### Pkg

mockgen \
  -source=pkg/database/sql/ports.go \
  -destination=pkg/database/sql/mocks/ports_mock.go \
  -package=mocks

mockgen \
  -source=pkg/generator/ports.go \
  -destination=pkg/generator/mocks/ports_mock.go \
  -package=mocks