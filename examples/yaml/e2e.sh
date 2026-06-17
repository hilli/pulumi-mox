#!/usr/bin/env bash
# Live end-to-end test of the mox provider against a local `mox localserve`.
#
# Brings up an ephemeral `mox localserve` (admin API on http://localhost:1080),
# stands up the examples/yaml stack against a throwaway file:// Pulumi backend,
# runs `pulumi up`, prints the generated DNS records, then tears everything down.
#
# Requires the plugin to be installed first (the Makefile `e2e` target depends
# on `install_plugin`). mox is taken from PATH, or installed via `go install`.
#
# Env knobs:
#   MOX_E2E_REUSE=1  Reuse an already-running localserve on :1080 instead of
#                    aborting. The script never stops a localserve it did not
#                    start. A pre-existing example.com makes `pulumi up` fail.
#   MOX_E2E_KEEP=1   Leave the stack up and a self-started localserve running
#                    for inspection (skips teardown). Prints how to tear down.
set -euo pipefail

ADMIN_URL="http://localhost:1080"
ADMIN_PASSWORD="moxadmin"
STACK="e2e-$RANDOM"

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # examples/yaml
workdir="$(mktemp -d)"
mox_pid=""
mox_started=0
stack_inited=0
pulumi_yaml_backup=""

log() { printf '==> %s\n' "$*"; }

cleanup() {
  status=$?
  set +e
  if [ "${MOX_E2E_KEEP:-0}" = "1" ]; then
    echo
    log "MOX_E2E_KEEP=1 — leaving resources up for inspection:"
    log "  stack:    $STACK"
    log "  backend:  $PULUMI_BACKEND_URL"
    log "  adminUrl: $ADMIN_URL"
    [ "$mox_started" = "1" ] && log "  mox pid:  $mox_pid (data: $workdir/mox-data, log: $workdir/mox.log)"
    log "  teardown: (cd '$here' && PULUMI_CONFIG_PASSPHRASE='$PULUMI_CONFIG_PASSPHRASE' PULUMI_BACKEND_URL='$PULUMI_BACKEND_URL' pulumi destroy --yes -s $STACK && pulumi stack rm --yes -s $STACK)"
    [ "$mox_started" = "1" ] && log "            kill $mox_pid"
    exit $status
  fi
  if [ "$stack_inited" = "1" ]; then
    ( cd "$here" && pulumi destroy --yes -s "$STACK" )
    ( cd "$here" && pulumi stack rm --yes -s "$STACK" )
  fi
  if [ -n "$pulumi_yaml_backup" ] && [ -f "$pulumi_yaml_backup" ]; then
    cp "$pulumi_yaml_backup" "$here/Pulumi.yaml"
  fi
  rm -f "$here/Pulumi.$STACK.yaml"
  if [ "$mox_started" = "1" ] && [ -n "$mox_pid" ]; then
    kill "$mox_pid" 2>/dev/null
    wait "$mox_pid" 2>/dev/null
  fi
  rm -rf "$workdir"
  exit $status
}
trap cleanup EXIT

admin_up() {
  curl -fsS -o /dev/null -X POST "$ADMIN_URL/admin/api/LoginPrep" \
    -H 'Content-Type: application/json' --data '{"params":[]}' 2>/dev/null
}

# --- ensure mox binary ---
if command -v mox >/dev/null 2>&1; then
  MOX_BIN="$(command -v mox)"
else
  log "mox not on PATH — installing github.com/mjl-/mox@latest ..."
  go install github.com/mjl-/mox@latest
  gobin="$(go env GOBIN)"
  [ -n "$gobin" ] || gobin="$(go env GOPATH)/bin"
  MOX_BIN="$gobin/mox"
fi
[ -x "$MOX_BIN" ] || { echo "mox binary not found/executable: $MOX_BIN" >&2; exit 1; }
log "using mox: $MOX_BIN ($("$MOX_BIN" version 2>&1 | head -n1 || true))"

# --- ensure localserve ---
if admin_up; then
  if [ "${MOX_E2E_REUSE:-0}" = "1" ]; then
    log "reusing already-running localserve on $ADMIN_URL (not managed by this script)"
  else
    cat >&2 <<EOF
A service is already listening on $ADMIN_URL/admin/.
Refusing to start a second 'mox localserve'. Stop the existing one, or re-run
with MOX_E2E_REUSE=1 to run against it (a pre-existing example.com will make
'pulumi up' fail).
EOF
    exit 1
  fi
else
  log "starting 'mox localserve' (data dir: $workdir/mox-data) ..."
  "$MOX_BIN" localserve -dir "$workdir/mox-data" >"$workdir/mox.log" 2>&1 &
  mox_pid=$!
  mox_started=1
  for _ in $(seq 1 50); do
    if ! kill -0 "$mox_pid" 2>/dev/null; then
      echo "mox localserve exited early; log:" >&2; cat "$workdir/mox.log" >&2; exit 1
    fi
    admin_up && break
    sleep 0.2
  done
  if ! admin_up; then
    echo "mox localserve admin API did not come up; log:" >&2; cat "$workdir/mox.log" >&2; exit 1
  fi
  log "localserve admin API is up (pid $mox_pid)"
fi

# --- pulumi up against a throwaway backend ---
export PULUMI_CONFIG_PASSPHRASE="e2e"
mkdir -p "$workdir/state"
export PULUMI_BACKEND_URL="file://$workdir/state"

cd "$here"
pulumi_yaml_backup="$workdir/Pulumi.yaml.orig"
cp "$here/Pulumi.yaml" "$pulumi_yaml_backup"
log "pulumi stack init $STACK"
pulumi stack init "$STACK"
stack_inited=1
pulumi config set mox:adminUrl "$ADMIN_URL"
pulumi config set --secret mox:adminPassword "$ADMIN_PASSWORD"

log "pulumi up"
pulumi up --yes

log "renaming Sieve script through pulumi up"
perl -0pi -e 's/name: forwarding\n/name: forwarding-renamed\n/' "$here/Pulumi.yaml"
pulumi up --yes

echo
log "Domain created. Generated DNS records:"
pulumi stack output dnsRecords
echo
log "Extra account created: $(pulumi stack output testUserAddress)"
log "Generated account password: $(pulumi stack output testUserPassword --show-secrets)"
log "Alias created: $(pulumi stack output aliasAddress)"
log "DKIM selector created: $(pulumi stack output dkimSelector)"
log "DKIM private key file: $(pulumi stack output dkimPrivateKeyFile)"
log "Web redirects: $(pulumi stack output webRedirects)"
log "DNSBL monitoring zones: $(pulumi stack output dnsblZones)"
log "Update checks enabled: $(pulumi stack output checkUpdatesEnabled)"
echo
log "e2e OK"
