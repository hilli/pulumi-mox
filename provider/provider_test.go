package provider

import (
	"context"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// TestConfigDiffIgnoresVersionAndInternalKeys is the regression guard for the
// version-bump churn bug: a plugin version bump must NOT replace the provider
// (and therefore every downstream resource) when the real config is unchanged.
//
// infer's default config diff treats infer's internal bookkeeping keys
// ("__pulumi-go-provider-*") and "version" as replacement-forcing once they are
// present in old state. Our custom Config.Diff compares only the typed fields,
// so two identical configs report no changes regardless of plugin version.
func TestConfigDiffIgnoresVersionAndInternalKeys(t *testing.T) {
	cfg := &Config{}
	same := &Config{
		AdminURL:           "https://mox-admin.example.ts.net",
		AdminPassword:      "secret",
		InsecureSkipVerify: boolPtr(false),
	}

	resp, err := cfg.Diff(context.Background(), infer.DiffRequest[*Config, *Config]{
		ID:     "mox",
		State:  same,
		Inputs: same,
	})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if resp.HasChanges {
		t.Fatalf("Diff() reported changes for identical config: %+v", resp.DetailedDiff)
	}
	if len(resp.DetailedDiff) != 0 {
		t.Fatalf("Diff() detailedDiff = %v, want empty", resp.DetailedDiff)
	}
}

func TestConfigDiffClassifiesFieldChanges(t *testing.T) {
	base := &Config{
		AdminURL:           "https://old.example.ts.net",
		AdminPassword:      "old",
		InsecureSkipVerify: boolPtr(false),
	}
	tests := []struct {
		name     string
		newCfg   *Config
		wantKey  string
		wantKind p.DiffKind
	}{
		{
			name:     "adminUrl change replaces",
			newCfg:   &Config{AdminURL: "https://new.example.ts.net", AdminPassword: "old", InsecureSkipVerify: boolPtr(false)},
			wantKey:  "adminUrl",
			wantKind: p.UpdateReplace,
		},
		{
			name:     "insecureSkipVerify change replaces",
			newCfg:   &Config{AdminURL: "https://old.example.ts.net", AdminPassword: "old", InsecureSkipVerify: boolPtr(true)},
			wantKey:  "insecureSkipVerify",
			wantKind: p.UpdateReplace,
		},
		{
			name:     "adminPassword change updates in place",
			newCfg:   &Config{AdminURL: "https://old.example.ts.net", AdminPassword: "new", InsecureSkipVerify: boolPtr(false)},
			wantKey:  "adminPassword",
			wantKind: p.Update,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{}
			resp, err := cfg.Diff(context.Background(), infer.DiffRequest[*Config, *Config]{
				ID:     "mox",
				State:  base,
				Inputs: tc.newCfg,
			})
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if !resp.HasChanges {
				t.Fatalf("Diff() reported no changes, want a change on %q", tc.wantKey)
			}
			pd, ok := resp.DetailedDiff[tc.wantKey]
			if !ok {
				t.Fatalf("Diff() detailedDiff = %v, missing key %q", resp.DetailedDiff, tc.wantKey)
			}
			if pd.Kind != tc.wantKind {
				t.Fatalf("Diff() %q kind = %q, want %q", tc.wantKey, pd.Kind, tc.wantKind)
			}
		})
	}
}
