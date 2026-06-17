package provider

import (
	"reflect"
	"testing"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

func TestWebserverConfigRoundTrip(t *testing.T) {
	in := WebserverConfigArgs{
		Redirects: []WebDomainRedirect{{From: "old.example.com", To: "new.example.com"}},
		Handlers: []WebHandler{{
			LogName:               "app",
			Domain:                "app.example.com",
			PathRegexp:            "^/",
			DontRedirectPlainHTTP: true,
			Compress:              true,
			Forward: &WebForward{
				URL:             "http://127.0.0.1:3000",
				ResponseHeaders: map[string]string{"x-test": "yes"},
			},
		}},
	}

	got := fromMoxWebserverConfig(toMoxWebserverConfig(in))
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", got, in)
	}
}

func TestWebserverConfigFromDNSRedirects(t *testing.T) {
	got := fromMoxWebserverConfig(moxadmin.WebserverConfig{
		WebDNSDomainRedirects: [][2]moxadmin.DomainName{{
			{ASCII: "old.example.com", Unicode: "old.example.com"},
			{ASCII: "new.example.com", Unicode: "new.example.com"},
		}},
	})

	want := []WebDomainRedirect{{From: "old.example.com", To: "new.example.com"}}
	if !reflect.DeepEqual(got.Redirects, want) {
		t.Fatalf("redirects = %#v, want %#v", got.Redirects, want)
	}
}

func TestWebserverChangedNormalizesThroughWireShape(t *testing.T) {
	a := WebserverConfigArgs{Handlers: []WebHandler{{Domain: "example.com", PathRegexp: "^/", Redirect: &WebRedirect{BaseURL: "https://example.org/"}}}}
	b := fromMoxWebserverConfig(toMoxWebserverConfig(a))

	if webserverChanged(a, b) {
		t.Fatalf("equivalent webserver configs reported as changed")
	}
}
