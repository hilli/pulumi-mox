package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// Domain is the reference resource for the provider. It manages a mox domain and
// exposes the mox-generated DNS records (zone-file lines) as an output so callers
// can program their DNS provider (e.g. Cloudflare).
//
// Adding a domain in mox requires an initial account and localpart; mox creates
// the account if it does not yet exist. There is no in-place update for these
// fields, so any change forces a replacement (Update is intentionally not
// implemented).
type Domain struct{}

// DomainArgs are the user-supplied inputs.
type DomainArgs struct {
	// Domain is the domain name to add, e.g. "example.com".
	Domain string `pulumi:"domain"`
	// Account is the initial account name to create/associate with the domain.
	// Defaults to the domain name when omitted.
	Account *string `pulumi:"account,optional"`
	// Localpart is the local part of the initial address (the part before "@").
	// Defaults to "postmaster" when omitted.
	Localpart *string `pulumi:"localpart,optional"`
	// Disabled marks the domain as administratively disabled in mox.
	Disabled *bool `pulumi:"disabled,optional"`
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
	if err := client.DomainAdd(ctx, disabled, req.Inputs.Domain, account, localpart); err != nil {
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("adding domain %q: %w", req.Inputs.Domain, err)
	}

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
	disabled := found.Disabled
	inputs.Disabled = &disabled

	state := DomainState{DomainArgs: inputs, DnsRecords: records}

	return infer.ReadResponse[DomainArgs, DomainState]{ID: id, Inputs: inputs, State: state}, nil
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
