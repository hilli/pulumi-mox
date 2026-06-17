package moxadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestMonitorDNSBLsSavePayload(t *testing.T) {
	var gotParams []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoginPrep"):
			_, _ = w.Write([]byte(`{"result":"login-token"}`))
		case strings.HasSuffix(r.URL.Path, "/Login"):
			_, _ = w.Write([]byte(`{"result":"csrf-token"}`))
		case strings.HasSuffix(r.URL.Path, "/MonitorDNSBLsSave"):
			var body struct {
				Params []any `json:"params"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			gotParams = body.Params
			_, _ = w.Write([]byte(`{"result":null}`))
		default:
			http.Error(w, "unexpected method "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{AdminURL: srv.URL, AdminPassword: "pw"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.MonitorDNSBLsSave(context.Background(), []string{"sbl.example", "bl.example"}); err != nil {
		t.Fatalf("MonitorDNSBLsSave: %v", err)
	}
	if len(gotParams) != 1 || gotParams[0] != "sbl.example\nbl.example" {
		t.Fatalf("params = %#v, want newline-joined zones", gotParams)
	}
}

func TestCheckUpdatesEnabled(t *testing.T) {
	c := newTestClient(t, "CheckUpdatesEnabled", `true`)

	enabled, err := c.CheckUpdatesEnabled(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdatesEnabled: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want true")
	}
}

func TestDomainDKIMAddPayload(t *testing.T) {
	var gotParams []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoginPrep"):
			_, _ = w.Write([]byte(`{"result":"login-token"}`))
		case strings.HasSuffix(r.URL.Path, "/Login"):
			_, _ = w.Write([]byte(`{"result":"csrf-token"}`))
		case strings.HasSuffix(r.URL.Path, "/DomainDKIMAdd"):
			var body struct {
				Params []any `json:"params"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			gotParams = body.Params
			_, _ = w.Write([]byte(`{"result":null}`))
		default:
			http.Error(w, "unexpected method "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{AdminURL: srv.URL, AdminPassword: "pw"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.DomainDKIMAdd(context.Background(), "example.com", "sel", "ed25519", "sha256", true, false, true, []string{"from"}, 3600_000_000_000); err != nil {
		t.Fatalf("DomainDKIMAdd: %v", err)
	}

	want := []any{"example.com", "sel", "ed25519", "sha256", true, false, true, []any{"from"}, float64(3600_000_000_000)}
	if !reflect.DeepEqual(gotParams, want) {
		t.Fatalf("params = %#v, want %#v", gotParams, want)
	}
}
