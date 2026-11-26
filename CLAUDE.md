# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

All common commands are Taskfile tasks. Run `task` to list them. The namespaces are `iam:`, `pkg:`, and `shared:`.

```bash
task iam:build | iam:vet | iam:test | iam:lint | iam:fmt   # IAM module
task pkg:test MODULES="jwt log"                            # only some pkg modules (default: all)
task iam:all                                               # mock, json, lint, vet, test, build
task iam:check:isolated                                    # tidy check + build/vet/test with GOWORK=off (as CI and Docker do)
task affected TARGET=lint,vet,test [BASE=origin/master]    # run tasks only on modules the diff affects (needs gta)
```

Run one test from the directory of its module (each module has its own `go.mod`):

```bash
cd services/iam && go test ./internal/application/device/register/ -run TestDeviceRegisterCreateUser
```

Code generation. Do not edit the output by hand:

- `task iam:mock` / `task pkg:mock`: `mockgen` mocks into a `mocks/` package next to the interface. When you add an interface file, add its `mockgen` line to the `mock` task of that module's `Taskfile.yaml`.
- `task iam:json`: `easyjson` for the `payload.go` files of the use cases (`*_easyjson.go`). Add new payload files to the `json` task.

Local run:

```bash
task infra:up        # docker compose: MySQL (host port 9999) and Keycloak (http://localhost:8180)
task iam:dbmigrate   # apply services/iam/db/schemas with ridgepole (Schemafile includes *.schema)
task iam:run         # scripts/local/iam.sh: makes a self-signed cert, a ticket signing key, a fake hub descriptor, then runs IAM
task iam:token       # access token of the local Keycloak test user
```

`task iam:run` needs `services/iam/cmd/.local.env`. Copy it from `.template.env`. `deploy/auth/local/.env` and `deploy/mysql/local/.env` (from `.template`) must exist for `infra.sh`. See `deploy/auth/local/README.md`.

## Repository layout

- One Go module for each deployable and for each shared library. `go.work` joins them for the editor and for local runs.
- `pkg/<name>`: one module per folder (`xchat/pkg/jwt`, `xchat/pkg/database`, ...). They are generic libraries. `pkg/log/zap` is a fork of zap (see its README).
- `services/iam`: the identity service (module `xchat/services/iam`).
- `services/internal/shared`: types that several services share (ticket, hub descriptor, device ID/state).
- `scripts/affected`: the CLI behind `task affected`. It is intentionally not in `go.work`. Run it with `GOWORK=off go -C scripts/affected ...`.

Module rules:

- A service gets `pkg` modules and `internal/shared` through `replace` directives in its `go.mod`, so it must also build with `GOWORK=off`. After a dependency change, run `task <ns>:tidy` and `task <ns>:check:isolated`.
- `pkg` cannot import `services/internal/shared`, because Go `internal` rules forbid it. A service cannot import another service's `internal`. `services/internal/shared` must not import a service, or the modules form a cycle.
- `config.Load(service, env)` reads `.local.env` / `.prod.env` (selected by the `-env` flag) relative to the working directory. Environment variables override the file. Run a service from `services/<name>/cmd`, or with `task iam:run`.

## IAM architecture

IAM uses hexagonal layers under `services/iam/internal`:

- `domain/{user,device}`: IDs, states, info types, and domain errors.
- `application/device/<usecase>`: one package for each use case (`register`, `issueticket`). It contains `usecase.go`, `ports.go` (the interfaces that the use case needs), `payload.go` (easyjson request/response), and `errors.go`.
- `adapter/http`: a gin HTTPS server. `route/` has one handler for each use case. The handler depends on a small interface of its use case (for example `DeviceTicketIssuer`), which is mocked in `route/mocks`. `route/handler.go` has the shared steps: `requireAuthenticatedUser`, `withTimeout`, `decodeRequestBody`, and `respondUsecaseFailure`. Each handler maps its use case's errors to a `usecaseFailure`. A response never contains the text of the error. Client rejections are counted in `reject.Stats[rejection.Reason]`.
- `infrastructure/mysql`: GORM readers and writers that implement the ports. Their tests use `go-sqlmock`, not a real database. `infrastructure/crypto`: bcrypt, X25519, and the Ed25519 ticket signer.
- `cmd/bootstrap`: the manual dependency injection. `routes.go` `buildRoutes` builds each use case and its handler. To add a use case, wire it there and append its handler to the returned `[]iamhttp.Mounter`. All config is read and validated before the database opens.

Database access pattern (`pkg/database/sql`):

- A use case calls `db.Transaction(ctx, sql.TxSourcePrimary, fn)` or `db.WithDB(ctx, source, fn)`. The callback context carries the handle. Repository code gets it with `sql.ExtractDB(dbCtx)`. Port methods take this `dbCtx` as the first argument.
- Do not nest `Transaction` or `WithDB` inside a callback. They always start from the root DB.
- A reader returns `nil, nil` when no row exists.

Authentication: the bearer token is an OIDC access token (Auth0 in production, Keycloak locally). It is verified against the JWKS from `HTTP_SERVER_OIDC_*`. IAM's private claims are in one `ext` object. `ext.user_id` is the Snowflake user ID as a string. IAM stores only a truncated HMAC of the phone number and its last three digits. See `README.md` for the details.

Pooled buffers: some APIs transfer ownership of a `[]byte` from `pkg/pool/slice` (for example `TicketSigner.Sign`). The caller must clear the buffer and return it to the pool. Handlers return the byte fields of a request (password, keys) with `slicepool.PutZeroed` after use, because they can hold secrets.

## Conventions

- Each `.go` file starts with the Apache-2.0 copyright header that the existing files use.
- Import groups, in this order: standard library; project imports that are not in `pkg`/`libs`; project `xchat/pkg/...` imports; third-party imports.
- Comments, including Go doc comments, use ASD-STE100 style: short sentences, active voice, present tense, `must` / `do not` for requirements. The comments explain the reason, not what the code shows.
- Tests of a caller use the generated gomock mocks of its ports (`gomock.NewController(t)`, `EXPECT()` with checked arguments). Do not write hand-written fakes.
