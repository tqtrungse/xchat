# xchat

## Repository layout

The repository has one Go module for each deployable and for the shared code.
`go.work` joins them for the editor and for local runs.

```
go.work
pkg/                 one Go module for each folder:
  bytesconv/         module xchat/pkg/bytesconv
  collection/        module xchat/pkg/collection
  config/            module xchat/pkg/config
  cpu/               module xchat/pkg/cpu
  database/          module xchat/pkg/database: SQL client and GORM helpers
  generator/         module xchat/pkg/generator
  jitter/            module xchat/pkg/jitter
  jwt/               module xchat/pkg/jwt
  k8s/               module xchat/pkg/k8s
  log/               module xchat/pkg/log: logger, zap fork, and test helpers
  pool/              module xchat/pkg/pool: generic pools
  reject/            module xchat/pkg/reject
services/
  iam/               module xchat/services/iam
    cmd/             main package, bootstrap, and the .template.env file
    internal/        code that only this service can import
  internal/shared/   module xchat/services/internal/shared: types that several
                     services share
deploy/              docker compose, Keycloak realm, database migrator image
scripts/             helper scripts that the tasks call
Taskfile.yaml        common commands: run `task` to list them
```

- A service reaches a `pkg` module and `internal/shared` through `replace`
  directives in its `go.mod`. Therefore, the module also builds without
  `go.work`. Run `task iam:check:isolated` and `task pkg:check:isolated` to
  check that.
- Every service can import `services/internal/shared`. `pkg` cannot, because
  the Go rule for an `internal` directory limits it to the code under
  `services/`. Another service cannot import `services/iam/internal` either.
  The compiler rejects both.
- `services/internal/shared` must not import a service. Its dependency would
  then point back to the service, and the modules would form a cycle. The rest
  of a model, such as `user.Info`, stays in the domain package of its service.
- A service reads its config relative to its own directory. Run it from
  `services/<name>`, or use `task iam:run`.

## IAM

IAM is the identity service of xchat. It uses Auth0 as the identity provider,
verifies each phone number on the device of the user, and controls the
registration of each device.

### Phone verification with Auth0 Passwordless

IAM uses the Auth0 Passwordless connection with SMS to verify the phone number
of a user. The verification occurs on the device:

1. The client starts the passwordless flow for the phone number.
2. Auth0 sends a one-time code by SMS.
3. The user enters the code on the device.
4. Auth0 verifies the code and gives an access token to the client.
5. The client sends the access token to IAM.
6. IAM verifies the token with the JWKS of the Auth0 tenant.

IAM stores only a truncated HMAC of the phone number and the last three
digits. The full phone number stays in Auth0.

### `ext` claim

The private claims of IAM are in one object, the `ext` claim of the access
token. An Auth0 Action adds this object. The Keycloak realm for local
development adds it with the claim name `ext.user_id`.

```json
{
  "ext": {
    "user_id": "123456789012345678"
  }
}
```

| Claim         | Value                    |
| ------------- | ------------------------ |
| `ext.user_id` | Snowflake ID of the user |

The Snowflake ID is the primary key of the user in IAM. It is a 64-bit
integer. Use a string for the claim value. A JSON number can lose precision
above 2^53 in a JavaScript client.

The Action sets the object with one call, for example
`api.accessToken.setCustomClaim('ext', { user_id: userID })`. Do not confuse
`ext` with `app_metadata` or `user_metadata` of the Auth0 user. Those stay in
Auth0. The Action copies only the values that IAM needs into `ext`.

### Configuration

IAM reads the OIDC settings from the environment:

| Variable                    | Description                                                                              |
| --------------------------- | ---------------------------------------------------------------------------------------- |
| `HTTP_SERVER_OIDC_ISSUER`   | Issuer URL of the Auth0 tenant, for example `https://example.us.auth0.com/`               |
| `HTTP_SERVER_OIDC_AUDIENCE` | API identifier of the IAM API in Auth0                                                   |
| `HTTP_SERVER_OIDC_JWKS_URL` | JWKS URL of the tenant, for example `https://example.us.auth0.com/.well-known/jwks.json` |

`HTTP_SERVER_BEARER_REALM` is optional. It sets the realm in the
`WWW-Authenticate` header of each authentication error, for example `iam`. If
it is empty, the header has no realm attribute.

Local development uses Keycloak. See `deploy/auth/local/README.md`.
