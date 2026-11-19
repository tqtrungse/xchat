#!/usr/bin/env bash

exec curl -fsS -X POST \
  http://localhost:8180/realms/iam-local/protocol/openid-connect/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=password' \
  --data-urlencode 'client_id=iam-test' \
  --data-urlencode 'username=iam-test' \
  --data-urlencode 'password=iam-test-password' | jq -r .access_token