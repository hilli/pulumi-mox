// This file declares the provider's read-only data sources as Pulumi functions
// (invokes). Each function wraps a read-only mox admin "sherpa" method via the
// Get-prefixed client methods in internal/moxadmin/datasources.go and projects
// the faithful generated types from internal/moxapi into the schema.
//
// infer derives each function token from the struct name by lowercasing the
// first letter (GetCheckDomain -> getCheckDomain). It reads only the pulumi and
// provider struct tags; the json tags on moxapi types are used purely for the
// sherpa wire decode and are ignored by infer.
package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxapi"
)

// --- getVersion ---------------------------------------------------------------

// GetVersion reports the mox server version and build platform.
type GetVersion struct{}

// GetVersionArgs takes no inputs.
type GetVersionArgs struct{}

// GetVersionResult holds the three scalar return values of the Version method.
type GetVersionResult struct {
	Version string `pulumi:"version"`
	Goos    string `pulumi:"goos"`
	Goarch  string `pulumi:"goarch"`
}

func (f *GetVersion) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the mox server version and the OS/architecture it was built for.")
}

func (r *GetVersionResult) Annotate(a infer.Annotator) {
	a.Describe(&r.Version, "Mox server version string.")
	a.Describe(&r.Goos, "Operating system the server was built for (GOOS).")
	a.Describe(&r.Goarch, "Architecture the server was built for (GOARCH).")
}

func (f *GetVersion) Invoke(ctx context.Context, req infer.FunctionRequest[GetVersionArgs]) (infer.FunctionResponse[GetVersionResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetVersionResult]{}, err
	}
	version, goos, goarch, err := client.GetVersion(ctx)
	if err != nil {
		return infer.FunctionResponse[GetVersionResult]{}, err
	}
	return infer.FunctionResponse[GetVersionResult]{Output: GetVersionResult{Version: version, Goos: goos, Goarch: goarch}}, nil
}

// --- getCheckDomain -----------------------------------------------------------

// GetCheckDomain runs mox's DNS and configuration health checks for a domain.
type GetCheckDomain struct{}

// GetCheckDomainArgs selects the domain to check.
type GetCheckDomainArgs struct {
	Domain string `pulumi:"domain"`
}

// GetCheckDomainResult wraps the full check result.
type GetCheckDomainResult struct {
	Result moxapi.CheckResult `pulumi:"result"`
}

func (f *GetCheckDomain) Annotate(a infer.Annotator) {
	a.Describe(f, "Runs mox's DNS and configuration health checks for a domain (MX, SPF, DKIM, DMARC, TLSRPT, MTA-STS, DANE, ...).")
}

func (a *GetCheckDomainArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Domain name to check, e.g. \"example.com\".")
}

func (f *GetCheckDomain) Invoke(ctx context.Context, req infer.FunctionRequest[GetCheckDomainArgs]) (infer.FunctionResponse[GetCheckDomainResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetCheckDomainResult]{}, err
	}
	res, err := client.GetCheckDomain(ctx, req.Input.Domain)
	if err != nil {
		return infer.FunctionResponse[GetCheckDomainResult]{}, err
	}
	return infer.FunctionResponse[GetCheckDomainResult]{Output: GetCheckDomainResult{Result: res}}, nil
}

// --- getDomain ----------------------------------------------------------------

// GetDomain returns DNS-derived details for a domain.
type GetDomain struct{}

// GetDomainArgs selects the domain.
type GetDomainArgs struct {
	Domain string `pulumi:"domain"`
}

// GetDomainResult wraps the domain details.
type GetDomainResult struct {
	Result moxapi.Domain `pulumi:"result"`
}

func (f *GetDomain) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns DNS-derived details for a domain (the parsed/normalised domain name).")
}

func (a *GetDomainArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Domain name to look up, e.g. \"example.com\".")
}

func (f *GetDomain) Invoke(ctx context.Context, req infer.FunctionRequest[GetDomainArgs]) (infer.FunctionResponse[GetDomainResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetDomainResult]{}, err
	}
	res, err := client.GetDomain(ctx, req.Input.Domain)
	if err != nil {
		return infer.FunctionResponse[GetDomainResult]{}, err
	}
	return infer.FunctionResponse[GetDomainResult]{Output: GetDomainResult{Result: res}}, nil
}

// --- getDomainConfig ----------------------------------------------------------

// GetDomainConfig returns the full effective configuration for a domain.
type GetDomainConfig struct{}

// GetDomainConfigArgs selects the domain.
type GetDomainConfigArgs struct {
	Domain string `pulumi:"domain"`
}

// GetDomainConfigResult wraps the domain configuration.
type GetDomainConfigResult struct {
	Result moxapi.ConfigDomain `pulumi:"result"`
}

func (f *GetDomainConfig) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the full effective mox configuration for a domain, including localpart, DKIM, DMARC, TLSRPT, MTA-STS and routing settings.")
}

func (a *GetDomainConfigArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Domain name whose configuration to return, e.g. \"example.com\".")
}

func (f *GetDomainConfig) Invoke(ctx context.Context, req infer.FunctionRequest[GetDomainConfigArgs]) (infer.FunctionResponse[GetDomainConfigResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetDomainConfigResult]{}, err
	}
	res, err := client.GetDomainConfig(ctx, req.Input.Domain)
	if err != nil {
		return infer.FunctionResponse[GetDomainConfigResult]{}, err
	}
	return infer.FunctionResponse[GetDomainConfigResult]{Output: GetDomainConfigResult{Result: res}}, nil
}

// --- getDomainLocalparts ------------------------------------------------------

// GetDomainLocalparts returns the localpart->account and localpart->alias maps
// for a domain.
type GetDomainLocalparts struct{}

// GetDomainLocalpartsArgs selects the domain.
type GetDomainLocalpartsArgs struct {
	Domain string `pulumi:"domain"`
}

// GetDomainLocalpartsResult holds the two map return values of the
// DomainLocalparts method.
type GetDomainLocalpartsResult struct {
	LocalpartAccounts map[string]string       `pulumi:"localpartAccounts,optional"`
	LocalpartAliases  map[string]moxapi.Alias `pulumi:"localpartAliases,optional"`
}

func (f *GetDomainLocalparts) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the localpart-to-account and localpart-to-alias mappings for a domain.")
}

func (a *GetDomainLocalpartsArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Domain name whose localparts to list, e.g. \"example.com\".")
}

func (r *GetDomainLocalpartsResult) Annotate(a infer.Annotator) {
	a.Describe(&r.LocalpartAccounts, "Map of localpart to the account that owns it.")
	a.Describe(&r.LocalpartAliases, "Map of localpart to its alias definition.")
}

func (f *GetDomainLocalparts) Invoke(ctx context.Context, req infer.FunctionRequest[GetDomainLocalpartsArgs]) (infer.FunctionResponse[GetDomainLocalpartsResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetDomainLocalpartsResult]{}, err
	}
	accounts, aliases, err := client.GetDomainLocalparts(ctx, req.Input.Domain)
	if err != nil {
		return infer.FunctionResponse[GetDomainLocalpartsResult]{}, err
	}
	return infer.FunctionResponse[GetDomainLocalpartsResult]{Output: GetDomainLocalpartsResult{LocalpartAccounts: accounts, LocalpartAliases: aliases}}, nil
}

// --- getClientConfigsDomain ---------------------------------------------------

// GetClientConfigsDomain returns autoconfig/autodiscover client settings for a
// domain.
type GetClientConfigsDomain struct{}

// GetClientConfigsDomainArgs selects the domain.
type GetClientConfigsDomainArgs struct {
	Domain string `pulumi:"domain"`
}

// GetClientConfigsDomainResult wraps the client configuration.
type GetClientConfigsDomainResult struct {
	Result moxapi.ClientConfigs `pulumi:"result"`
}

func (f *GetClientConfigsDomain) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the IMAP/SMTP client autoconfiguration settings (hostnames, ports, TLS) for a domain.")
}

func (a *GetClientConfigsDomainArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Domain name whose client configuration to return, e.g. \"example.com\".")
}

func (f *GetClientConfigsDomain) Invoke(ctx context.Context, req infer.FunctionRequest[GetClientConfigsDomainArgs]) (infer.FunctionResponse[GetClientConfigsDomainResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetClientConfigsDomainResult]{}, err
	}
	res, err := client.GetClientConfigsDomain(ctx, req.Input.Domain)
	if err != nil {
		return infer.FunctionResponse[GetClientConfigsDomainResult]{}, err
	}
	return infer.FunctionResponse[GetClientConfigsDomainResult]{Output: GetClientConfigsDomainResult{Result: res}}, nil
}

// --- getConfig ----------------------------------------------------------------

// GetConfig returns the effective dynamic server configuration. The result can
// contain secrets (for example DKIM private keys) and is marked sensitive.
type GetConfig struct{}

// GetConfigArgs takes no inputs.
type GetConfigArgs struct{}

// GetConfigResult wraps the dynamic configuration. The whole object is marked
// secret because it can embed private key material.
type GetConfigResult struct {
	Result moxapi.Dynamic `pulumi:"result" provider:"secret"`
}

func (f *GetConfig) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the effective dynamic mox configuration. The result is marked secret because it can contain private key material.")
}

func (f *GetConfig) Invoke(ctx context.Context, req infer.FunctionRequest[GetConfigArgs]) (infer.FunctionResponse[GetConfigResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetConfigResult]{}, err
	}
	res, err := client.GetConfig(ctx)
	if err != nil {
		return infer.FunctionResponse[GetConfigResult]{}, err
	}
	return infer.FunctionResponse[GetConfigResult]{Output: GetConfigResult{Result: res}}, nil
}

// --- getConfigFiles -----------------------------------------------------------

// GetConfigFiles returns the static and dynamic config file paths and their raw
// contents.
type GetConfigFiles struct{}

// GetConfigFilesArgs takes no inputs.
type GetConfigFilesArgs struct{}

// GetConfigFilesResult holds the four scalar return values of the ConfigFiles
// method. The file contents are marked secret because they can contain
// credentials and private keys.
type GetConfigFilesResult struct {
	StaticPath  string `pulumi:"staticPath"`
	DynamicPath string `pulumi:"dynamicPath"`
	Static      string `pulumi:"static" provider:"secret"`
	Dynamic     string `pulumi:"dynamic" provider:"secret"`
}

func (f *GetConfigFiles) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns the paths and raw contents of mox's static and dynamic config files. The contents are marked secret.")
}

func (r *GetConfigFilesResult) Annotate(a infer.Annotator) {
	a.Describe(&r.StaticPath, "Filesystem path of the static config file (mox.conf).")
	a.Describe(&r.DynamicPath, "Filesystem path of the dynamic config file (domains.conf).")
	a.Describe(&r.Static, "Raw contents of the static config file.")
	a.Describe(&r.Dynamic, "Raw contents of the dynamic config file.")
}

func (f *GetConfigFiles) Invoke(ctx context.Context, req infer.FunctionRequest[GetConfigFilesArgs]) (infer.FunctionResponse[GetConfigFilesResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetConfigFilesResult]{}, err
	}
	staticPath, dynamicPath, static, dynamic, err := client.GetConfigFiles(ctx)
	if err != nil {
		return infer.FunctionResponse[GetConfigFilesResult]{}, err
	}
	return infer.FunctionResponse[GetConfigFilesResult]{Output: GetConfigFilesResult{
		StaticPath:  staticPath,
		DynamicPath: dynamicPath,
		Static:      static,
		Dynamic:     dynamic,
	}}, nil
}

// --- getTLSPublicKeys ---------------------------------------------------------

// GetTLSPublicKeys lists TLS client-authentication public keys.
type GetTLSPublicKeys struct{}

// GetTLSPublicKeysArgs optionally restricts the result to a single account.
type GetTLSPublicKeysArgs struct {
	Account *string `pulumi:"account,optional"`
}

// GetTLSPublicKeysResult wraps the list of public keys.
type GetTLSPublicKeysResult struct {
	Result []moxapi.TLSPublicKey `pulumi:"result,optional"`
}

func (f *GetTLSPublicKeys) Annotate(a infer.Annotator) {
	a.Describe(f, "Lists TLS client-authentication public keys, optionally restricted to a single account.")
}

func (a *GetTLSPublicKeysArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Account, "Restrict the result to this account. Omit to list keys for all accounts.")
}

func (f *GetTLSPublicKeys) Invoke(ctx context.Context, req infer.FunctionRequest[GetTLSPublicKeysArgs]) (infer.FunctionResponse[GetTLSPublicKeysResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetTLSPublicKeysResult]{}, err
	}
	account := ""
	if req.Input.Account != nil {
		account = *req.Input.Account
	}
	res, err := client.GetTLSPublicKeys(ctx, account)
	if err != nil {
		return infer.FunctionResponse[GetTLSPublicKeysResult]{}, err
	}
	return infer.FunctionResponse[GetTLSPublicKeysResult]{Output: GetTLSPublicKeysResult{Result: res}}, nil
}

// --- getLoginAttempts ---------------------------------------------------------

// GetLoginAttempts returns recent login attempts for an account.
type GetLoginAttempts struct{}

// GetLoginAttemptsArgs selects the account and caps the number of entries.
type GetLoginAttemptsArgs struct {
	Account string `pulumi:"account"`
	Limit   int    `pulumi:"limit,optional"`
}

// GetLoginAttemptsResult wraps the list of login attempts.
type GetLoginAttemptsResult struct {
	Result []moxapi.LoginAttempt `pulumi:"result,optional"`
}

func (f *GetLoginAttempts) Annotate(a infer.Annotator) {
	a.Describe(f, "Returns recent login attempts for an account, most recent first.")
}

func (a *GetLoginAttemptsArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Account, "Account name whose login attempts to return.")
	an.Describe(&a.Limit, "Maximum number of attempts to return, most recent first. Set to 0 (the default) to return all attempts.")
}

func (f *GetLoginAttempts) Invoke(ctx context.Context, req infer.FunctionRequest[GetLoginAttemptsArgs]) (infer.FunctionResponse[GetLoginAttemptsResult], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.FunctionResponse[GetLoginAttemptsResult]{}, err
	}
	res, err := client.GetLoginAttempts(ctx, req.Input.Account, int32(req.Input.Limit))
	if err != nil {
		return infer.FunctionResponse[GetLoginAttemptsResult]{}, err
	}
	return infer.FunctionResponse[GetLoginAttemptsResult]{Output: GetLoginAttemptsResult{Result: res}}, nil
}
