# AGENTS.md

Guidance for AI coding agents working in this repository. Applies to any agent that reads
`AGENTS.md` (Codex, Cursor, Copilot, Claude Code, …).

## Repository

A Terraform provider for [OpenZiti](https://openziti.io), built on the
`terraform-plugin-framework` (v1.x, not the legacy SDKv2). Published to the Terraform Registry as
`netfoundry/ziti`. Release is driven by goreleaser on a pushed `v*` tag
(`.github/workflows/release.yml`) — that workflow is the only CI; there is no test or lint job.

## Common commands

The `GNUmakefile` is the primary entry point. The default target runs `fmt lint install generate`.

- `make build` — `go build -v ./...`
- `make install` — `go install -v ./...` (puts the binary in `$GOPATH/bin`, which is what
  `dev_overrides` points at)
- `make fmt` — `gofmt -s -w -e .`
- `make lint` — `golangci-lint run` (config in `.golangci.yml`, schema `version: "2"`).
  `golangci-lint` is not vendored; it must be on `PATH` or the default `make` target fails at this
  step. Run the other targets individually if it is unavailable.
- `make generate` — runs `go generate ./...` from `tools/`: (1) `terraform fmt -recursive
  ../examples/`, (2) regenerates `docs/` via `tfplugindocs`. Requires `terraform` on `PATH`.
- `make test` / `make testacc` — the repo has no Go test files, so both currently do nothing.
  Verification is manual (see below). If you add tests, a single one runs with
  `go test -v -run TestName ./internal/provider/...`; acceptance tests are gated behind `TF_ACC=1`.

## Local plugin development

The provider's `Address` is `hashicorp.com/netfoundry/ziti` (set in `main.go`), **not** the registry
address. Local development relies on a `dev_overrides` block in `~/.terraformrc` pointing
`netfoundry/ziti` at `$GOPATH/bin` (see README). **Do not run `terraform init` while `dev_overrides`
is active** — it will try to fetch from the registry and fail. Re-run `make install` after code
changes; Terraform picks up the new binary automatically. `version` is `"dev"` in local builds and
injected by goreleaser on release.

### Manual verification

`testing/resources/<tf_type>/main.tf` and `testing/data-sources/<tf_type>/main.tf` are standalone
HCL files for end-to-end verification against a real Ziti controller. Each includes its own
`terraform { required_providers {} }` and `provider "ziti" {}` block and relies on env vars for
credentials. This is the only functional test path — exercise a change here before considering it
done, and never run it against a shared or production controller without the owner's approval.

## Authentication / environment

`provider.go`'s `Configure` runs once, authenticates, and stashes a `*zitiData{apiToken, host}` into
both `resp.ResourceData` and `resp.DataSourceData`. Every resource and data source pulls it back out
in its own `Configure` via `req.ProviderData.(*zitiData)`.

Two auth methods, selected implicitly:

- **Certificate (mTLS)** — used whenever both cert and key PEM resolve to non-empty.
  `POST <host>/authenticate?method=cert`, presenting the client cert during the TLS handshake. Cert
  material resolution order: `identity_file` > `identity_json` > explicit `cert`/`key`/`ca`
  attributes (explicit fields override values extracted from identity JSON). Identity JSON is a Ziti
  identity file; `parsePemFromZitiIdentity` pulls `id.cert` / `id.key` / `id.ca` and strips the
  `pem:` prefix.
- **Username/password** — the fallback. `POST <host>/authenticate?method=password` with a JSON body.
  `username`/`password` are required only when cert auth is not in play.

Both return `data.token`, which is sent as the `zt-session` header on every subsequent call.

Env vars: `ZITI_API_HOST`, `ZITI_API_USERNAME`, `ZITI_API_PASSWORD`, `ZITI_API_IDENTITY_FILE`,
`ZITI_API_IDENTITY_JSON`. Config attributes take precedence over env vars; `ZITI_API_HOST` is
consulted only when neither `host` nor `hosts` yields a value.

`host` must already include the management API base path, e.g.
`https://ctrl.example.com:1280/edge/management/v1`.

### HA / multi-controller

`host` (single) and `hosts` (list) are merged into one deduplicated, ordered candidate list — `host`
first, then `hosts` — and tried in order; the first successful authentication wins. When `hosts` was
non-empty, the provider then calls `/fabric/v1/cluster/list-members` (URL built from the scheme and
authority of the active host, dropping the management base path), sorts members leader-first,
converts each `tls:HOST:PORT` address back to `https://HOST:PORT<original path>` via
`memberToHost`, and re-authenticates — so the leader is preferred for writes. Failures here only log
a warning and keep the already-working host.

### TLS

TLS verification is **disabled** (`InsecureSkipVerify: true`) in password auth, cluster-member
discovery, and the shared request helper in `common.go`. The one exception: cert auth sets `RootCAs`
and enables verification when a `ca` PEM is supplied and parses successfully.

## Architecture

### Provider wiring

`internal/provider/provider.go` defines `zitiProvider`, its schema, and the `DataSources()` /
`Resources()` lists. **Any new resource or data source must be added to one of those slices** or it
will not be registered. `main.go` serves it via `providerserver.Serve` (with a `-debug` flag for
delve).

### Shared HTTP client

`internal/provider/common.go` is the single HTTP layer. All CRUD helpers (`CreateZitiResource`,
`ReadZitiResource`, `UpdateZitiResource`, `PatchZitiResource`, `DeleteZitiResource`) delegate to
`doRequest`, which uses `hashicorp/go-retryablehttp` for transparent retries. A 404 returns the
sentinel `errNotFound`; Read handlers check `errors.Is(err, errNotFound)` and call
`resp.State.RemoveResource(ctx)` to drop the resource from state rather than erroring.

Responses are parsed with `tidwall/gjson` (e.g. `gjson.Get(cresp, "data.id")`) for quick path
extraction, or `json.Unmarshal` into `map[string]interface{}` when the full payload is needed.

### Resource / data source conventions

Each Ziti entity has paired files in `internal/provider/`:

- `<name>_resource.go` — full CRUD + import (`ResourceWithImportState`)
- `<name>_data_source.go` — read-only lookup

Resources follow a consistent skeleton: `Metadata` sets `TypeName = req.ProviderTypeName +
"_<name>"`, `Schema` describes attributes (each with a `MarkdownDescription` that flows into
generated docs), and CRUD methods build URLs as `fmt.Sprintf("%s/<collection>[/<id>]",
r.resourceConfig.host, ...)`. IDs are `url.QueryEscape`d in paths.

Payload types come from `github.com/openziti/edge-api/rest_model` (e.g.
`rest_model.EdgeRouterPolicyCreate`, `rest_model.Roles`, `rest_model.Tags`).

Identity resources are split across four files because the upstream API has four creation variants
with different required fields: `identity_resource.go` (generic), `identity_updb_resource.go`
(username/password DB), `identity_ca_resource.go` (CA-enrolled), `identity_none_resource.go`. Keep
their schemas aligned where fields overlap.

### Drift suppression in Read

The Ziti API materializes defaults for fields that were never configured (e.g.
`listenOptions.connectTimeout` → `"0s"`, `connectTimeoutSeconds` → `0`) and omits empty strings
entirely. A naive Read writes those back and every `plan` then shows a spurious diff. The convention
is for Read to build a `newState` from the API response, then selectively copy values back from the
*prior* state:

- Identity-ish fields (`ID`, `Name`, `ConfigTypeId`, `Tags`, `LastUpdated`) are carried over from old
  state wholesale.
- API-default-vs-null mismatches go through `preserveNullListenOptionAttr(newObj, oldObj, attrName,
  zeroValue, nullValue)` in `config_host_v1_resource.go` — if the API returned the zero value and old
  state held null, null is kept.
- Omitted-empty-string fields are restored when old state held `""` and the response gave null.

Follow this pattern for any new attribute whose API representation differs from "unset".

### `utilities.go`

Reflection-heavy helpers that translate between Terraform's `types.*` values and `rest_model`
structs / generic JSON maps. Prefer these over hand-rolled reflection:

- `JsonStructToObject` / `JsonStructToObject2` — struct → `map[string]interface{}` with configurable
  omitempty and pointer-deref behavior
- `AttributesToNativeTypes` / `NativeBasicTypedAttributesToTerraform` — round-trip `types.Object`
  attributes through Go-native maps
- `convertKeysToCamel` / `convertKeysToSnake` (via `iancoleman/strcase`) — bridge Terraform's
  snake_case schema to the API's camelCase JSON
- `TagsFromAttributes` — `types.Map` → `*rest_model.Tags`; returns `nil` when empty so the field can
  be omitted
- `ElementsToListOfStructs[T]` / `ElementsToListOfStructsPointers[T]` — generic helpers for nested
  lists of objects

## Docs and examples

Everything under `docs/` is generated — **do not edit it by hand**. The sources are:

1. Attribute `MarkdownDescription` strings in the Go schema
2. `examples/resources/<tf_type>/resource.tf`, `examples/data-sources/<tf_type>/data-source.tf`,
   `examples/resources/<tf_type>/import.sh`
3. `examples/provider/provider.tf`

Run `make generate` after any schema or example change.

**`docs/guides/` is destroyed by `make generate`.** `tfplugindocs` treats `guides/` as a managed
subdirectory and removes it before rendering, and this repo has no `templates/` directory to
re-seed it from. A hand-written guide must live at `templates/guides/<name>.md.tmpl` to survive
regeneration.

## Changelogs

Versioned notes live in `changelogs/CHANGELOG.<major>.<minor>.md`. Update the file matching the
target release when landing user-visible changes.
