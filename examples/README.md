# Examples

This directory contains two YAML examples:

- `yaml/` is the runnable smoke example used by `make e2e`.
- `yaml-advanced/` is a reference example for real deployments.

## Runnable smoke example: `yaml/`

`examples/yaml/Pulumi.yaml` is designed to run against `mox localserve`. It uses
only config that exists in a throwaway localserve instance, while still exercising
the provider's main resources: domains, accounts, addresses/aliases, Sieve,
global routes, webserver config, log levels, DNSBL monitoring, and read-only
update-check status.

From the repository root:

```sh
make install_plugin
cd examples/yaml
pulumi stack init dev
pulumi config set mox:adminUrl http://localhost:1080
pulumi config set --secret mox:adminPassword moxadmin
pulumi up
```

Normally you do not need to run those steps by hand. The e2e helper starts or
reuses `mox localserve`, creates a temporary Pulumi backend/stack, applies the
example, performs a second update to exercise Sieve rename, prints key outputs,
and destroys the stack:

```sh
make e2e
```

If a localserve instance is already listening on `http://localhost:1080`, run:

```sh
MOX_E2E_REUSE=1 make e2e
```

## Reference example: `yaml-advanced/`

`examples/yaml-advanced/Pulumi.yaml` is not meant to run unchanged against
`mox localserve`. It demonstrates realistic production shapes, including:

- non-empty account, domain, and global routes;
- references to named transports that must already exist in static `mox.conf`;
- MTA-STS, DMARC, TLSRPT, and Sieve policy settings;
- web redirects, internal handlers, reverse proxying, and static file serving;
- log-level overrides and DNSBL monitoring.

Use it as a copy/paste starting point for a real Mox deployment, then replace
domains, transport names, filesystem paths, and policy values with your own.

To make `yaml-advanced/` executable as an e2e test, we would need a dedicated
Mox fixture with a custom static config defining the referenced transports and
filesystem paths. The current localserve e2e intentionally avoids that extra
fixture and stays lightweight.
