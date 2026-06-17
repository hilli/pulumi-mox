# pulumi-mox

A native [Pulumi](https://www.pulumi.com/) provider for the
[mox](https://github.com/mjl-/mox) mail server, built with
[`pulumi-go-provider`](https://github.com/pulumi/pulumi-go-provider) (the
`infer` package). It lets Pulumi programs declare mox **domains**, **accounts**
and **addresses**, and read mox-generated **DNS records** so they can be
programmed into a DNS provider (e.g. Cloudflare).

> **Status: usable.** This repository compiles, vets cleanly, and ships fully
> wired resources for domains, accounts, addresses, aliases, Sieve scripts,
> global routes, webserver config, log levels, and DNSBL monitoring with
> idempotent Create, Read-based drift detection, Update, and Delete. A committed,
> tagged Go SDK is
> published; other languages are generated on demand (see
> [Install / use](#install--use)). See [Open items](#open-items) for what
> remains.

---

## Why

A mail migration consuming this provider needs to declare mox domains and
accounts as infrastructure and feed mox's generated DNS records into Cloudflare,
instead of running ad-hoc `docker exec mox mox ...` commands. A native provider
gives a typed, multi-language, drift-aware way to do that.

## Install / use

The provider ships a prebuilt plugin (GitHub releases) plus a committed, tagged
**Go SDK**. Other languages are generated on demand with `pulumi package add` —
this works because the published schema carries the correct repository, license,
plugin-download URL and Go import path.

> Replace `X.Y.Z` with a released version — see
> [Releases](https://github.com/hilli/pulumi-mox/releases) (e.g. `0.1.0`).

### Go

```sh
go get github.com/hilli/pulumi-mox/sdk/go/mox@vX.Y.Z
```

The plugin-download URL is baked into the SDK, so Pulumi auto-installs the
matching plugin binary at deploy time — no separate `plugin install` step.

### Other languages (Python, Node.js, .NET, Java, YAML)

Install the plugin from GitHub, then add the package to your project (Pulumi
generates a local SDK from the provider's schema):

```sh
pulumi plugin install resource mox X.Y.Z --server github://api.github.com/hilli/pulumi-mox
pulumi package add mox@X.Y.Z
```

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
  domain.go                    Domain resource — full CRUD + mutable domain config
  account.go                   Account resource — full CRUD + account settings/routes
  address.go                   Address resource — full CRUD
  alias.go                     Alias/mailing-list resource — full CRUD
  sieve.go                     Account Sieve script resource — full CRUD + rename
  global_routes.go             Server-level outgoing routes singleton resource
  webserver_config.go          Web redirects/handlers singleton resource
  log_level.go                 Package log-level override resource
  dnsbl_monitoring.go          DNSBL monitoring singleton resource
  cmd/pulumi-resource-mox/
    main.go                    Plugin entrypoint
internal/moxadmin/
  client.go                    sherpa HTTP client (session/cookie + CSRF)
examples/yaml/
  Pulumi.yaml                  Localserve smoke example used by make e2e
examples/yaml-advanced/
  Pulumi.yaml                  Richer reference example for real deployments
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
Outputs: `effectivePassword` (secret) — the password the account was created
with. Create is idempotent (adopts an existing account); when a `password` is
supplied it is applied via `SetPassword` and echoed into `effectivePassword`,
and when none is supplied on a brand-new account the provider generates one
(crypto/rand, 24-char alphanumeric) and surfaces it via `effectivePassword`.
Adopting a pre-existing account never changes its password (and leaves
`effectivePassword` empty). Read enumerates accounts to detect drift; Update
rotates the password via `SetPassword` (refreshing `effectivePassword`) and
migrates the primary address via `AddressAdd`/`AddressRemove` without replacing
the account; Delete calls `AccountRemove`. The `account` name is replace-only.
Retrieve a generated password with
`pulumi stack output <name> --show-secrets`. Note mox stores passwords hashed,
so `effectivePassword` only ever reflects a password the provider itself set — a
password set out-of-band cannot be recovered.

### `mox:Address` (fully wired)

Inputs: `address`, `account`. Create is idempotent (`AddressAdd`, treating
"already configured" as success); Read locates the owning account to detect
drift; Update re-points the address to a new account (remove-then-add with
rollback); Delete calls `AddressRemove`. The `address` is replace-only.

### `mox:Alias` (fully wired)

Inputs: `address`, `members`, and optional list behavior flags. Create uses
`AliasAdd`; Read reflects membership and managed settings from `DomainConfig`;
Update adds/removes members and updates list behavior in place; Delete calls
`AliasRemove`.

### `mox:Sieve` (fully wired)

Inputs: `account`, `name`, `content`, optional `active`. Create stores and
optionally activates the script; Read fetches script content and active state;
Update can rename the script, replace content, and activate/deactivate it;
Delete deactivates first when needed and removes the script.

### `mox:GlobalRoutes` (fully wired)

Singleton resource for server-level outgoing routes. Inputs: `routes`, using the
same route shape as account/domain routes. Create/Update call `RoutesSave`; Read
reflects `Config().Routes`; Delete clears the global route list.

### `mox:WebserverConfig` (fully wired)

Singleton resource for dynamic web redirects and web handlers. Inputs:
`redirects` (`from`/`to`) and ordered `handlers` (static, redirect, forward, or
internal). Create/Update use `WebserverConfigSave` with mox's old/current
optimistic check; Read calls `WebserverConfig`; Delete clears redirects and
handlers.

### `mox:LogLevel` (fully wired)

Manages one package log-level override. Inputs: `package`, `level`. Create/Update
call `LogLevelSet`; Read reflects `LogLevels`; Delete calls `LogLevelRemove`.
Changing `package` replaces the resource.

### `mox:DNSBLMonitoring` (fully wired)

Singleton resource for dynamic DNS blocklists that mox monitors for outgoing IP
listings without using those DNSBLs for incoming-message rejection. Inputs:
`zones []string`. Create/Update call `MonitorDNSBLsSave`; Read reflects
`Config().MonitorDNSBLs`; Delete clears the monitored zone list.

### `getCheckUpdatesEnabled` (read-only)

Returns whether mox's static `CheckUpdates` setting is enabled. Mox exposes this
through the admin API as `CheckUpdatesEnabled`, but there is no admin API setter:
the setting lives in `mox.conf`, outside this provider's API-driven write model.

## Configuration

| Config key (`mox:`)   | Env fallback          | Secret | Notes                                   |
|-----------------------|-----------------------|--------|-----------------------------------------|
| `adminUrl`            | `MOX_ADMIN_URL`       | no     | Base admin URL (no `/admin/api` suffix) |
| `adminPassword`       | `MOX_ADMIN_PASSWORD`  | yes    | Used for LoginPrep/Login                |
| `insecureSkipVerify`  | —                     | no     | Dev/self-signed only                    |

### Reaching a non-public admin API

The mox admin API is sensitive and is usually not exposed to the public
internet — it's commonly bound to `localhost` (or a private interface) on the
mail host. The provider only needs to be able to reach `adminUrl`; **how you
make that endpoint reachable is entirely up to you.** Pulumi has no built-in
tunnelling feature, and the provider does not open connections for you. Any of
these work, pick whatever fits your environment:

- a mesh/overlay VPN (WireGuard, Tailscale, etc.) so the host is reachable on a
  private address;
- a bastion / jump host;
- private network peering, or simply running Pulumi from inside the same
  network;
- a reverse proxy that terminates TLS with a valid certificate;
- an SSH tunnel (shown below) — a zero-infrastructure option that's handy for
  one-off or local runs.

The rest of this section walks through the SSH-tunnel option as a concrete
**suggestion**, not a requirement. The provider's HTTP client is a clone of
Go's `http.DefaultTransport`, so it already honours the standard `HTTP_PROXY` /
`HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY` environment variables with no code
changes — which is what makes the SOCKS approach below work without touching the
provider.

There are two practical SSH approaches.

#### 1. Local port forward (recommended)

Forward a local port to the admin port on the mail host:

```sh
# foreground; Ctrl-C to close
ssh -L 8443:localhost:443 user@mail.example.com

# or background it (-f -N = no remote command, go to background)
ssh -f -N -L 8443:localhost:443 user@mail.example.com
```

`localhost:443` is resolved **on the mail host**, so this also works when the
admin interface is only listening on the host's loopback.

The catch is TLS: the mox certificate is issued for `mail.example.com`, not
`localhost`, so pointing `adminUrl` at `https://localhost:8443` fails
hostname verification. Pick one:

- **Map the real hostname to the tunnel** (keeps verification on — preferred).
  Add a hosts entry so the real name resolves to loopback:

  ```sh
  echo "127.0.0.1 mail.example.com" | sudo tee -a /etc/hosts
  ```

  then forward on the real port and use the real URL:

  ```sh
  ssh -f -N -L 443:localhost:443 user@mail.example.com   # needs local root for :443
  pulumi config set mox:adminUrl https://mail.example.com
  ```

- **Skip verification** (quick, less safe — dev only):

  ```sh
  pulumi config set mox:adminUrl https://localhost:8443
  pulumi config set mox:insecureSkipVerify true
  ```

#### 2. SOCKS proxy (`ssh -D`)

If you do not want per-host forwards, open a dynamic SOCKS proxy and let the
provider route through it. This preserves TLS verification because the request
still targets the real hostname:

```sh
ssh -f -N -D 1080 user@mail.example.com

# socks5h = resolve DNS through the proxy too
export ALL_PROXY=socks5h://localhost:1080
pulumi config set mox:adminUrl https://mail.example.com
pulumi up
```

`HTTPS_PROXY` works the same way for an HTTP/HTTPS forward proxy. Note that a
plain `ssh -L` local forward is **not** an HTTP proxy, so don't set
`HTTPS_PROXY` to a forwarded port — use approach 1's `adminUrl` instead.

## Build / test / release

```sh
eval "$(mise activate bash)"   # pins Go 1.25 (see mise.toml)

make build           # -> bin/pulumi-resource-mox
make vet             # go vet ./...
make test            # go test ./...
make install_plugin  # install the built plugin into the local Pulumi cache
make gen_sdk         # generate ALL language SDKs into sdk/ (scratch; gitignored except sdk/go)
make gen_go_sdk      # regenerate the committed Go SDK (sdk/go/mox) at $(VERSION)
make sdk_build       # tidy + build the committed Go SDK module
```

The Go SDK under `sdk/go` is committed and CI-verified against the schema. After
changing resources/config, run `make gen_go_sdk && make sdk_build` and commit the
result so `go get` consumers stay in sync; CI fails on drift.

To try the example against a dev mox:

```sh
make install_plugin
cd examples/yaml
pulumi stack init dev
pulumi config set mox:adminUrl https://mox-admin.example.com
pulumi config set --secret mox:adminPassword <password>
pulumi up
```

`examples/yaml/Pulumi.yaml` is intentionally localserve-friendly and safe for
`make e2e`. It exercises every writable resource with values that work against a
throwaway local server. `examples/yaml-advanced/Pulumi.yaml` shows richer
real-world shapes: non-empty account/domain/global routes, Sieve activation,
aliases, web redirects/handlers, log levels, DNSBL monitoring, and the read-only
update-check invoke. The advanced route examples reference named transports that
must already exist in static `mox.conf`.

### Live e2e with `mox localserve`

`make e2e` runs the `examples/yaml` stack against a throwaway, self-contained
mox backend — no real server or config required. It:

1. takes `mox` from `PATH`, or installs it with `go install github.com/mjl-/mox@latest`;
2. starts an ephemeral `mox localserve` (admin API on `http://localhost:1080`,
   password `moxadmin`) in a temp dir;
3. stands up the stack against a throwaway `file://` Pulumi backend, runs
   `pulumi up` (creating `example.com`, accounts, an alias, Sieve script,
   singleton config resources, and read-only invokes), performs a second
   `pulumi up` to exercise Sieve rename, prints key outputs, then
   `pulumi destroy`s and tears the whole thing down.

```sh
make e2e
```

The `testuser` account omits a `password` input, so the provider generates one
and exposes it as the `effectivePassword` secret output (surfaced as the
`testUserPassword` stack output). It is redacted in normal Pulumi output;
retrieve it with:

```sh
pulumi stack output testUserPassword --show-secrets
```

Env knobs (see `examples/yaml/e2e.sh`):

- `MOX_E2E_REUSE=1` — reuse an already-running `localserve` on `:1080` instead
  of starting (and stopping) one. The script never stops a `localserve` it did
  not start. A pre-existing `example.com` makes `pulumi up` fail.
- `MOX_E2E_KEEP=1` — leave the stack up and a self-started `localserve` running
  for inspection (skips teardown); prints how to tear down manually.

## Open items

These are left for the implementer:

- **DNS invoke:** consider exposing a `getDomainRecords` invoke in addition to
  the `Domain.dnsRecords` output.
- **Registry publishing:** the Go SDK is committed + tagged and other languages
  resolve on demand via `pulumi package add`; publishing to PyPI/npm/NuGet or the
  Pulumi registry is still optional and not done.

Done: idempotent Create (adopt on "already exists"), `Update` for
Account/Address, `Read` refresh on all resources, and sherpa `user:*` error
mapping into Pulumi diagnostics.
