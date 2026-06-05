# TODO — mox admin API coverage

The mox admin API (sherpa JSON-RPC, see `../mox/webadmin/api.json`) exposes ~97
methods. This provider currently wraps a baseline of ~14 of them. This file
tracks what we *could* add, grouped by how it maps onto Pulumi's declarative
model.

Legend: `[ ]` not started · `[~]` partial · `[x]` done.

## Already wrapped (baseline)

Auth/session: `LoginPrep`, `Login`, `Logout`.
Domains: `DomainAdd`, `DomainRemove`, `Domains`, `DomainRecords`.
Accounts: `AccountAdd`, `AccountRemove`, `Accounts`, `Account`, `SetPassword`.
Addresses: `AddressAdd`, `AddressRemove`.

Resources today: `mox:Domain`, `mox:Account`, `mox:Address` (all full CRUD).

---

## 1. Expand `mox:Account` with settings fields

These configure an existing account, so they map cleanly onto extra optional
input properties on the current `mox:Account` resource (set on Create, diff on
Update). No new resource type needed.

- [x] `AccountSettingsSave(accountName, maxOutgoingMessagesPerDay,
      maxFirstTimeRecipientsPerDay, maxMsgSize, firstTimeSenderDelay,
      noCustomPassword)` — quotas / rate limits / max message size.
- [x] `AccountLoginDisabledSave(accountName, loginDisabled)` — disable login
      while keeping the account (expose as `loginDisabled bool`).
- [x] `AccountRoutesSave(accountName, routes)` — per-account routing rules.

## 2. New `mox:Alias` resource (aliases / mailing lists)

mox aliases (localpart that fans out to multiple account members) are a distinct
object with their own lifecycle — best modelled as a new resource type.

- [x] `AliasAdd(aliaslp, domainName, alias)` — Create.
- [x] `AliasUpdate(aliaslp, domainName, postPublic, listMembers, allowMsgFrom)` —
      Update list behaviour.
- [x] `AliasRemove(aliaslp, domainName)` — Delete.
- [x] `AliasAddressesAdd` / `AliasAddressesRemove` — manage member addresses
      (could be sub-fields of `mox:Alias` or a child resource).
- [x] Read side: `Aliases` / `Alias` for drift detection.

## 3. Expand `mox:Domain` config (or split into child resources)

Domain has many independently-saveable config sections. Small ones fit as
optional fields on `mox:Domain`; larger ones (DKIM keys) may warrant their own
resource.

- [ ] `DomainDKIMAdd` / `DomainDKIMRemove` / `DomainDKIMSave` — DKIM selectors &
      keys. Strong candidate for a dedicated `mox:DomainDKIM` resource. (Deferred.)
- [x] `DomainMTASTSSave` — MTA-STS policy (`mtaSts` field + `Update`).
- [x] `DomainTLSRPTAddressSave` — TLS reporting address (`tlsRpt` field + `Update`).
- [x] `DomainDMARCAddressSave` — DMARC aggregate-report address (`dmarc` field + `Update`).
- [x] `DomainRoutesSave` — per-domain routing (`routes` field + `Update`).
- [x] `DomainDescriptionSave` — free-text description (`description` field + `Update`).
- [x] `DomainClientSettingsDomainSave` — autoconfig client-settings domain
      (`clientSettingsDomain` field + `Update`).
- [x] `DomainLocalpartConfigSave` — localpart catchall/case rules
      (`localpartConfig` field + `Update`).
- [x] `DomainDisabledSave` — enable/disable a domain without removing it
      (`disabled` now mutable via `Update`).

## 4. Read-only data sources (Pulumi invokes / functions)

These return state with no mutation — expose as invokes (`getX`) rather than
resources.

- [x] `CheckDomain(domain)` — DNS/config health report (useful as a function).
- [x] `Domain(domain)` / `DomainConfig(domain)` — full domain config readout.
- [x] `DomainLocalparts(domain)` — list localparts in a domain.
- [x] `ClientConfigsDomain(domain)` — autoconfig/autodiscover client settings.
- [x] `Version()` — server version (handy for diagnostics).
- [x] `Config()` / `ConfigFiles()` — effective server config.
- [x] `TLSPublicKeys(accountFullName)` — list TLS client-auth public keys.
- [x] `LoginAttempts(...)` — recent login attempts (security audit).

## 5. Out of scope (operational / runtime)

These are runtime operations or monitoring surfaces that don't fit a
declarative, desired-state provider. Listed for completeness; revisit only if
there's a concrete use case.

- Queue management: `QueueList`, `QueueHold*`, `QueueFail`, `QueueDrop`,
  `QueueRequeue`, `QueueTransportSet`, `QueueSuppress*`, etc.
- Webhooks/hooks: `Hook*` (incoming/outgoing webhook management & retries).
- Reporting: DMARC / TLSRPT aggregate report viewing & suppression lists.
- DNSBL monitoring, log-level control, webserver/transport config, TLS public
  key add/remove for client auth.
