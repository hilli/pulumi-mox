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

- [ ] `AccountSettingsSave(accountName, maxOutgoingMessagesPerDay,
      maxFirstTimeRecipientsPerDay, maxMsgSize, firstTimeSenderDelay,
      noCustomPassword)` — quotas / rate limits / max message size.
- [ ] `AccountLoginDisabledSave(accountName, loginDisabled)` — disable login
      while keeping the account (expose as `loginDisabled bool`).
- [ ] `AccountRoutesSave(accountName, routes)` — per-account routing rules.

## 2. New `mox:Alias` resource (aliases / mailing lists)

mox aliases (localpart that fans out to multiple account members) are a distinct
object with their own lifecycle — best modelled as a new resource type.

- [ ] `AliasAdd(aliaslp, domainName, alias)` — Create.
- [ ] `AliasUpdate(aliaslp, domainName, postPublic, listMembers, allowMsgFrom)` —
      Update list behaviour.
- [ ] `AliasRemove(aliaslp, domainName)` — Delete.
- [ ] `AliasAddressesAdd` / `AliasAddressesRemove` — manage member addresses
      (could be sub-fields of `mox:Alias` or a child resource).
- [ ] Read side: `Aliases` / `Alias` for drift detection.

## 3. Expand `mox:Domain` config (or split into child resources)

Domain has many independently-saveable config sections. Small ones fit as
optional fields on `mox:Domain`; larger ones (DKIM keys) may warrant their own
resource.

- [ ] `DomainDKIMAdd` / `DomainDKIMRemove` / `DomainDKIMSave` — DKIM selectors &
      keys. Strong candidate for a dedicated `mox:DomainDKIM` resource.
- [ ] `DomainMTASTSSave` — MTA-STS policy.
- [ ] `DomainTLSRPTAddressSave` — TLS reporting address.
- [ ] `DomainDMARCAddressSave` — DMARC aggregate-report address.
- [ ] `DomainRoutesSave` — per-domain routing.
- [ ] `DomainDescriptionSave` — free-text description.
- [ ] `DomainClientSettingsDomainSave` — autoconfig client-settings domain.
- [ ] `DomainLocalpartConfigSave` — localpart catchall/case rules.
- [ ] `DomainDisabledSave` — enable/disable a domain without removing it.

## 4. Read-only data sources (Pulumi invokes / functions)

These return state with no mutation — expose as invokes (`getX`) rather than
resources.

- [ ] `CheckDomain(domain)` — DNS/config health report (useful as a function).
- [ ] `Domain(domain)` / `DomainConfig(domain)` — full domain config readout.
- [ ] `DomainLocalparts(domain)` — list localparts in a domain.
- [ ] `ClientConfigsDomain(domain)` — autoconfig/autodiscover client settings.
- [ ] `Version()` — server version (handy for diagnostics).
- [ ] `Config()` / `ConfigFiles()` — effective server config.
- [ ] `TLSPublicKeys(accountFullName)` — list TLS client-auth public keys.
- [ ] `LoginAttempts(...)` — recent login attempts (security audit).

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
