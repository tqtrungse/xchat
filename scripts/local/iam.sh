#!/usr/bin/env bash

# Runs IAM locally. With -env=local, IAM reads .local.env from its working
# directory (services/iam/cmd), and an environment variable overrides a value of
# that file.
#
# The script prepares the local files that IAM needs:
#   - a self-signed certificate for localhost;
#   - an Ed25519 key to sign tickets, if the config has none;
#   - a FAKE descriptor of the hub, if the config has no descriptor file.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
SERVICE_DIR="$ROOT_DIR/services/iam"
LOCAL_DIR="$SERVICE_DIR/cmd"

CONFIG_FILE="$LOCAL_DIR/.local.env"
CERT_FILE="$LOCAL_DIR/.local.crt"
KEY_FILE="$LOCAL_DIR/.local.key"
# The default path of the fake hub descriptor. The script writes only this file.
LOCAL_DESCRIPTOR_FILE="$LOCAL_DIR/.local.hub-descriptor.json"
TEMPLATE_FILE="$SERVICE_DIR/cmd/.template.env"

# The values of the fake hub descriptor. They must match hub.Descriptor.Validate.
HUB_LOCAL_ADDRESS="localhost:2009"
HUB_LOCAL_AUDIENCE="xchat-hub-local"
HUB_LOCAL_KEY_ID=1
HUB_LOCAL_PROTOCOL="Noise_NK_25519_ChaChaPoly_SHA256"
HUB_LOCAL_VERSION=1

TEMP_FILE=""
cleanup() {
  if [[ -n "$TEMP_FILE" ]]; then
    rm -f -- "$TEMP_FILE"
  fi
}
trap cleanup EXIT

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Missing local IAM config: $CONFIG_FILE" >&2
  echo "Initialize it with:" >&2
  echo "  cp \"$TEMPLATE_FILE\" \"$CONFIG_FILE\"" >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "openssl is required to prepare the local IAM files" >&2
  exit 1
fi

# Read KEY from the config file. The last assignment wins. The function removes
# CR and the quotes around the value.
get_config() {
  local key="$1" value
  value="$(grep -E "^$key=" "$CONFIG_FILE" | tail -n 1 | cut -d= -f2- | tr -d '\r' || true)"
  value="${value#\"}"
  value="${value%\"}"
  printf '%s' "$value"
}

# Set KEY="value" in the config file. The function replaces the existing line,
# and keeps CRLF if the line uses it. If the key is absent, it appends the line.
# The value goes through the environment, so awk does not read escape sequences.
set_config() {
  local key="$1" value="$2"
  TEMP_FILE="$(mktemp "$CONFIG_FILE.XXXXXX")"
  if grep -qE "^$key=" "$CONFIG_FILE"; then
    SET_KEY="$key" SET_VALUE="$value" awk '
      index($0, ENVIRON["SET_KEY"] "=") == 1 {
        print ENVIRON["SET_KEY"] "=\"" ENVIRON["SET_VALUE"] "\"" (($0 ~ /\r$/) ? "\r" : "")
        next
      }
      { print }
    ' "$CONFIG_FILE" > "$TEMP_FILE"
  else
    cat -- "$CONFIG_FILE" > "$TEMP_FILE"
    [[ -z "$(tail -c 1 -- "$CONFIG_FILE")" ]] || echo >> "$TEMP_FILE"
    printf '%s="%s"\n' "$key" "$value" >> "$TEMP_FILE"
  fi
  # mktemp creates the file with mode 600, and the config can hold a private key.
  mv -f -- "$TEMP_FILE" "$CONFIG_FILE"
  TEMP_FILE=""
}

# IAM signs each ticket with an Ed25519 seed. Keep a configured key stable, and
# generate a key only for an empty value or a placeholder.
if [[ -z "$(get_config TICKET_SIGNING_KEY_ID)" ]]; then
  set_config TICKET_SIGNING_KEY_ID 1
fi

TICKET_SIGNING_PRIVATE_KEY="$(get_config TICKET_SIGNING_PRIVATE_KEY)"
if [[ -z "$TICKET_SIGNING_PRIVATE_KEY" || "$TICKET_SIGNING_PRIVATE_KEY" == *"<"* ]]; then
  # The last 32 bytes of the PKCS#8 DER form are the seed. The pipe keeps the
  # private key out of the file system.
  TICKET_SIGNING_PRIVATE_KEY="$(
    openssl genpkey -algorithm ED25519 |
      openssl pkey -outform DER |
      tail -c 32 |
      openssl base64 -A
  )"
  set_config TICKET_SIGNING_PRIVATE_KEY "$TICKET_SIGNING_PRIVATE_KEY"
  echo "Generated TICKET_SIGNING_PRIVATE_KEY in $CONFIG_FILE" >&2
fi
unset TICKET_SIGNING_PRIVATE_KEY

# The hub is not part of this repository yet. For a local run, the script makes
# a descriptor with a random X25519 public key. Nobody holds the private key, so
# no hub can accept a ticket for this descriptor. IAM only needs a valid file.
# The script never replaces a file at another path that the config sets.
DESCRIPTOR_FILE="$(get_config HUB_NOISE_DESCRIPTOR_FILE)"
if [[ -z "$DESCRIPTOR_FILE" ]]; then
  DESCRIPTOR_FILE="$LOCAL_DESCRIPTOR_FILE"
  set_config HUB_NOISE_DESCRIPTOR_FILE "$DESCRIPTOR_FILE"
fi

if [[ ! -s "$DESCRIPTOR_FILE" ]]; then
  if [[ "$DESCRIPTOR_FILE" != "$LOCAL_DESCRIPTOR_FILE" ]]; then
    echo "Missing hub descriptor: $DESCRIPTOR_FILE" >&2
    echo "Provision the file, or clear HUB_NOISE_DESCRIPTOR_FILE in $CONFIG_FILE" >&2
    echo "so that the script makes a fake descriptor for the local run." >&2
    exit 1
  fi

  HUB_LOCAL_PUBLIC_KEY="$(
    openssl genpkey -algorithm X25519 |
      openssl pkey -pubout -outform DER |
      tail -c 32 |
      openssl base64 -A
  )"
  (
    umask 077
    printf '{\n  "address": "%s",\n  "audience": "%s",\n  "key_id": %s,\n  "public_key": "%s",\n  "protocol": "%s",\n  "version": %s\n}\n' \
      "$HUB_LOCAL_ADDRESS" \
      "$HUB_LOCAL_AUDIENCE" \
      "$HUB_LOCAL_KEY_ID" \
      "$HUB_LOCAL_PUBLIC_KEY" \
      "$HUB_LOCAL_PROTOCOL" \
      "$HUB_LOCAL_VERSION" > "$DESCRIPTOR_FILE"
  )
  echo "Generated a FAKE hub descriptor for local use: $DESCRIPTOR_FILE" >&2
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

# The config loader looks for .local.env in the working directory. The script
# runs IAM from the cmd directory, where the main package and .local.env are.
cd "$LOCAL_DIR"
exec go run . -env=local
