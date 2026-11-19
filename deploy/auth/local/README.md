# Local Keycloak for IAM

This setup is for local development only. Keycloak imports the realm `iam-local`
from `auth-realm.json`. The realm has a password-grant test client `iam-test`
and a test user `iam-test` / `iam-test-password`. The client adds `iam-api` to
the audience of the access token.

## Start Keycloak

`scripts/local/infra.sh` starts the services of
`deploy/local-docker-compose.yaml`. It reads two environment files:

- `deploy/auth/local/.env` with the Keycloak settings:

  ```sh
  AUTH_REALM_JSON=/absolute/path/to/xchat/deploy/auth/local/auth-realm.json
  AUTH_ADMIN_USER=admin
  AUTH_ADMIN_PASSWORD=<choose a password>
  ```

  Docker Compose resolves a relative `AUTH_REALM_JSON` from the `deploy`
  folder, so an absolute path is the simplest choice.

- `deploy/mysql/local/.env`, created from `deploy/mysql/local/.template`.

Start Keycloak from the repository root:

```sh
scripts/local/infra.sh up -d keycloak
```

Keycloak listens on `http://localhost:8180`. The host port is not 8080,
because local tools often take that port. Check that the JWKS has a key:

```sh
curl -s http://localhost:8180/realms/iam-local/protocol/openid-connect/certs
```

The response must be JSON with a non-empty `keys` array.

Keycloak requires the import file name `<realm>-realm.json`, so the compose
file mounts `auth-realm.json` as `iam-local-realm.json`. If you rename the realm,
change that mount too, or Keycloak fails with `File name / realm name
mismatch`.

Keycloak imports a realm only when the realm does not exist. After a change to
`auth-realm.json`, recreate the container:

```sh
scripts/local/infra.sh up -d --force-recreate keycloak
```

## Start IAM

`services/iam/cmd/iam/.example.env` has the OIDC settings for this realm:

```sh
HTTP_SERVER_OIDC_ISSUER=http://localhost:8180/realms/iam-local
HTTP_SERVER_OIDC_AUDIENCE=iam-api
HTTP_SERVER_OIDC_JWKS_URL=http://localhost:8180/realms/iam-local/protocol/openid-connect/certs
```

IAM accepts these `http` URLs only because the host is a loopback address. Any
other host must use `https`.

Create the local config once, then start IAM:

```sh
cp services/iam/cmd/iam/.example.env services/iam/cmd/iam/.local.env
scripts/local/iam.sh
```

The script creates a self-signed certificate for `localhost` and runs IAM with
`-env=local`. IAM reports readiness after it loads the JWKS:

```sh
curl -k -i https://localhost:2008/readyz
```

`204` means that IAM can verify a token. `503` means that IAM has no key yet;
the log of IAM shows the reason.

## Get an access token

Get the token from the same host and port as the issuer above. Keycloak puts
that URL in the `iss` claim, and IAM rejects a token from another issuer.
This command requires `jq`:

```sh
ACCESS_TOKEN="$(curl -fsS -X POST \
  http://localhost:8180/realms/iam-local/protocol/openid-connect/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=password' \
  --data-urlencode 'client_id=iam-test' \
  --data-urlencode 'username=iam-test' \
  --data-urlencode 'password=iam-test-password' | jq -r .access_token)"
```

Send the token to an authenticated route under `/api`:

```sh
curl -k -H "Authorization: Bearer $ACCESS_TOKEN" https://localhost:2008/api/...
```
