#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"

cd "$ROOT_DIR"

### IAM

mockgen \
  -source=internal/iam/application/registerdevice/ports.go \
  -destination=internal/iam/application/registerdevice/mocks/ports_mock.go \
  -package=mocks

### Pkg

mockgen \
  -source=pkg/database/sql/port.go \
  -destination=pkg/database/sql/mocks/port_mock.go \
  -package=mocks

mockgen \
  -source=pkg/generator/port.go \
  -destination=pkg/generator/mocks/port_mock.go \
  -package=mocks

mockgen \
  -source=pkg/jwt/port.go \
  -destination=pkg/jwt/mocks/port_mock.go \
  -package=mocks