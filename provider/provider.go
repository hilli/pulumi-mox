// Package provider wires up the native Pulumi "mox" provider.
//
// It uses pulumi-go-provider's infer package to derive the schema and SDKs from
// the Go types declared here. The provider talks to mox's admin "sherpa" API
// (see internal/moxadmin) using session/cookie + CSRF auth.
package provider

import (
	"context"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// Version is the provider/plugin version. It is overridden at build time via
// -ldflags "-X github.com/hilli/pulumi-mox/provider.Version=x.y.z".
var Version = "0.1.0"

// Config holds the provider-level configuration. Values come from the Pulumi
// stack config (mox:adminUrl, mox:adminPassword, mox:insecureSkipVerify) with
// MOX_ADMIN_URL / MOX_ADMIN_PASSWORD environment fallbacks.
//
// It is registered as a pointer (infer.Config(&Config{})) so the pointer-receiver
// Configure method can populate the shared client; resources read a value copy
// via infer.GetConfig[Config], which carries the *moxadmin.Client pointer.
type Config struct {
	// AdminURL is the base URL of the mox admin interface, e.g.
	// "https://mox-admin.example.com". The "/admin/api/<Method>" suffix is
	// appended per call by the client.
	AdminURL string `pulumi:"adminUrl,optional"`
	// AdminPassword is the mox admin password used for the LoginPrep/Login flow.
	AdminPassword string `pulumi:"adminPassword,optional" provider:"secret"`
	// InsecureSkipVerify disables TLS verification (dev/self-signed only).
	InsecureSkipVerify *bool `pulumi:"insecureSkipVerify,optional"`

	// client is the shared admin client built in Configure. It is not a Pulumi
	// field (no pulumi tag) and is read by resources via clientFromContext.
	client *moxadmin.Client
}

// Annotate supplies descriptions and env-var fallbacks for the config fields.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.AdminURL, "Base URL of the mox admin interface, e.g. https://mox-admin.example.com.")
	a.SetDefault(&c.AdminURL, "", "MOX_ADMIN_URL")
	a.Describe(&c.AdminPassword, "Password for the mox admin LoginPrep/Login flow.")
	a.SetDefault(&c.AdminPassword, "", "MOX_ADMIN_PASSWORD")
	a.Describe(&c.InsecureSkipVerify, "Disable TLS certificate verification (dev/self-signed only).")
}

// Configure builds the shared admin client. Login itself is performed lazily by
// the client on the first API call, so `pulumi preview` (which does not invoke
// resource Create/Read/Delete) works even without reaching the server. If
// credentials are absent the client is left nil and resource operations fail
// with a clear message via clientFromContext.
func (c *Config) Configure(_ context.Context) error {
	if c.AdminURL == "" || c.AdminPassword == "" {
		return nil
	}
	insecure := false
	if c.InsecureSkipVerify != nil {
		insecure = *c.InsecureSkipVerify
	}
	client, err := moxadmin.New(moxadmin.Config{
		AdminURL:           c.AdminURL,
		AdminPassword:      c.AdminPassword,
		InsecureSkipVerify: insecure,
	})
	if err != nil {
		return fmt.Errorf("mox provider: building admin client: %w", err)
	}
	c.client = client
	return nil
}

// clientFromContext returns the configured admin client. Resources call this in
// their Create/Read/Delete methods. It returns an actionable error when the
// provider was configured without credentials.
func clientFromContext(ctx context.Context) (*moxadmin.Client, error) {
	cfg := infer.GetConfig[Config](ctx)
	if cfg.client == nil {
		return nil, fmt.Errorf("mox provider: adminUrl and adminPassword (or MOX_ADMIN_URL / MOX_ADMIN_PASSWORD) must be set")
	}
	return cfg.client, nil
}

// Provider builds the configured infer provider.
func Provider() (p.Provider, error) {
	return infer.NewProviderBuilder().
		WithNamespace("hilli").
		WithDisplayName("mox").
		WithDescription("Manage mox mail server domains, accounts and addresses via the admin API.").
		WithConfig(infer.Config(&Config{})).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).
		WithResources(
			infer.Resource(&Domain{}),
			infer.Resource(&Account{}),
			infer.Resource(&Address{}),
			infer.Resource(&Alias{}),
		).
		WithFunctions(
			infer.Function(&GetVersion{}),
			infer.Function(&GetCheckDomain{}),
			infer.Function(&GetDomain{}),
			infer.Function(&GetDomainConfig{}),
			infer.Function(&GetDomainLocalparts{}),
			infer.Function(&GetClientConfigsDomain{}),
			infer.Function(&GetConfig{}),
			infer.Function(&GetConfigFiles{}),
			infer.Function(&GetTLSPublicKeys{}),
			infer.Function(&GetLoginAttempts{}),
		).
		Build()
}
