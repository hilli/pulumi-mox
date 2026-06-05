package provider

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// Domain is the reference resource for the provider. It manages a mox domain and
// exposes the mox-generated DNS records (zone-file lines) as an output so callers
// can program their DNS provider (e.g. Cloudflare).
//
// Adding a domain in mox requires an initial account and localpart; mox creates
// the account if it does not yet exist. The domain name, initial account and
// initial localpart are immutable and force a replacement when changed. The
// remaining configuration fields (description, client-settings domain, MTA-STS,
// DMARC, TLSRPT, localpart config, routes and disabled) are mutable in place via
// Update.
type Domain struct{}

// DomainArgs are the user-supplied inputs.
type DomainArgs struct {
	// Domain is the domain name to add, e.g. "example.com".
	Domain string `pulumi:"domain" provider:"replaceOnChanges"`
	// Account is the initial account name to create/associate with the domain.
	// Defaults to the domain name when omitted.
	Account *string `pulumi:"account,optional" provider:"replaceOnChanges"`
	// Localpart is the local part of the initial address (the part before "@").
	// Defaults to "postmaster" when omitted.
	Localpart *string `pulumi:"localpart,optional" provider:"replaceOnChanges"`
	// Disabled marks the domain as administratively disabled in mox.
	Disabled *bool `pulumi:"disabled,optional"`
	// Description is a free-form description of the domain.
	Description *string `pulumi:"description,optional"`
	// ClientSettingsDomain is the hostname clients should use for IMAP/SMTP
	// autoconfiguration.
	ClientSettingsDomain *string `pulumi:"clientSettingsDomain,optional"`
	// MtaSts configures the domain's MTA-STS policy. Unset leaves it unmanaged;
	// removing a previously-set block clears the policy.
	MtaSts *DomainMtaSts `pulumi:"mtaSts,optional"`
	// Dmarc configures the DMARC aggregate-report destination. Removing a
	// previously-set block clears the DMARC config.
	Dmarc *DomainReportAddress `pulumi:"dmarc,optional"`
	// TlsRpt configures the TLSRPT report destination. Removing a
	// previously-set block clears the TLSRPT config.
	TlsRpt *DomainReportAddress `pulumi:"tlsRpt,optional"`
	// LocalpartConfig configures catch-all separators and case sensitivity.
	LocalpartConfig *DomainLocalpartConfig `pulumi:"localpartConfig,optional"`
	// Routes are the domain's outgoing routing rules.
	Routes []AccountRoute `pulumi:"routes,optional"`
}

// DomainMtaSts is the domain's MTA-STS policy configuration.
type DomainMtaSts struct {
	// PolicyID identifies the policy version; changing it signals an updated
	// policy to senders.
	PolicyID string `pulumi:"policyId"`
	// Mode is the MTA-STS mode: "enforce", "testing" or "none".
	Mode string `pulumi:"mode"`
	// MaxAgeSeconds is the policy lifetime in seconds.
	MaxAgeSeconds int `pulumi:"maxAgeSeconds"`
	// Mx lists the permitted MX host patterns.
	Mx []string `pulumi:"mx,optional"`
}

// DomainReportAddress is a reporting-address destination, shared by the DMARC
// and TLSRPT settings.
type DomainReportAddress struct {
	// Localpart is the local part of the report destination address.
	Localpart string `pulumi:"localpart"`
	// Domain is the report destination's domain. Empty (the default) means the
	// domain itself.
	Domain string `pulumi:"domain,optional"`
	// Account is the mox account that receives the reports.
	Account string `pulumi:"account"`
	// Mailbox is the destination mailbox within the account.
	Mailbox string `pulumi:"mailbox,optional"`
}

// DomainLocalpartConfig configures localpart handling for the domain.
type DomainLocalpartConfig struct {
	// CatchallSeparators are the separators used for sub-addressing (e.g. "+").
	CatchallSeparators []string `pulumi:"catchallSeparators,optional"`
	// CaseSensitive makes localparts case-sensitive when true.
	CaseSensitive bool `pulumi:"caseSensitive,optional"`
}

// DomainState is the checkpointed output state.
type DomainState struct {
	DomainArgs
	// DnsRecords are the zone-file lines mox expects to exist for this domain
	// (MX, SPF, DKIM, DMARC, MTA-STS, TLSRPT, ...). Only complete after the
	// domain has been added (DKIM keys are generated on add).
	DnsRecords []string `pulumi:"dnsRecords"`
}

// Annotate documents the resource and its fields.
func (d *Domain) Annotate(a infer.Annotator) {
	a.Describe(d, "A mail domain managed by mox. Exposes the mox-generated DNS records as an output.")
}

func (a *DomainArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Domain name to add, e.g. \"example.com\".")
	an.Describe(&a.Account, "Initial account name to create/associate. Defaults to the domain name.")
	an.Describe(&a.Localpart, "Local part of the initial address (before \"@\"). Defaults to \"postmaster\".")
	an.Describe(&a.Disabled, "Whether the domain is administratively disabled in mox.")
	an.Describe(&a.Description, "Free-form description of the domain.")
	an.Describe(&a.ClientSettingsDomain, "Hostname clients should use for IMAP/SMTP autoconfiguration.")
	an.Describe(&a.MtaSts, "MTA-STS policy configuration. Removing a previously-set block clears the policy.")
	an.Describe(&a.Dmarc, "DMARC aggregate-report destination. Removing a previously-set block clears it.")
	an.Describe(&a.TlsRpt, "TLSRPT report destination. Removing a previously-set block clears it.")
	an.Describe(&a.LocalpartConfig, "Localpart catch-all separators and case-sensitivity.")
	an.Describe(&a.Routes, "Domain-level outgoing routing rules.")
}

func (m *DomainMtaSts) Annotate(an infer.Annotator) {
	an.Describe(&m.PolicyID, "Policy version identifier; change it to signal an updated policy to senders.")
	an.Describe(&m.Mode, "MTA-STS mode: \"enforce\", \"testing\" or \"none\".")
	an.Describe(&m.MaxAgeSeconds, "Policy lifetime in seconds.")
	an.Describe(&m.Mx, "Permitted MX host patterns.")
}

func (r *DomainReportAddress) Annotate(an infer.Annotator) {
	an.Describe(&r.Localpart, "Local part of the report destination address.")
	an.Describe(&r.Domain, "Report destination domain. Empty means the domain itself.")
	an.Describe(&r.Account, "Mox account that receives the reports.")
	an.Describe(&r.Mailbox, "Destination mailbox within the account.")
}

func (l *DomainLocalpartConfig) Annotate(an infer.Annotator) {
	an.Describe(&l.CatchallSeparators, "Separators used for sub-addressing, e.g. \"+\".")
	an.Describe(&l.CaseSensitive, "Whether localparts are case-sensitive.")
}

func (s *DomainState) Annotate(an infer.Annotator) {
	an.Describe(&s.DnsRecords, "Zone-file lines mox expects to exist for this domain (MX, SPF, DKIM, DMARC, ...).")
}

// resolveDomainInputs applies the documented defaults.
func resolveDomainInputs(in DomainArgs) (account, localpart string, disabled bool) {
	account = in.Domain
	if in.Account != nil && *in.Account != "" {
		account = *in.Account
	}
	localpart = "postmaster"
	if in.Localpart != nil && *in.Localpart != "" {
		localpart = *in.Localpart
	}
	if in.Disabled != nil {
		disabled = *in.Disabled
	}
	return account, localpart, disabled
}

// toNanos converts seconds to nanoseconds (a Go time.Duration value).
func toNanos(seconds int) int64 {
	return int64(seconds) * int64(time.Second)
}

// fromNanos converts nanoseconds to whole seconds (sub-second parts truncate).
func fromNanos(ns int64) int {
	return int(ns / int64(time.Second))
}

// stringsEqual reports whether two string slices are equal, treating nil and
// empty as equal.
func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// validateMtaSts fail-fast validates an MTA-STS block before any RPC.
func validateMtaSts(m *DomainMtaSts) error {
	if m == nil {
		return nil
	}
	if m.PolicyID == "" {
		return fmt.Errorf("mtaSts.policyId must not be empty")
	}
	switch m.Mode {
	case "enforce", "testing", "none":
	default:
		return fmt.Errorf("mtaSts.mode must be one of enforce, testing, none (got %q)", m.Mode)
	}
	if m.MaxAgeSeconds <= 0 {
		return fmt.Errorf("mtaSts.maxAgeSeconds must be positive (got %d)", m.MaxAgeSeconds)
	}
	if int64(m.MaxAgeSeconds) > math.MaxInt64/int64(time.Second) {
		return fmt.Errorf("mtaSts.maxAgeSeconds is too large")
	}
	return nil
}

// mtaStsChanged reports whether two MTA-STS blocks differ.
func mtaStsChanged(a, b *DomainMtaSts) bool {
	if (a == nil) != (b == nil) {
		return true
	}
	if a == nil {
		return false
	}
	return a.PolicyID != b.PolicyID ||
		a.Mode != b.Mode ||
		a.MaxAgeSeconds != b.MaxAgeSeconds ||
		!stringsEqual(a.Mx, b.Mx)
}

// reportAddrChanged reports whether two DMARC/TLSRPT report addresses differ.
func reportAddrChanged(a, b *DomainReportAddress) bool {
	if (a == nil) != (b == nil) {
		return true
	}
	if a == nil {
		return false
	}
	return *a != *b
}

// localpartConfigChanged reports whether two localpart configs differ.
func localpartConfigChanged(a, b *DomainLocalpartConfig) bool {
	if (a == nil) != (b == nil) {
		return true
	}
	if a == nil {
		return false
	}
	return a.CaseSensitive != b.CaseSensitive || !stringsEqual(a.CatchallSeparators, b.CatchallSeparators)
}

// applyDomainConfig writes the supplied optional config fields for a domain in
// the order mox requires (localpart config before DMARC/TLSRPT so separator
// validation against report localparts is consistent). The disabled flag is set
// via DomainAdd and is not written here.
func applyDomainConfig(ctx context.Context, client *moxadmin.Client, domain string, in DomainArgs) error {
	if in.Description != nil {
		if err := client.DomainDescriptionSave(ctx, domain, *in.Description); err != nil {
			return fmt.Errorf("saving description: %w", err)
		}
	}
	if in.ClientSettingsDomain != nil {
		if err := client.DomainClientSettingsDomainSave(ctx, domain, *in.ClientSettingsDomain); err != nil {
			return fmt.Errorf("saving client-settings domain: %w", err)
		}
	}
	if in.LocalpartConfig != nil {
		if err := client.DomainLocalpartConfigSave(ctx, domain, in.LocalpartConfig.CatchallSeparators, in.LocalpartConfig.CaseSensitive); err != nil {
			return fmt.Errorf("saving localpart config: %w", err)
		}
	}
	if in.Dmarc != nil {
		if err := client.DomainDMARCAddressSave(ctx, domain, in.Dmarc.Localpart, in.Dmarc.Domain, in.Dmarc.Account, in.Dmarc.Mailbox); err != nil {
			return fmt.Errorf("saving DMARC address: %w", err)
		}
	}
	if in.TlsRpt != nil {
		if err := client.DomainTLSRPTAddressSave(ctx, domain, in.TlsRpt.Localpart, in.TlsRpt.Domain, in.TlsRpt.Account, in.TlsRpt.Mailbox); err != nil {
			return fmt.Errorf("saving TLSRPT address: %w", err)
		}
	}
	if len(in.Routes) > 0 {
		if err := client.DomainRoutesSave(ctx, domain, toMoxRoutes(in.Routes)); err != nil {
			return fmt.Errorf("saving routes: %w", err)
		}
	}
	if in.MtaSts != nil {
		if err := client.DomainMTASTSSave(ctx, domain, in.MtaSts.PolicyID, in.MtaSts.Mode, toNanos(in.MtaSts.MaxAgeSeconds), in.MtaSts.Mx); err != nil {
			return fmt.Errorf("saving MTA-STS policy: %w", err)
		}
	}
	return nil
}

// domainConfigManaged reports whether the user manages any DomainConfig-backed
// field (anything beyond the immutable domain/account/localpart identity and the
// Disabled flag, which is read from the domain list).
func domainConfigManaged(in DomainArgs) bool {
	return in.Description != nil ||
		in.ClientSettingsDomain != nil ||
		in.MtaSts != nil ||
		in.Dmarc != nil ||
		in.TlsRpt != nil ||
		in.LocalpartConfig != nil ||
		in.Routes != nil
}

// reflectDomainConfig updates inputs in place from mox's domain config, touching
// only fields the user manages (present in managed). Unmanaged fields are left
// untouched so mox defaults are not surfaced as drift.
func reflectDomainConfig(inputs *DomainArgs, managed DomainArgs, cfg moxadmin.DomainConfig) {
	if managed.Description != nil {
		d := cfg.Description
		inputs.Description = &d
	}
	if managed.ClientSettingsDomain != nil {
		c := cfg.ClientSettingsDomain
		inputs.ClientSettingsDomain = &c
	}
	if managed.LocalpartConfig != nil {
		lc := &DomainLocalpartConfig{CaseSensitive: cfg.LocalpartCaseSensitive}
		if len(cfg.LocalpartCatchallSeparators) > 0 {
			lc.CatchallSeparators = cfg.LocalpartCatchallSeparators
		}
		inputs.LocalpartConfig = lc
	}
	if managed.Dmarc != nil {
		inputs.Dmarc = fromMoxReportAddress(cfg.DMARC)
	}
	if managed.TlsRpt != nil {
		inputs.TlsRpt = fromMoxReportAddress(cfg.TLSRPT)
	}
	if managed.MtaSts != nil {
		if cfg.MTASTS == nil {
			inputs.MtaSts = nil
		} else {
			inputs.MtaSts = &DomainMtaSts{
				PolicyID:      cfg.MTASTS.PolicyID,
				Mode:          cfg.MTASTS.Mode,
				MaxAgeSeconds: fromNanos(cfg.MTASTS.MaxAge),
				Mx:            cfg.MTASTS.MX,
			}
		}
	}
	if managed.Routes != nil {
		inputs.Routes = fromMoxRoutes(cfg.Routes)
	}
}

// fromMoxReportAddress converts a mox report address to the resource type,
// reflecting a nil server value as a nil (cleared) block.
func fromMoxReportAddress(a *moxadmin.DomainReportAddress) *DomainReportAddress {
	if a == nil {
		return nil
	}
	return &DomainReportAddress{
		Localpart: a.Localpart,
		Domain:    a.Domain,
		Account:   a.Account,
		Mailbox:   a.Mailbox,
	}
}

// Create adds the domain, then reads back its DNS records.
func (d *Domain) Create(ctx context.Context, req infer.CreateRequest[DomainArgs]) (infer.CreateResponse[DomainState], error) {
	state := DomainState{DomainArgs: req.Inputs}

	if req.DryRun {
		return infer.CreateResponse[DomainState]{ID: req.Inputs.Domain, Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[DomainState]{}, err
	}

	account, localpart, disabled := resolveDomainInputs(req.Inputs)
	if err := validateMtaSts(req.Inputs.MtaSts); err != nil {
		return infer.CreateResponse[DomainState]{}, err
	}
	if err := client.DomainAdd(ctx, disabled, req.Inputs.Domain, account, localpart); err != nil {
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("adding domain %q: %w", req.Inputs.Domain, err)
	}

	// Apply optional config fields. On any failure, best-effort remove the
	// freshly-added domain so a retry does not hit "already exists".
	if err := applyDomainConfig(ctx, client, req.Inputs.Domain, req.Inputs); err != nil {
		_ = client.DomainRemove(ctx, req.Inputs.Domain)
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("configuring domain %q: %w", req.Inputs.Domain, err)
	}

	// Read DNS records after all saves: DMARC/MTA-STS/TLSRPT affect the output.
	records, err := client.DomainRecords(ctx, req.Inputs.Domain)
	if err != nil {
		// Best-effort rollback so a transient read failure does not strand an
		// unmanaged domain in mox (a retry would otherwise hit "already exists").
		_ = client.DomainRemove(ctx, req.Inputs.Domain)
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("reading DNS records for domain %q: %w", req.Inputs.Domain, err)
	}
	state.DnsRecords = records

	return infer.CreateResponse[DomainState]{ID: req.Inputs.Domain, Output: state}, nil
}

// Read refreshes state from mox. The resource ID is the domain name.
func (d *Domain) Read(ctx context.Context, req infer.ReadRequest[DomainArgs, DomainState]) (infer.ReadResponse[DomainArgs, DomainState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[DomainArgs, DomainState]{}, err
	}

	domains, err := client.Domains(ctx)
	if err != nil {
		return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("listing domains: %w", err)
	}

	var found *moxadmin.Domain
	for i := range domains {
		// Domain names are case-insensitive; mox stores a canonical ASCII form.
		if strings.EqualFold(domains[i].Name.ASCII, req.ID) || strings.EqualFold(domains[i].Name.Unicode, req.ID) {
			found = &domains[i]
			break
		}
	}
	if found == nil {
		// Domain no longer exists: signal deletion by returning an empty ID.
		return infer.ReadResponse[DomainArgs, DomainState]{}, nil
	}

	// Use the canonical ASCII name as the ID going forward.
	id := found.Name.ASCII

	records, err := client.DomainRecords(ctx, id)
	if err != nil {
		return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("reading DNS records for domain %q: %w", id, err)
	}

	inputs := req.Inputs
	inputs.Domain = found.Name.ASCII
	// Reflect Disabled only when the user manages it, so an unmanaged field does
	// not surface mox's default as drift.
	if req.Inputs.Disabled != nil {
		disabled := found.Disabled
		inputs.Disabled = &disabled
	}

	// Refresh user-managed config fields from mox's domain config.
	if domainConfigManaged(req.Inputs) {
		cfg, err := client.DomainConfig(ctx, id)
		if err != nil {
			return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("reading config for domain %q: %w", id, err)
		}
		reflectDomainConfig(&inputs, req.Inputs, cfg)
	}

	state := DomainState{DomainArgs: inputs, DnsRecords: records}

	return infer.ReadResponse[DomainArgs, DomainState]{ID: id, Inputs: inputs, State: state}, nil
}

// Update applies mutable config changes to an existing domain. The
// domain/account/localpart identity is immutable (replaceOnChanges), so this
// only reconciles the optional config fields plus the disabled flag.
func (d *Domain) Update(ctx context.Context, req infer.UpdateRequest[DomainArgs, DomainState]) (infer.UpdateResponse[DomainState], error) {
	inputs := req.Inputs
	state := DomainState{DomainArgs: inputs, DnsRecords: req.State.DnsRecords}

	if err := validateMtaSts(inputs.MtaSts); err != nil {
		return infer.UpdateResponse[DomainState]{}, err
	}

	if req.DryRun {
		return infer.UpdateResponse[DomainState]{Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, err
	}

	domain := req.ID
	prior := req.State.DomainArgs

	if strPtrVal(inputs.Description) != strPtrVal(prior.Description) {
		if err := client.DomainDescriptionSave(ctx, domain, strPtrVal(inputs.Description)); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving description: %w", err)
		}
	}
	if strPtrVal(inputs.ClientSettingsDomain) != strPtrVal(prior.ClientSettingsDomain) {
		if err := client.DomainClientSettingsDomainSave(ctx, domain, strPtrVal(inputs.ClientSettingsDomain)); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving client-settings domain: %w", err)
		}
	}
	if boolOr(inputs.Disabled, false) != boolOr(prior.Disabled, false) {
		if err := client.DomainDisabledSave(ctx, domain, boolOr(inputs.Disabled, false)); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving disabled flag: %w", err)
		}
	}
	if routesChanged(prior.Routes, inputs.Routes) {
		if err := client.DomainRoutesSave(ctx, domain, toMoxRoutes(inputs.Routes)); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving routes: %w", err)
		}
	}

	// DMARC/TLSRPT/localpart trio: mox rejects separators that collide with an
	// existing report localpart, so (1) clear report addresses being removed,
	// (2) write localpart config, (3) set the desired report addresses.
	if prior.Dmarc != nil && inputs.Dmarc == nil {
		if err := client.DomainDMARCAddressSave(ctx, domain, "", "", "", ""); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("clearing DMARC address: %w", err)
		}
	}
	if prior.TlsRpt != nil && inputs.TlsRpt == nil {
		if err := client.DomainTLSRPTAddressSave(ctx, domain, "", "", "", ""); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("clearing TLSRPT address: %w", err)
		}
	}
	if localpartConfigChanged(prior.LocalpartConfig, inputs.LocalpartConfig) {
		var seps []string
		var caseSensitive bool
		if inputs.LocalpartConfig != nil {
			seps = inputs.LocalpartConfig.CatchallSeparators
			caseSensitive = inputs.LocalpartConfig.CaseSensitive
		}
		if err := client.DomainLocalpartConfigSave(ctx, domain, seps, caseSensitive); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving localpart config: %w", err)
		}
	}
	if inputs.Dmarc != nil && reportAddrChanged(prior.Dmarc, inputs.Dmarc) {
		if err := client.DomainDMARCAddressSave(ctx, domain, inputs.Dmarc.Localpart, inputs.Dmarc.Domain, inputs.Dmarc.Account, inputs.Dmarc.Mailbox); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving DMARC address: %w", err)
		}
	}
	if inputs.TlsRpt != nil && reportAddrChanged(prior.TlsRpt, inputs.TlsRpt) {
		if err := client.DomainTLSRPTAddressSave(ctx, domain, inputs.TlsRpt.Localpart, inputs.TlsRpt.Domain, inputs.TlsRpt.Account, inputs.TlsRpt.Mailbox); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving TLSRPT address: %w", err)
		}
	}

	// MTA-STS: clear when removed, otherwise set when changed.
	if mtaStsChanged(prior.MtaSts, inputs.MtaSts) {
		if inputs.MtaSts == nil {
			if err := client.DomainMTASTSSave(ctx, domain, "", "none", 0, nil); err != nil {
				return infer.UpdateResponse[DomainState]{}, fmt.Errorf("clearing MTA-STS policy: %w", err)
			}
		} else if err := client.DomainMTASTSSave(ctx, domain, inputs.MtaSts.PolicyID, inputs.MtaSts.Mode, toNanos(inputs.MtaSts.MaxAgeSeconds), inputs.MtaSts.Mx); err != nil {
			return infer.UpdateResponse[DomainState]{}, fmt.Errorf("saving MTA-STS policy: %w", err)
		}
	}

	// DMARC/MTA-STS/TLSRPT changes alter the expected DNS records.
	records, err := client.DomainRecords(ctx, domain)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, fmt.Errorf("reading DNS records for domain %q: %w", domain, err)
	}
	state.DnsRecords = records

	return infer.UpdateResponse[DomainState]{Output: state}, nil
}

// Delete removes the domain.
func (d *Domain) Delete(ctx context.Context, req infer.DeleteRequest[DomainState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.DomainRemove(ctx, req.ID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("removing domain %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}
