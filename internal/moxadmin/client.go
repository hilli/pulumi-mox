// Package moxadmin is a small client for the mox admin "sherpa" JSON-RPC API.
//
// Transport contract (verified against ../mox/webadmin/admin.js and webauth):
//
//   - Every call is POST {AdminURL}/admin/api/<Method> with header
//     Content-Type: application/json and body {"params":[<positional args>]}.
//   - Success responses are {"result": <value>}; errors are
//     {"error": {"code": "...", "message": "..."}}. User-triggered errors use a
//     "user:" code prefix (e.g. "user:badLogin").
//   - Auth is session/cookie + CSRF, NOT HTTP Basic:
//   - POST /admin/api/LoginPrep with {"params":[]} -> returns a loginToken
//     and sets cookie "webadminlogin".
//   - POST /admin/api/Login with {"params":[loginToken, password]} (the
//     "webadminlogin" cookie must be sent back) -> returns a CSRF token and
//     sets cookie "webadminsession".
//   - Subsequent calls carry the "webadminsession" cookie (handled by the
//     cookie jar) plus header "x-mox-csrf: <csrfToken>".
//
// The admin URL is the base of the mox admin interface (so
// {AdminURL}/admin/api/<Method>).
package moxadmin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
)

// csrfHeader is the header name mox expects the CSRF token in for API POSTs.
const csrfHeader = "x-mox-csrf"

// Config holds the connection settings for the mox admin API.
type Config struct {
	// AdminURL is the base URL of the mox admin interface, e.g.
	// "https://mox-admin.example.com". The "/admin/api/<Method>" suffix is
	// appended per call.
	AdminURL string
	// AdminPassword is the mox admin password used for the LoginPrep/Login flow.
	AdminPassword string
	// InsecureSkipVerify disables TLS certificate verification. Intended only
	// for dev/self-signed setups; leave false in production.
	InsecureSkipVerify bool
}

// Client is a session-authenticated client for the mox admin sherpa API. It is
// safe for concurrent use; the login flow is performed lazily on first call and
// guarded by a mutex.
type Client struct {
	cfg  Config
	http *http.Client

	mu    sync.Mutex
	csrf  string // CSRF token captured from Login; empty until logged in.
	ready bool   // true once a successful Login has populated the cookie jar.
}

// apiError is the sherpa error envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error reports a sherpa-level error returned by the mox admin API.
type Error struct {
	Method string
	Code   string
	Msg    string
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("mox admin %s: %s (%s)", e.Method, e.Msg, e.Code)
	}
	return fmt.Sprintf("mox admin %s: %s", e.Method, e.Msg)
}

// IsUserError reports whether the error is a sherpa "user:" error (a request
// the caller got wrong, e.g. bad login or duplicate domain), as opposed to a
// server/transport failure.
func (e *Error) IsUserError() bool {
	return strings.HasPrefix(e.Code, "user:")
}

// IsAlreadyExists reports whether err is a sherpa user error indicating the
// object the caller tried to create already exists. mox surfaces these as
// "user:" errors whose message mentions that the account/address is already
// present, so Create can treat them as success (idempotency).
func IsAlreadyExists(err error) bool {
	var me *Error
	if !errors.As(err, &me) || !me.IsUserError() {
		return false
	}
	return strings.Contains(strings.ToLower(me.Msg), "already")
}

// IsUserError reports whether err wraps a sherpa "user:" error. It is the
// package-level counterpart to (*Error).IsUserError, letting callers classify
// an error value (e.g. to treat a missing object during Read as drift rather
// than a hard failure) without a manual errors.As.
func IsUserError(err error) bool {
	var me *Error
	if errors.As(err, &me) {
		return me.IsUserError()
	}
	return false
}

// New constructs a Client. It returns an error only if the cookie jar cannot be
// created; no network calls are made here (login is lazy).
func New(cfg Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for dev/self-signed.
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Jar:       jar,
			Transport: transport,
		},
	}, nil
}

// post performs a single sherpa POST and decodes the {"result"|"error"}
// envelope into out (a pointer, or nil to discard the result). withCSRF
// controls whether the captured CSRF header is attached.
func (c *Client) post(ctx context.Context, method string, params []any, withCSRF bool, out any) error {
	body, err := json.Marshal(struct {
		Params []any `json:"params"`
	}{Params: params})
	if err != nil {
		return fmt.Errorf("marshaling params for %s: %w", method, err)
	}

	url := strings.TrimRight(c.cfg.AdminURL, "/") + "/admin/api/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request for %s: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if withCSRF && c.csrf != "" {
		req.Header.Set(csrfHeader, c.csrf)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", method, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading %s response: %w", method, err)
	}

	if resp.StatusCode != http.StatusOK {
		// Sherpa returns 200 even for application errors; a non-200 here is a
		// transport/server problem.
		return &Error{Method: method, Code: fmt.Sprintf("http:%d", resp.StatusCode), Msg: strings.TrimSpace(string(raw))}
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *apiError       `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decoding %s response: %w", method, err)
	}
	if envelope.Error != nil {
		return &Error{Method: method, Code: envelope.Error.Code, Msg: envelope.Error.Message}
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("unmarshaling %s result: %w", method, err)
		}
	}
	return nil
}

// login runs LoginPrep + Login and captures the CSRF token. Caller must hold mu.
func (c *Client) login(ctx context.Context) error {
	var loginToken string
	if err := c.post(ctx, "LoginPrep", []any{}, false, &loginToken); err != nil {
		return err
	}
	var csrf string
	if err := c.post(ctx, "Login", []any{loginToken, c.cfg.AdminPassword}, false, &csrf); err != nil {
		return err
	}
	c.csrf = csrf
	c.ready = true
	return nil
}

// ensureLogin logs in if not already authenticated. Caller must hold mu.
func (c *Client) ensureLogin(ctx context.Context) error {
	if c.ready {
		return nil
	}
	return c.login(ctx)
}

// Call invokes an admin sherpa method with positional params, logging in lazily
// if needed. The result is decoded into out (pass nil to discard). On a sherpa
// session error it retries once after re-authenticating.
func (c *Client) Call(ctx context.Context, method string, params []any, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureLogin(ctx); err != nil {
		return err
	}

	err := c.post(ctx, method, params, true, out)
	if err == nil {
		return nil
	}

	// If the session expired, drop it and try one fresh login.
	if me, ok := err.(*Error); ok && isSessionError(me) {
		c.ready = false
		c.csrf = ""
		if lerr := c.login(ctx); lerr != nil {
			return lerr
		}
		return c.post(ctx, method, params, true, out)
	}
	return err
}

// isSessionError reports whether an error indicates an invalid/expired session
// that warrants a re-login.
func isSessionError(e *Error) bool {
	return strings.Contains(e.Code, "badAuth") ||
		strings.Contains(e.Code, "noAuth") ||
		strings.Contains(e.Code, "badSession") ||
		strings.Contains(e.Code, "csrf")
}

// Logout invalidates the current session, if any.
func (c *Client) Logout(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.ready {
		return nil
	}
	err := c.post(ctx, "Logout", []any{}, true, nil)
	c.ready = false
	c.csrf = ""
	return err
}

// --- Typed wrappers for the admin RPC methods used by the provider ---
//
// Param ordering mirrors ../mox/webadmin/admin.go (the leading ctx argument is
// server-side only and not sent over the wire).

// DomainAdd adds a new domain with an initial account + localpart.
// admin.go: DomainAdd(disabled bool, domain, accountName, localpart string).
func (c *Client) DomainAdd(ctx context.Context, disabled bool, domain, accountName, localpart string) error {
	return c.Call(ctx, "DomainAdd", []any{disabled, domain, accountName, localpart}, nil)
}

// DomainRemove removes an existing domain.
func (c *Client) DomainRemove(ctx context.Context, domain string) error {
	return c.Call(ctx, "DomainRemove", []any{domain}, nil)
}

// DomainRecords returns zone-file lines describing the DNS records that should
// exist for the domain. Only complete AFTER DomainAdd (DKIM keys are generated
// on add).
func (c *Client) DomainRecords(ctx context.Context, domain string) ([]string, error) {
	var records []string
	if err := c.Call(ctx, "DomainRecords", []any{domain}, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// DomainName is mox's representation of a domain name, carrying both the ASCII
// (IDNA/punycode) and Unicode forms. It is the "Domain" field of a ConfigDomain.
type DomainName struct {
	ASCII   string `json:"ASCII"`
	Unicode string `json:"Unicode"`
}

// Domain mirrors the subset of mox's ConfigDomain that the provider reads back
// (the type returned by the Domains() admin method). The full mox struct has
// many more fields (DKIM, DMARC, routes, aliases, ...); unknown JSON keys are
// ignored. Note the domain *name* lives in the nested Name field, not at the
// top level.
type Domain struct {
	Disabled    bool       `json:"Disabled"`
	Description string     `json:"Description"`
	Name        DomainName `json:"Domain"`
}

// Domains returns all configured domains.
func (c *Client) Domains(ctx context.Context) ([]Domain, error) {
	var domains []Domain
	if err := c.Call(ctx, "Domains", []any{}, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// AccountAdd adds a new account with an initial address.
func (c *Client) AccountAdd(ctx context.Context, accountName, address string) error {
	return c.Call(ctx, "AccountAdd", []any{accountName, address}, nil)
}

// AccountRemove removes an account.
func (c *Client) AccountRemove(ctx context.Context, accountName string) error {
	return c.Call(ctx, "AccountRemove", []any{accountName}, nil)
}

// AddressAdd adds an email address to an existing account.
func (c *Client) AddressAdd(ctx context.Context, address, accountName string) error {
	return c.Call(ctx, "AddressAdd", []any{address, accountName}, nil)
}

// AddressRemove removes an email address.
func (c *Client) AddressRemove(ctx context.Context, address string) error {
	return c.Call(ctx, "AddressRemove", []any{address}, nil)
}

// SetPassword sets an account's password (mox requires at least 8 characters).
func (c *Client) SetPassword(ctx context.Context, accountName, password string) error {
	return c.Call(ctx, "SetPassword", []any{accountName, password}, nil)
}

// Account mirrors the subset of mox's config.Account that the provider reads
// back. The full mox struct has many more fields (full name, quota, routes,
// aliases, ...); unknown JSON keys are ignored. Destinations is keyed by the
// account's configured email addresses (a key without a localpart, e.g.
// "@example.com", is a catchall).
type Account struct {
	Destinations map[string]json.RawMessage `json:"Destinations"`
}

// Addresses returns the email addresses configured on the account, derived from
// the Destinations map keys.
func (a Account) Addresses() []string {
	addrs := make([]string, 0, len(a.Destinations))
	for addr := range a.Destinations {
		addrs = append(addrs, addr)
	}
	return addrs
}

// HasAddress reports whether the account has the given address configured,
// matching case-insensitively.
func (a Account) HasAddress(address string) bool {
	for addr := range a.Destinations {
		if strings.EqualFold(addr, address) {
			return true
		}
	}
	return false
}

// Accounts returns the names of all configured accounts (all) and the subset
// that are disabled. The sherpa Accounts method returns two values and no
// error, which is encoded on the wire as a two-element JSON array.
func (c *Client) Accounts(ctx context.Context) (all, disabled []string, err error) {
	var arr [2]json.RawMessage
	if err := c.Call(ctx, "Accounts", []any{}, &arr); err != nil {
		return nil, nil, err
	}
	if len(arr[0]) > 0 {
		if err := json.Unmarshal(arr[0], &all); err != nil {
			return nil, nil, fmt.Errorf("decoding Accounts result: %w", err)
		}
	}
	if len(arr[1]) > 0 {
		if err := json.Unmarshal(arr[1], &disabled); err != nil {
			return nil, nil, fmt.Errorf("decoding Accounts result: %w", err)
		}
	}
	return all, disabled, nil
}

// Account returns the configuration for a single account by name. The sherpa
// Account method returns the account config plus a disk-usage int and no error,
// encoded on the wire as a two-element JSON array; the disk usage is discarded.
func (c *Client) Account(ctx context.Context, name string) (Account, error) {
	var arr [2]json.RawMessage
	if err := c.Call(ctx, "Account", []any{name}, &arr); err != nil {
		return Account{}, err
	}
	var acc Account
	if len(arr[0]) > 0 {
		if err := json.Unmarshal(arr[0], &acc); err != nil {
			return Account{}, fmt.Errorf("decoding Account result: %w", err)
		}
	}
	return acc, nil
}

// FindAddressAccount returns the name of the account that the given address is
// routed to, searching all configured accounts. The boolean is false when no
// account routes the address.
func (c *Client) FindAddressAccount(ctx context.Context, address string) (string, bool, error) {
	all, _, err := c.Accounts(ctx)
	if err != nil {
		return "", false, err
	}
	for _, name := range all {
		acc, err := c.Account(ctx, name)
		if err != nil {
			return "", false, fmt.Errorf("reading account %q: %w", name, err)
		}
		if acc.HasAddress(address) {
			return name, true, nil
		}
	}
	return "", false, nil
}

// AccountExists reports whether an account with the given name is configured.
func (c *Client) AccountExists(ctx context.Context, name string) (bool, error) {
	all, _, err := c.Accounts(ctx)
	if err != nil {
		return false, err
	}
	for _, a := range all {
		if a == name {
			return true, nil
		}
	}
	return false, nil
}
