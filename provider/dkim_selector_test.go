package provider

import (
	"reflect"
	"testing"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

func boolPtr(v bool) *bool { return &v }

func intPtr(v int) *int { return &v }

func TestSetSigning(t *testing.T) {
	tests := []struct {
		name     string
		sign     []string
		selector string
		enabled  bool
		want     []string
	}{
		{name: "adds missing selector", sign: []string{"old"}, selector: "new", enabled: true, want: []string{"old", "new"}},
		{name: "keeps existing selector once", sign: []string{"old", "new"}, selector: "new", enabled: true, want: []string{"old", "new"}},
		{name: "removes selector", sign: []string{"old", "new"}, selector: "new", enabled: false, want: []string{"old"}},
		{name: "removes duplicates", sign: []string{"new", "old", "new"}, selector: "new", enabled: false, want: []string{"old"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := setSigning(tc.sign, tc.selector, tc.enabled); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("setSigning() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestSelectorToMoxPreservesPrivateKeyAndAppliesInputs(t *testing.T) {
	args := DomainDKIMSelectorArgs{
		Hash:            "sha512",
		HeaderRelaxed:   boolPtr(true),
		BodyRelaxed:     boolPtr(false),
		SealHeaders:     boolPtr(false),
		Headers:         []string{"from", "subject"},
		LifetimeSeconds: intPtr(3600),
	}
	prior := moxadmin.DKIMSelector{PrivateKeyFile: "dkim/test.pem", Algorithm: "ed25519"}

	got := selectorToMox(args, prior)
	if got.PrivateKeyFile != prior.PrivateKeyFile {
		t.Fatalf("PrivateKeyFile = %q, want %q", got.PrivateKeyFile, prior.PrivateKeyFile)
	}
	if got.Hash != "sha512" || !got.Canonicalization.HeaderRelaxed || got.Canonicalization.BodyRelaxed || !got.DontSealHeaders || got.Expiration != "3600s" {
		t.Fatalf("selector settings not applied: %#v", got)
	}
	if !reflect.DeepEqual(got.Headers, args.Headers) {
		t.Fatalf("Headers = %#v, want %#v", got.Headers, args.Headers)
	}
}

func TestSelectorFromMoxPreservesManagedInputs(t *testing.T) {
	inputs := DomainDKIMSelectorArgs{
		Domain:          "example.com",
		Selector:        "sel",
		Hash:            "",
		Algorithm:       "",
		LifetimeSeconds: intPtr(86400),
		Sign:            boolPtr(false),
	}
	moxSel := moxadmin.DKIMSelector{
		Hash:             "sha256",
		HashEffective:    "sha256",
		Canonicalization: moxadmin.Canonicalization{HeaderRelaxed: true, BodyRelaxed: true},
		PrivateKeyFile:   "dkim/sel.pem",
		Algorithm:        "ed25519",
	}

	got := selectorFromMox("example.com", "sel", moxSel, []string{"sel"}, &inputs)
	if got.Hash != "" || got.Algorithm != "" || got.LifetimeSeconds == nil || *got.LifetimeSeconds != 86400 || boolOr(got.Sign, false) {
		t.Fatalf("managed inputs were not preserved: %#v", got.DomainDKIMSelectorArgs)
	}
	if got.PrivateKeyFile != "dkim/sel.pem" || got.EffectiveHash != "sha256" {
		t.Fatalf("outputs not populated: %#v", got)
	}
}

func TestValidateDKIMSelectorArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    DomainDKIMSelectorArgs
		wantErr bool
	}{
		{name: "valid", args: DomainDKIMSelectorArgs{Domain: "example.com", Selector: "sel"}},
		{name: "empty domain", args: DomainDKIMSelectorArgs{Selector: "sel"}, wantErr: true},
		{name: "empty selector", args: DomainDKIMSelectorArgs{Domain: "example.com"}, wantErr: true},
		{name: "negative lifetime", args: DomainDKIMSelectorArgs{Domain: "example.com", Selector: "sel", LifetimeSeconds: intPtr(-1)}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDKIMSelectorArgs(tc.args)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
