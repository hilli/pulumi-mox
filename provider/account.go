package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// passwordCharset is the alphabet used for provider-generated passwords. It is
// alphanumeric to stay compatible with mox's SetPassword (PRECIS OpaqueString)
// validation.
const passwordCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// generatePassword returns a cryptographically random alphanumeric password of
// the given length.
func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	max := big.NewInt(int64(len(passwordCharset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = passwordCharset[n.Int64()]
	}
	return string(b), nil
}

// Account is a mox account. An account owns one or more email addresses and a
// password used for IMAP/SMTP authentication.
type Account struct{}

// AccountArgs are the user-supplied inputs.
type AccountArgs struct {
	// Account is the account name (mox's account identifier). It is the resource
	// ID, so changing it forces a replacement.
	Account string `pulumi:"account" provider:"replaceOnChanges"`
	// Address is the initial email address for the account, e.g.
	// "user@example.com". mox requires an address when creating an account.
	Address string `pulumi:"address"`
	// Password is an optional account password. When set, the provider calls
	// SetPassword after creating the account. Generation is intentionally left to
	// the caller (e.g. your own provisioning program with random.RandomPassword +
	// write-back to a secret store); the provider stays generation-agnostic.
	Password *string `pulumi:"password,optional" provider:"secret"`

	// MaxOutgoingMessagesPerDay caps the number of messages the account may
	// submit per day. Unset (or 0) uses mox's default. mox stores the
	// message-limit and password-policy settings together, so the provider reads
	// the current values and overrides only the fields you set.
	MaxOutgoingMessagesPerDay *int `pulumi:"maxOutgoingMessagesPerDay,optional"`
	// MaxFirstTimeRecipientsPerDay caps new (first-time) recipients per day.
	// Unset (or 0) uses mox's default.
	MaxFirstTimeRecipientsPerDay *int `pulumi:"maxFirstTimeRecipientsPerDay,optional"`
	// MaxMessageSize is the per-message size quota in bytes. Unset (or 0) uses
	// the global default.
	MaxMessageSize *int64 `pulumi:"maxMessageSize,optional"`
	// FirstTimeSenderDelay, when true (the default), applies mox's delay to
	// messages from first-time senders. Set false to disable the delay.
	FirstTimeSenderDelay *bool `pulumi:"firstTimeSenderDelay,optional"`
	// NoCustomPassword, when true, forbids the account from setting its own
	// password via the account web UI.
	NoCustomPassword *bool `pulumi:"noCustomPassword,optional"`
	// LoginDisabled, when set to a non-empty string, disables login for the
	// account and uses the string as the rejection message shown to clients. An
	// empty string (or unset) leaves login enabled.
	LoginDisabled *string `pulumi:"loginDisabled,optional"`
	// Routes are per-account outgoing routing rules, evaluated before the global
	// routes.
	Routes []AccountRoute `pulumi:"routes,optional"`
}

// AccountRoute is a single per-account outgoing routing rule.
type AccountRoute struct {
	// FromDomains restricts the rule to messages from these sender domains.
	FromDomains []string `pulumi:"fromDomains,optional"`
	// ToDomains restricts the rule to messages to these recipient domains.
	ToDomains []string `pulumi:"toDomains,optional"`
	// MinimumAttempts is the minimum number of earlier delivery attempts before
	// this route is used.
	MinimumAttempts *int `pulumi:"minimumAttempts,optional"`
	// Transport names the transport mox should use for matching messages.
	Transport string `pulumi:"transport"`
}

// AccountState is the checkpointed output state.
type AccountState struct {
	AccountArgs
	// EffectivePassword is the password the account was created with: either the
	// password supplied via the password input, or — when none was supplied — a
	// password the provider generated. It is a secret output so the credential can
	// be propagated to whoever needs it (e.g. via `pulumi stack output
	// effectivePassword --show-secrets`). mox stores passwords hashed, so this only
	// ever reflects a password the provider itself set; a password set out-of-band
	// cannot be recovered, and adopting a pre-existing account leaves it empty.
	EffectivePassword string `pulumi:"effectivePassword" provider:"secret"`
}

// Annotate documents the resource and its fields.
func (a *Account) Annotate(an infer.Annotator) {
	an.Describe(a, "A mox account, owning one or more email addresses.")
}

func (a *AccountArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Account, "Account name (mox's account identifier).")
	an.Describe(&a.Address, "Initial email address for the account, e.g. \"user@example.com\".")
	an.Describe(&a.Password, "Optional account password. When set, it is applied via SetPassword. When omitted on a newly created account, the provider generates one and exposes it as the effectivePassword secret output.")
	an.Describe(&a.MaxOutgoingMessagesPerDay, "Maximum messages the account may submit per day. Unset (or 0) uses mox's default.")
	an.Describe(&a.MaxFirstTimeRecipientsPerDay, "Maximum new (first-time) recipients per day. Unset (or 0) uses mox's default.")
	an.Describe(&a.MaxMessageSize, "Per-message size quota in bytes. Unset (or 0) uses the global default.")
	an.Describe(&a.FirstTimeSenderDelay, "When true (mox's default), apply the delay to messages from first-time senders. Set false to disable it.")
	an.Describe(&a.NoCustomPassword, "When true, forbid the account from setting its own password via the account web UI.")
	an.Describe(&a.LoginDisabled, "When a non-empty string, disable login and use the string as the rejection message. Empty (or unset) leaves login enabled.")
	an.Describe(&a.Routes, "Per-account outgoing routing rules, evaluated before the global routes.")
}

// intOr returns *p when non-nil, otherwise fallback.
func intOr(p *int, fallback int) int {
	if p != nil {
		return *p
	}
	return fallback
}

// int64Or returns *p when non-nil, otherwise fallback.
func int64Or(p *int64, fallback int64) int64 {
	if p != nil {
		return *p
	}
	return fallback
}

// boolOr returns *p when non-nil, otherwise fallback.
func boolOr(p *bool, fallback bool) bool {
	if p != nil {
		return *p
	}
	return fallback
}

// strPtrVal returns *p when non-nil, otherwise "".
func strPtrVal(p *string) string {
	if p != nil {
		return *p
	}
	return ""
}

// hasAccountSettings reports whether any of the five fields written together by
// AccountSettingsSave were supplied.
func hasAccountSettings(args AccountArgs) bool {
	return args.MaxOutgoingMessagesPerDay != nil ||
		args.MaxFirstTimeRecipientsPerDay != nil ||
		args.MaxMessageSize != nil ||
		args.FirstTimeSenderDelay != nil ||
		args.NoCustomPassword != nil
}

// applyAccountSettings reads the account's current settings and writes back the
// five-field bundle, overriding only the fields the user supplied (mox persists
// them atomically). It is a no-op when no settings inputs were given.
func applyAccountSettings(ctx context.Context, client *moxadmin.Client, name string, args AccountArgs) error {
	if !hasAccountSettings(args) {
		return nil
	}
	cur, err := client.Account(ctx, name)
	if err != nil {
		return fmt.Errorf("reading account %q settings: %w", name, err)
	}
	maxOut := intOr(args.MaxOutgoingMessagesPerDay, cur.MaxOutgoingMessagesPerDay)
	maxFirst := intOr(args.MaxFirstTimeRecipientsPerDay, cur.MaxFirstTimeRecipientsPerDay)
	maxMsgSize := int64Or(args.MaxMessageSize, cur.QuotaMessageSize)
	firstTimeSenderDelay := boolOr(args.FirstTimeSenderDelay, !cur.NoFirstTimeSenderDelay)
	noCustomPassword := boolOr(args.NoCustomPassword, cur.NoCustomPassword)
	if err := client.AccountSettingsSave(ctx, name, maxOut, maxFirst, maxMsgSize, firstTimeSenderDelay, noCustomPassword); err != nil {
		return fmt.Errorf("saving account %q settings: %w", name, err)
	}
	return nil
}

// toMoxRoutes converts the resource's route inputs to the client's wire type.
func toMoxRoutes(routes []AccountRoute) []moxadmin.Route {
	out := make([]moxadmin.Route, 0, len(routes))
	for _, r := range routes {
		out = append(out, moxadmin.Route{
			FromDomain:      r.FromDomains,
			ToDomain:        r.ToDomains,
			MinimumAttempts: intOr(r.MinimumAttempts, 0),
			Transport:       r.Transport,
		})
	}
	return out
}

// fromMoxRoutes converts mox's routes to resource inputs. Empty slices and zero
// MinimumAttempts are left nil/unset to avoid nil-vs-empty churn against inputs.
func fromMoxRoutes(routes []moxadmin.Route) []AccountRoute {
	if len(routes) == 0 {
		return nil
	}
	out := make([]AccountRoute, 0, len(routes))
	for _, r := range routes {
		ar := AccountRoute{Transport: r.Transport}
		if len(r.FromDomain) > 0 {
			ar.FromDomains = r.FromDomain
		}
		if len(r.ToDomain) > 0 {
			ar.ToDomains = r.ToDomain
		}
		if r.MinimumAttempts != 0 {
			ma := r.MinimumAttempts
			ar.MinimumAttempts = &ma
		}
		out = append(out, ar)
	}
	return out
}

// routesChanged reports whether two route slices differ after normalization.
// Both sides are round-tripped through the wire/normalized forms so that
// equivalent shapes (nil vs empty slice, an explicit zero MinimumAttempts vs an
// unset one) compare equal and don't trigger redundant AccountRoutesSave calls
// after a refresh.
func routesChanged(inputs, state []AccountRoute) bool {
	return !reflect.DeepEqual(fromMoxRoutes(toMoxRoutes(inputs)), fromMoxRoutes(toMoxRoutes(state)))
}

func (s *AccountState) Annotate(an infer.Annotator) {
	an.Describe(&s.EffectivePassword, "The password the account was created with — the supplied password, or a provider-generated one when none was supplied. Secret output; empty when an existing account was adopted. mox stores passwords hashed, so a password set out-of-band cannot be recovered here.")
}

// WireDependencies marks the generated/echoed password as always secret so its
// value is encrypted in state regardless of how it flows through the engine.
func (a *Account) WireDependencies(f infer.FieldSelector, args *AccountArgs, state *AccountState) {
	f.OutputField(&state.EffectivePassword).AlwaysSecret()
}

// Create adds the account and records the password it was created with. When a
// password is supplied it is applied; when none is supplied and the account is
// brand-new, the provider generates one. An adopted (pre-existing) account is
// never re-passworded.
func (a *Account) Create(ctx context.Context, req infer.CreateRequest[AccountArgs]) (infer.CreateResponse[AccountState], error) {
	state := AccountState{AccountArgs: req.Inputs}

	if req.DryRun {
		return infer.CreateResponse[AccountState]{ID: req.Inputs.Account, Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[AccountState]{}, err
	}

	adopted := false
	if err := client.AccountAdd(ctx, req.Inputs.Account, req.Inputs.Address); err != nil {
		if moxadmin.IsAlreadyExists(err) {
			adopted = true
		} else {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("adding account %q: %w", req.Inputs.Account, err)
		}
	}

	supplied := req.Inputs.Password != nil && *req.Inputs.Password != ""
	switch {
	case supplied:
		if err := client.SetPassword(ctx, req.Inputs.Account, *req.Inputs.Password); err != nil {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("setting password for account %q: %w", req.Inputs.Account, err)
		}
		state.EffectivePassword = *req.Inputs.Password
	case !adopted:
		// Brand-new account without a supplied password: generate one so the
		// account is usable and the credential can be propagated as an output.
		gen, err := generatePassword(24)
		if err != nil {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("generating password for account %q: %w", req.Inputs.Account, err)
		}
		if err := client.SetPassword(ctx, req.Inputs.Account, gen); err != nil {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("setting generated password for account %q: %w", req.Inputs.Account, err)
		}
		state.EffectivePassword = gen
	default:
		// Adopted an existing account and no password was supplied: leave its
		// password untouched (regenerating would clobber the real credential).
	}

	if err := applyAccountSettings(ctx, client, req.Inputs.Account, req.Inputs); err != nil {
		return infer.CreateResponse[AccountState]{}, err
	}

	if req.Inputs.LoginDisabled != nil {
		if err := client.AccountLoginDisabledSave(ctx, req.Inputs.Account, *req.Inputs.LoginDisabled); err != nil {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("saving login-disabled for account %q: %w", req.Inputs.Account, err)
		}
	}

	if req.Inputs.Routes != nil {
		if err := client.AccountRoutesSave(ctx, req.Inputs.Account, toMoxRoutes(req.Inputs.Routes)); err != nil {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("saving routes for account %q: %w", req.Inputs.Account, err)
		}
	}

	return infer.CreateResponse[AccountState]{ID: req.Inputs.Account, Output: state}, nil
}

// Read refreshes state from mox and detects drift. It returns an empty ID when
// the account no longer exists (so Pulumi treats it as deleted). When the
// account exists, the configured address is reconciled from mox's
// Account(name).Destinations so out-of-band address changes are surfaced.
func (a *Account) Read(ctx context.Context, req infer.ReadRequest[AccountArgs, AccountState]) (infer.ReadResponse[AccountArgs, AccountState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[AccountArgs, AccountState]{}, err
	}

	exists, err := client.AccountExists(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[AccountArgs, AccountState]{}, fmt.Errorf("checking account %q: %w", req.ID, err)
	}
	if !exists {
		// Empty ID signals the resource is gone.
		return infer.ReadResponse[AccountArgs, AccountState]{}, nil
	}

	acc, err := client.Account(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[AccountArgs, AccountState]{}, fmt.Errorf("reading account %q: %w", req.ID, err)
	}

	inputs := req.Inputs
	inputs.Account = req.ID
	// If the recorded address is no longer configured on the account, reflect
	// the account's first known address so drift is visible.
	if inputs.Address == "" || !acc.HasAddress(inputs.Address) {
		if addrs := acc.Addresses(); len(addrs) > 0 {
			inputs.Address = addrs[0]
		}
	}

	// Reflect only the settings the user manages (those set in inputs), so mox's
	// unmanaged server defaults don't surface as spurious state.
	if req.Inputs.MaxOutgoingMessagesPerDay != nil {
		v := acc.MaxOutgoingMessagesPerDay
		inputs.MaxOutgoingMessagesPerDay = &v
	}
	if req.Inputs.MaxFirstTimeRecipientsPerDay != nil {
		v := acc.MaxFirstTimeRecipientsPerDay
		inputs.MaxFirstTimeRecipientsPerDay = &v
	}
	if req.Inputs.MaxMessageSize != nil {
		v := acc.QuotaMessageSize
		inputs.MaxMessageSize = &v
	}
	if req.Inputs.FirstTimeSenderDelay != nil {
		v := !acc.NoFirstTimeSenderDelay
		inputs.FirstTimeSenderDelay = &v
	}
	if req.Inputs.NoCustomPassword != nil {
		v := acc.NoCustomPassword
		inputs.NoCustomPassword = &v
	}
	if req.Inputs.LoginDisabled != nil {
		v := acc.LoginDisabled
		inputs.LoginDisabled = &v
	}
	if req.Inputs.Routes != nil {
		routes := fromMoxRoutes(acc.Routes)
		if routes == nil {
			routes = []AccountRoute{}
		}
		inputs.Routes = routes
	}

	state := AccountState{AccountArgs: inputs}
	// mox stores passwords hashed; carry the recorded value forward unchanged.
	state.EffectivePassword = req.State.EffectivePassword
	return infer.ReadResponse[AccountArgs, AccountState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

// Update applies in-place changes that don't require replacing the account:
// rotating the password (SetPassword) and migrating the primary address
// (AddressAdd new, then AddressRemove old). It is a no-op during a dry run.
func (a *Account) Update(ctx context.Context, req infer.UpdateRequest[AccountArgs, AccountState]) (infer.UpdateResponse[AccountState], error) {
	state := AccountState{AccountArgs: req.Inputs}
	// Preserve the recorded password by default; only an explicit rotation changes it.
	state.EffectivePassword = req.State.EffectivePassword

	if req.DryRun {
		return infer.UpdateResponse[AccountState]{Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[AccountState]{}, err
	}

	// Migrate the primary address without replacing the account.
	if req.Inputs.Address != req.State.Address {
		if req.Inputs.Address != "" {
			if err := client.AddressAdd(ctx, req.Inputs.Address, req.State.Account); err != nil && !moxadmin.IsAlreadyExists(err) {
				return infer.UpdateResponse[AccountState]{}, fmt.Errorf("adding address %q to account %q: %w", req.Inputs.Address, req.State.Account, err)
			}
		}
		if req.State.Address != "" {
			if err := client.AddressRemove(ctx, req.State.Address); err != nil {
				return infer.UpdateResponse[AccountState]{}, fmt.Errorf("removing old address %q: %w", req.State.Address, err)
			}
		}
	}

	// Rotate the password when it changed (or was newly set).
	if req.Inputs.Password != nil && *req.Inputs.Password != "" {
		changed := req.State.Password == nil || *req.State.Password != *req.Inputs.Password
		if changed {
			if err := client.SetPassword(ctx, req.State.Account, *req.Inputs.Password); err != nil {
				return infer.UpdateResponse[AccountState]{}, fmt.Errorf("setting password for account %q: %w", req.State.Account, err)
			}
			state.EffectivePassword = *req.Inputs.Password
		}
	}

	// Apply message-limit / password-policy settings when any are managed.
	if err := applyAccountSettings(ctx, client, req.State.Account, req.Inputs); err != nil {
		return infer.UpdateResponse[AccountState]{}, err
	}

	// Apply the login-disabled message when it changed. nil and "" both mean
	// "login enabled", so clearing a previously-set message re-enables login.
	if strPtrVal(req.Inputs.LoginDisabled) != strPtrVal(req.State.LoginDisabled) {
		if err := client.AccountLoginDisabledSave(ctx, req.State.Account, strPtrVal(req.Inputs.LoginDisabled)); err != nil {
			return infer.UpdateResponse[AccountState]{}, fmt.Errorf("saving login-disabled for account %q: %w", req.State.Account, err)
		}
	}

	// Apply routes when they changed. Clearing them (nil) resets the account's
	// routes on the server.
	if routesChanged(req.Inputs.Routes, req.State.Routes) {
		if err := client.AccountRoutesSave(ctx, req.State.Account, toMoxRoutes(req.Inputs.Routes)); err != nil {
			return infer.UpdateResponse[AccountState]{}, fmt.Errorf("saving routes for account %q: %w", req.State.Account, err)
		}
	}

	return infer.UpdateResponse[AccountState]{Output: state}, nil
}

// Delete removes the account.
func (a *Account) Delete(ctx context.Context, req infer.DeleteRequest[AccountState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.AccountRemove(ctx, req.ID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("removing account %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}
