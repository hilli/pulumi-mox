package moxapi

import (
	"encoding/json"
	"testing"
)

// These tests validate that the generated json tags faithfully decode the
// sherpa wire format, including the positional JSON arrays that mox uses for
// methods with multiple return values. There is no live mox server in CI, so
// the wire payloads here are hand-written fixtures shaped like the real API.

// TestVersionArrayDecode mirrors GetVersion: three return values arrive as a
// positional JSON array decoded into [3]json.RawMessage.
func TestVersionArrayDecode(t *testing.T) {
	wire := `["1.2.3", "linux", "amd64"]`

	var arr [3]json.RawMessage
	if err := json.Unmarshal([]byte(wire), &arr); err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}

	var version, goos, goarch string
	for i, dst := range []*string{&version, &goos, &goarch} {
		if err := json.Unmarshal(arr[i], dst); err != nil {
			t.Fatalf("decode element %d: %v", i, err)
		}
	}

	if version != "1.2.3" || goos != "linux" || goarch != "amd64" {
		t.Fatalf("got version=%q goos=%q goarch=%q", version, goos, goarch)
	}
}

// TestConfigFilesArrayDecode mirrors GetConfigFiles: four return values arrive
// as a positional JSON array.
func TestConfigFilesArrayDecode(t *testing.T) {
	wire := `["/etc/mox/mox.conf", "/etc/mox/domains.conf", "static body", "dynamic body"]`

	var arr [4]json.RawMessage
	if err := json.Unmarshal([]byte(wire), &arr); err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}

	var staticPath, dynamicPath, static, dynamic string
	for i, dst := range []*string{&staticPath, &dynamicPath, &static, &dynamic} {
		if err := json.Unmarshal(arr[i], dst); err != nil {
			t.Fatalf("decode element %d: %v", i, err)
		}
	}

	if staticPath != "/etc/mox/mox.conf" || dynamicPath != "/etc/mox/domains.conf" {
		t.Fatalf("got staticPath=%q dynamicPath=%q", staticPath, dynamicPath)
	}
	if static != "static body" || dynamic != "dynamic body" {
		t.Fatalf("got static=%q dynamic=%q", static, dynamic)
	}
}

// TestDomainLocalpartsArrayDecode mirrors GetDomainLocalparts: two return
// values (a map and a map of Alias) arrive as a positional JSON array. This
// exercises the nested Domain field on Alias.
func TestDomainLocalpartsArrayDecode(t *testing.T) {
	wire := `[
		{"postmaster": "admin", "info": "support"},
		{
			"team": {
				"Addresses": ["a@example.com", "b@example.com"],
				"PostPublic": true,
				"ListMembers": false,
				"AllowMsgFrom": true,
				"LocalpartStr": "team",
				"Domain": {"ASCII": "example.com", "Unicode": "example.com"}
			}
		}
	]`

	var arr [2]json.RawMessage
	if err := json.Unmarshal([]byte(wire), &arr); err != nil {
		t.Fatalf("unmarshal array: %v", err)
	}

	var accounts map[string]string
	if err := json.Unmarshal(arr[0], &accounts); err != nil {
		t.Fatalf("decode accounts: %v", err)
	}
	if accounts["postmaster"] != "admin" || accounts["info"] != "support" {
		t.Fatalf("got accounts=%v", accounts)
	}

	var aliases map[string]Alias
	if err := json.Unmarshal(arr[1], &aliases); err != nil {
		t.Fatalf("decode aliases: %v", err)
	}
	team, ok := aliases["team"]
	if !ok {
		t.Fatalf("missing team alias in %v", aliases)
	}
	if !team.PostPublic || !team.AllowMsgFrom || team.ListMembers {
		t.Fatalf("unexpected alias flags: %+v", team)
	}
	if len(team.Addresses) != 2 || team.Addresses[0] != "a@example.com" {
		t.Fatalf("got addresses=%v", team.Addresses)
	}
	if team.Domain.ASCII != "example.com" {
		t.Fatalf("got nested domain=%+v", team.Domain)
	}
}

// TestLoginAttemptsSliceDecode mirrors GetLoginAttempts: a slice return value
// arrives directly (single non-error return), exercising []int and int64
// fields.
func TestLoginAttemptsSliceDecode(t *testing.T) {
	wire := `[
		{
			"Key": [1, 2, 3],
			"Last": "2024-01-02T03:04:05Z",
			"First": "2024-01-01T00:00:00Z",
			"Count": 42,
			"AccountName": "alice",
			"LoginAddress": "alice@example.com",
			"RemoteIP": "203.0.113.7",
			"LocalIP": "198.51.100.3",
			"TLS": "TLS1.3",
			"TLSPubKeyFingerprint": "",
			"Protocol": "imap",
			"UserAgent": "thunderbird",
			"AuthMech": "scram-sha-256",
			"Result": "ok"
		}
	]`

	var attempts []LoginAttempt
	if err := json.Unmarshal([]byte(wire), &attempts); err != nil {
		t.Fatalf("unmarshal slice: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("got %d attempts", len(attempts))
	}
	a := attempts[0]
	if a.Count != 42 {
		t.Fatalf("got count=%d", a.Count)
	}
	if len(a.Key) != 3 || a.Key[2] != 3 {
		t.Fatalf("got key=%v", a.Key)
	}
	if a.AccountName != "alice" || a.Result != "ok" {
		t.Fatalf("got accountName=%q result=%q", a.AccountName, a.Result)
	}
}

// TestNestedMapSliceDecode validates a map-of-slice field (IPRevCheckResult.
// IPNames) and a map field (Dynamic.Domains), the trickiest cardinality cases.
func TestNestedMapSliceDecode(t *testing.T) {
	ipRevWire := `{
		"Hostname": {"ASCII": "mail.example.com", "Unicode": "mail.example.com"},
		"IPNames": {
			"203.0.113.7": ["mail.example.com.", "smtp.example.com."],
			"2001:db8::1": ["mail.example.com."]
		},
		"Errors": [],
		"Warnings": ["minor"],
		"Instructions": []
	}`

	var ipRev IPRevCheckResult
	if err := json.Unmarshal([]byte(ipRevWire), &ipRev); err != nil {
		t.Fatalf("unmarshal IPRevCheckResult: %v", err)
	}
	if ipRev.Hostname.ASCII != "mail.example.com" {
		t.Fatalf("got hostname=%+v", ipRev.Hostname)
	}
	names := ipRev.IPNames["203.0.113.7"]
	if len(names) != 2 || names[1] != "smtp.example.com." {
		t.Fatalf("got ipNames[203.0.113.7]=%v", names)
	}
	if len(ipRev.Warnings) != 1 || ipRev.Warnings[0] != "minor" {
		t.Fatalf("got warnings=%v", ipRev.Warnings)
	}

	dynamicWire := `{
		"Domains": {
			"example.com": {"Disabled": false, "Description": "primary"}
		},
		"WebDomainRedirects": {"old.example.com": "new.example.com"}
	}`

	var dyn Dynamic
	if err := json.Unmarshal([]byte(dynamicWire), &dyn); err != nil {
		t.Fatalf("unmarshal Dynamic: %v", err)
	}
	if _, ok := dyn.Domains["example.com"]; !ok {
		t.Fatalf("missing example.com in %v", dyn.Domains)
	}
	if dyn.WebDomainRedirects["old.example.com"] != "new.example.com" {
		t.Fatalf("got redirects=%v", dyn.WebDomainRedirects)
	}
}
