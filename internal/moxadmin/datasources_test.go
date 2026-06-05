package moxadmin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient spins up an httptest server that satisfies the lazy
// LoginPrep/Login flow and serves a fixed JSON body for the named method. The
// returned client is pointed at the server.
func newTestClient(t *testing.T, method, methodResult string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/LoginPrep"):
			_, _ = w.Write([]byte(`{"result":"login-token"}`))
		case strings.HasSuffix(r.URL.Path, "/Login"):
			_, _ = w.Write([]byte(`{"result":"csrf-token"}`))
		case strings.HasSuffix(r.URL.Path, "/"+method):
			_, _ = w.Write([]byte(`{"result":` + methodResult + `}`))
		default:
			http.Error(w, "unexpected method "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{AdminURL: srv.URL, AdminPassword: "pw"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestGetVersionCardinality(t *testing.T) {
	tests := []struct {
		name    string
		result  string
		wantErr bool
	}{
		{name: "exact", result: `["1.2.3","linux","amd64"]`, wantErr: false},
		{name: "too few", result: `["1.2.3","linux"]`, wantErr: true},
		{name: "too many", result: `["1.2.3","linux","amd64","extra"]`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, "Version", tc.result)
			version, goos, goarch, err := c.GetVersion(context.Background())
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got version=%q goos=%q goarch=%q", version, goos, goarch)
				}
				if !strings.Contains(err.Error(), "expected 3 values") {
					t.Fatalf("error %q does not mention expected cardinality", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if version != "1.2.3" || goos != "linux" || goarch != "amd64" {
				t.Fatalf("decoded wrong values: %q %q %q", version, goos, goarch)
			}
		})
	}
}
