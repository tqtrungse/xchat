#!/usr/bin/env bash

# Runs IAM locally. With -env=local, IAM reads cmd/iam/.local.env from the
# directory of the service (services/iam), and an environment variable
# overrides a value of that file.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
SERVICE_DIR="$ROOT_DIR/services/iam"

CONFIG_FILE="$SERVICE_DIR/cmd/iam/.local.env"
CERT_FILE="$SERVICE_DIR/cmd/iam/.local.crt"
KEY_FILE="$SERVICE_DIR/cmd/iam/.local.key"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Missing local IAM config: $CONFIG_FILE" >&2
  echo "Initialize it with:" >&2
  echo "  cp \"$SERVICE_DIR/cmd/iam/.example.env\" \"$CONFIG_FILE\"" >&2
  exit 1
fi

# IAM serves only HTTPS. Create a self-signed certificate for localhost once.
if [[ ! -s "$CERT_FILE" || ! -s "$KEY_FILE" ]]; then
  (
    umask 077
    openssl req -x509 -newkey rsa:2048 -sha256 -nodes -days 365 \
      -keyout "$KEY_FILE" \
      -out "$CERT_FILE" \
      -subj "/CN=localhost" \
      -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
  )
  echo "Generated a self-signed certificate: $CERT_FILE" >&2
fi

export HTTP_SERVER_CERT_FILE="$CERT_FILE"
export HTTP_SERVER_KEY_FILE="$KEY_FILE"

# The config loader looks for cmd/iam/.local.env relative to the working
# directory, so the service runs from its own directory.
cd "$SERVICE_DIR"
exec go run ./cmd/iam -env=local
