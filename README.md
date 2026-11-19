# xchat

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

### `user_id` claim

An Auth0 Action runs when Auth0 creates a user. The Action adds the `user_id`
claim to the tokens of the user. The value is the Snowflake ID that IAM uses as
the primary key of the user.

| Claim     | Value                    |
| --------- | ------------------------ |
| `user_id` | Snowflake ID of the user |

The Snowflake ID is a 64-bit integer. Use a string for the claim value. A JSON
number can lose precision above 2^53 in a JavaScript client.

### Configuration

IAM reads the OIDC settings from the environment:

| Variable                    | Description                                                                              |
| --------------------------- | ---------------------------------------------------------------------------------------- |
| `HTTP_SERVER_OIDC_ISSUER`   | Issuer URL of the Auth0 tenant, for example `https://example.us.auth0.com/`               |
| `HTTP_SERVER_OIDC_AUDIENCE` | API identifier of the IAM API in Auth0                                                   |
| `HTTP_SERVER_OIDC_JWKS_URL` | JWKS URL of the tenant, for example `https://example.us.auth0.com/.well-known/jwks.json` |

Local development uses Keycloak. See `deploy/auth/local/README.md`.
