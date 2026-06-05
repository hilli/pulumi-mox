package moxadmin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hilli/pulumi-mox/internal/moxapi"
)

// This file holds read-only "data source" client methods. They mirror the
// sherpa admin methods that return state without mutating it, and they decode
// into the faithful generated types in package moxapi (kept separate from the
// curated writable types in this package to avoid name collisions).
//
// Methods are named with a Get prefix so they do not clash with the curated
// mutating methods (for example, the Get prefix keeps GetDomainConfig distinct
// from the existing DomainConfig method).

// GetVersion returns the server version, OS, and architecture. The sherpa
// Version method returns three values, encoded on the wire as a JSON array.
func (c *Client) GetVersion(ctx context.Context) (version, goos, goarch string, err error) {
	var arr []json.RawMessage
	if err := c.Call(ctx, "Version", []any{}, &arr); err != nil {
		return "", "", "", err
	}
	if len(arr) != 3 {
		return "", "", "", fmt.Errorf("decoding Version result: expected 3 values, got %d", len(arr))
	}
	for i, dst := range []*string{&version, &goos, &goarch} {
		if len(arr[i]) == 0 {
			continue
		}
		if err := json.Unmarshal(arr[i], dst); err != nil {
			return "", "", "", fmt.Errorf("decoding Version result: %w", err)
		}
	}
	return version, goos, goarch, nil
}

// GetCheckDomain runs DNS and configuration health checks for a domain.
func (c *Client) GetCheckDomain(ctx context.Context, domainName string) (moxapi.CheckResult, error) {
	var r moxapi.CheckResult
	if err := c.Call(ctx, "CheckDomain", []any{domainName}, &r); err != nil {
		return moxapi.CheckResult{}, err
	}
	return r, nil
}

// GetDomain returns DNS-derived details for a domain.
func (c *Client) GetDomain(ctx context.Context, domain string) (moxapi.Domain, error) {
	var r moxapi.Domain
	if err := c.Call(ctx, "Domain", []any{domain}, &r); err != nil {
		return moxapi.Domain{}, err
	}
	return r, nil
}

// GetDomainConfig returns the full effective configuration for a domain.
func (c *Client) GetDomainConfig(ctx context.Context, domain string) (moxapi.ConfigDomain, error) {
	var r moxapi.ConfigDomain
	if err := c.Call(ctx, "DomainConfig", []any{domain}, &r); err != nil {
		return moxapi.ConfigDomain{}, err
	}
	return r, nil
}

// GetDomainLocalparts returns the localpart->account and localpart->alias maps
// for a domain. The sherpa DomainLocalparts method returns two values, encoded
// on the wire as a JSON array.
func (c *Client) GetDomainLocalparts(ctx context.Context, domain string) (localpartAccounts map[string]string, localpartAliases map[string]moxapi.Alias, err error) {
	var arr []json.RawMessage
	if err := c.Call(ctx, "DomainLocalparts", []any{domain}, &arr); err != nil {
		return nil, nil, err
	}
	if len(arr) != 2 {
		return nil, nil, fmt.Errorf("decoding DomainLocalparts result: expected 2 values, got %d", len(arr))
	}
	if len(arr[0]) > 0 {
		if err := json.Unmarshal(arr[0], &localpartAccounts); err != nil {
			return nil, nil, fmt.Errorf("decoding DomainLocalparts accounts: %w", err)
		}
	}
	if len(arr[1]) > 0 {
		if err := json.Unmarshal(arr[1], &localpartAliases); err != nil {
			return nil, nil, fmt.Errorf("decoding DomainLocalparts aliases: %w", err)
		}
	}
	return localpartAccounts, localpartAliases, nil
}

// GetClientConfigsDomain returns autoconfig/autodiscover client settings for a
// domain.
func (c *Client) GetClientConfigsDomain(ctx context.Context, domain string) (moxapi.ClientConfigs, error) {
	var r moxapi.ClientConfigs
	if err := c.Call(ctx, "ClientConfigsDomain", []any{domain}, &r); err != nil {
		return moxapi.ClientConfigs{}, err
	}
	return r, nil
}

// GetConfig returns the effective dynamic server configuration. The result can
// contain secrets and should be treated as sensitive.
func (c *Client) GetConfig(ctx context.Context) (moxapi.Dynamic, error) {
	var r moxapi.Dynamic
	if err := c.Call(ctx, "Config", []any{}, &r); err != nil {
		return moxapi.Dynamic{}, err
	}
	return r, nil
}

// GetConfigFiles returns the static and dynamic config file paths and their raw
// contents. The sherpa ConfigFiles method returns four values, encoded on the
// wire as a JSON array. The file contents can contain secrets.
func (c *Client) GetConfigFiles(ctx context.Context) (staticPath, dynamicPath, static, dynamic string, err error) {
	var arr []json.RawMessage
	if err := c.Call(ctx, "ConfigFiles", []any{}, &arr); err != nil {
		return "", "", "", "", err
	}
	if len(arr) != 4 {
		return "", "", "", "", fmt.Errorf("decoding ConfigFiles result: expected 4 values, got %d", len(arr))
	}
	for i, dst := range []*string{&staticPath, &dynamicPath, &static, &dynamic} {
		if len(arr[i]) == 0 {
			continue
		}
		if err := json.Unmarshal(arr[i], dst); err != nil {
			return "", "", "", "", fmt.Errorf("decoding ConfigFiles result: %w", err)
		}
	}
	return staticPath, dynamicPath, static, dynamic, nil
}

// GetTLSPublicKeys lists TLS client-auth public keys. accountOpt optionally
// restricts the result to a single account; pass an empty string for all.
func (c *Client) GetTLSPublicKeys(ctx context.Context, accountOpt string) ([]moxapi.TLSPublicKey, error) {
	var r []moxapi.TLSPublicKey
	if err := c.Call(ctx, "TLSPublicKeys", []any{accountOpt}, &r); err != nil {
		return nil, err
	}
	return r, nil
}

// GetLoginAttempts returns recent login attempts for an account, most recent
// first, capped at limit entries.
func (c *Client) GetLoginAttempts(ctx context.Context, accountName string, limit int32) ([]moxapi.LoginAttempt, error) {
	var r []moxapi.LoginAttempt
	if err := c.Call(ctx, "LoginAttempts", []any{accountName, limit}, &r); err != nil {
		return nil, err
	}
	return r, nil
}
