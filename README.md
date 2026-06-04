# pulumi-mox

A native [Pulumi](https://www.pulumi.com/) provider for the
[mox](https://github.com/mjl-/mox) mail server, built with
[`pulumi-go-provider`](https://github.com/pulumi/pulumi-go-provider) (the
`infer` package). It lets Pulumi programs declare mox **domains**, **accounts**
and **addresses**, and read mox-generated **DNS records** so they can be
programmed into a DNS provider (e.g. Cloudflare).

> **Status: usable.** This repository compiles, vets cleanly, and ships three
> fully-wired resources (`Domain`, `Account`, `Address`) with idempotent Create,
> Read-based drift detection, Update, and Delete. See the
> [Open items](#open-items) section for what remains (DNS invoke, SDK
> publishing).

---

## Why

A mail migration consuming this provider needs to declare mox domains and
accounts as infrastructure and feed mox's generated DNS records into Cloudflare,
instead of running ad-hoc `docker exec mox mox ...` commands. A native provider
gives a typed, multi-language, drift-aware way to do that.

## Architecture

- Built with `infer`: the Go types in `provider/` (resources + config) are the
  single source of truth; the schema and all language SDKs are derived from
  them. No Terraform bridge.
- The provider talks to mox's **admin "sherpa" JSON-RPC API** over HTTP. See
  `internal/moxadmin/client.go`.
- Entry point: `provider/cmd/pulumi-resource-mox/main.go` →
  `provider.Provider()` → `prov.Run(ctx, "mox", provider.Version)`.

### Layout

```
provider/
  provider.go                  Config + Provider() builder (infer wiring)
  domain.go                    Domain resource — FULLY WIRED reference impl
  account.go                   Account resource — STUB (create/delete only)
  address.go                   Address resource — STUB (create/delete only)
  cmd/pulumi-resource-mox/
    main.go                    Plugin entrypoint
internal/moxadmin/
  client.go                    sherpa HTTP client (session/cookie + CSRF)
examples/yaml/
  Pulumi.yaml                  Smoke example (mox:Domain + dnsRecords output)
Makefile                       build / install_plugin / gen_sdk / tidy / test
```

## The mox admin API (sherpa)

Source of truth: `../mox/webadmin/admin.go` and `../mox/webadmin/api.json`
(sherpadoc, for exact param/return shapes).

- Transport: `POST {adminURL}/admin/api/<Method>` with a JSON body
  `{"params":[ ...positional args... ]}`. Response is `{"result": ...}` or
  `{"error":{"code","message"}}`. **User errors are prefixed `user:`** in the
  message — map these to friendly Pulumi errors (see Open items).
- **Auth is session/cookie + CSRF — NOT HTTP Basic.** Verified in
  `../mox/webauth/webauth.go`:
  1. `LoginPrep()` → returns a `loginToken` and sets a `webadminlogin` cookie.
  2. `Login([loginToken, password])` → returns a CSRF token and sets a
     `webadminsession` cookie.
  3. Subsequent calls send the `webadminsession` cookie (carried by a
     `cookiejar.Jar`) plus an `x-mox-csrf: <token>` header.
  The client logs in lazily on the first `Call`, so `pulumi preview` works
  without reaching the server.
- The admin UI is typically reachable privately (for example over a VPN or
  Tailscale) at a URL like `https://mox-admin.example.com`.

### Methods used (with line refs in `../mox/webadmin/admin.go`)

| Method                                                   | Line  | Used by            |
|----------------------------------------------------------|-------|--------------------|
| `Domains() []config.Domain`                              | :1565 | Domain.Read        |
| `DomainRecords(domain string) []string`                  | :1936 | Domain create/read |
| `DomainAdd(disabled bool, domain, account, localpart)`   | :1976 | Domain.Create      |
| `DomainRemove(domain string)`                            | :1985 | Domain.Delete      |
| `AccountAdd(accountName, address string)`                | :1995 | Account.Create     |
| `AccountRemove(accountName string)`                      | :2001 | Account.Delete     |
| `AddressAdd(address, accountName string)`                | :2007 | Address.Create     |
| `AddressRemove(address string)`                          | :2013 | Address.Delete     |
| `SetPassword(accountName, password string)` (min 8 chars)| :2021 | Account.Create     |

### Gotchas

- **DKIM/DNS ordering:** per-domain DKIM keys are generated on `DomainAdd`, so
  `DomainRecords` is only complete *after* the domain is added. `Domain.Create`
  adds the domain first, then reads records.
- **`Domain.Name` is nested:** the admin `Domain` type has
  `Name DomainName json:"Domain"` where `DomainName{ASCII, Unicode string}`.
  Match on `Domain.Name.ASCII` (see `domain.go` `Read`).
- **Config registration:** `Config` is registered as a pointer
  (`infer.Config(&Config{})`) so the pointer-receiver `Configure` can populate
  the shared `*moxadmin.Client`. Resources read a value copy via
  `infer.GetConfig[Config](ctx)` (which still carries the client pointer) — see
  `clientFromContext`. Do not add a `sync.Once` to `Config` (copylocks vet
  failure).
- **Version lock:** `pulumi-go-provider` v1.3.2 pins **both**
  `pulumi/pkg/v3` and `pulumi/sdk/v3` to **v3.232.0**. Keep them locked together;
  do not let `go mod tidy` bump `sdk` independently (3.245.0 changed
  `lang.InstallDependencies` and breaks the build).

## Resources

### `mox:Domain` (reference — fully wired)

Inputs: `domain` (required), `account` (optional, defaults to `domain`),
`localpart` (optional, defaults to `postmaster`), `disabled` (optional).
Output: `dnsRecords []string` (zone-file lines). Create/Read/Delete are
implemented; there is no Update (changes force a replace).

### `mox:Account` (fully wired)

Inputs: `account`, `address`, optional `password` (`provider:"secret"`).
Create is idempotent (adopts an existing account, then sets the password if one
was given); Read enumerates accounts to detect drift; Update rotates the
password via `SetPassword` and migrates the primary address via
`AddressAdd`/`AddressRemove` without replacing the account; Delete calls
`AccountRemove`. The `account` name is replace-only. **Password generation is the
caller's responsibility** — the consuming program generates with
`random.RandomPassword` and stores in a secret manager; the provider stays
generation-agnostic.

### `mox:Address` (fully wired)

Inputs: `address`, `account`. Create is idempotent (`AddressAdd`, treating
"already configured" as success); Read locates the owning account to detect
drift; Update re-points the address to a new account (remove-then-add with
rollback); Delete calls `AddressRemove`. The `address` is replace-only.

## Configuration

| Config key (`mox:`)   | Env fallback          | Secret | Notes                                   |
|-----------------------|-----------------------|--------|-----------------------------------------|
| `adminUrl`            | `MOX_ADMIN_URL`       | no     | Base admin URL (no `/admin/api` suffix) |
| `adminPassword`       | `MOX_ADMIN_PASSWORD`  | yes    | Used for LoginPrep/Login                |
| `insecureSkipVerify`  | —                     | no     | Dev/self-signed only                    |

## Build / test / release

```sh
eval "$(mise activate bash)"   # pins Go 1.25 (see mise.toml)

make build           # -> bin/pulumi-resource-mox
make vet             # go vet ./...
make test            # go test ./...
make install_plugin  # install the built plugin into the local Pulumi cache
make gen_sdk         # generate language SDKs into sdk/ (do NOT commit blindly)
```

To try the example against a dev mox:

```sh
make install_plugin
cd examples/yaml
pulumi stack init dev
pulumi config set mox:adminUrl https://mox-admin.example.com
pulumi config set --secret mox:adminPassword <password>
pulumi up
```

## Open items

These are left for the implementer:

- **DNS invoke:** consider exposing a `getDomainRecords` invoke in addition to
  the `Domain.dnsRecords` output.
- **Schema/SDK publishing:** decide on versioning + whether to commit generated
  SDKs (`gen_sdk`) or publish to the Pulumi registry.

Done: idempotent Create (adopt on "already exists"), `Update` for
Account/Address, `Read` refresh on all resources, and sherpa `user:*` error
mapping into Pulumi diagnostics.

## Related

- `../mox` — the mox fork (admin API source of truth).
- A consuming program imports this provider's SDK to declare domains/accounts
  and feed `dnsRecords` into a DNS provider such as Cloudflare.
