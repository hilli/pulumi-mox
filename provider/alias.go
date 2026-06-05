package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// Alias is a mox alias (mailing list): a single address whose mail is delivered
// to a set of member account addresses.
type Alias struct{}

// AliasArgs are the user-supplied inputs.
type AliasArgs struct {
	// Address is the full alias address, e.g. "list@example.com". Changing it
	// forces a replacement: mox identifies an alias by its localpart+domain, so
	// a new address is a different resource.
	Address string `pulumi:"address" provider:"replaceOnChanges"`
	// Members are the member account addresses that receive mail sent to the
	// alias.
	Members []string `pulumi:"members"`
	// PostPublic, when true, allows anyone (not just members) to send to the
	// alias. Defaults to false.
	PostPublic *bool `pulumi:"postPublic,optional"`
	// ListMembers, when true, allows members to see the membership list.
	// Defaults to false.
	ListMembers *bool `pulumi:"listMembers,optional"`
	// AllowMsgFrom, when true, allows members to send messages using the alias
	// address in the From header. Defaults to false.
	AllowMsgFrom *bool `pulumi:"allowMsgFrom,optional"`
}

// AliasState is the checkpointed output state.
type AliasState struct {
	AliasArgs
}

// Annotate documents the resource and its fields.
func (a *Alias) Annotate(an infer.Annotator) {
	an.Describe(a, "A mox alias (mailing list): an address whose mail is delivered to a set of member account addresses.")
}

func (a *AliasArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Address, "Full alias address, e.g. \"list@example.com\".")
	an.Describe(&a.Members, "Member account addresses that receive mail sent to the alias.")
	an.Describe(&a.PostPublic, "Allow anyone (not just members) to send to the alias. Defaults to false.")
	an.Describe(&a.ListMembers, "Allow members to see the membership list. Defaults to false.")
	an.Describe(&a.AllowMsgFrom, "Allow members to use the alias address in the From header. Defaults to false.")
}

// Create adds the alias.
func (a *Alias) Create(ctx context.Context, req infer.CreateRequest[AliasArgs]) (infer.CreateResponse[AliasState], error) {
	state := AliasState{AliasArgs: req.Inputs}

	if req.DryRun {
		return infer.CreateResponse[AliasState]{ID: req.Inputs.Address, Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[AliasState]{}, err
	}

	localpart, domain, err := splitAddress(req.Inputs.Address)
	if err != nil {
		return infer.CreateResponse[AliasState]{}, err
	}

	alias := moxadmin.Alias{
		Addresses:    req.Inputs.Members,
		PostPublic:   boolOr(req.Inputs.PostPublic, false),
		ListMembers:  boolOr(req.Inputs.ListMembers, false),
		AllowMsgFrom: boolOr(req.Inputs.AllowMsgFrom, false),
	}
	if err := client.AliasAdd(ctx, localpart, domain, alias); err != nil && !moxadmin.IsAlreadyExists(err) {
		return infer.CreateResponse[AliasState]{}, fmt.Errorf("adding alias %q: %w", req.Inputs.Address, err)
	}

	return infer.CreateResponse[AliasState]{ID: req.Inputs.Address, Output: state}, nil
}

// Read reflects the alias from mox. If the domain or the alias localpart no
// longer exists, the resource is reported as gone (empty ID). Member addresses
// are always reflected; the boolean settings are reflected only when they are
// managed (set in the program), to avoid spurious diffs on defaulted fields.
func (a *Alias) Read(ctx context.Context, req infer.ReadRequest[AliasArgs, AliasState]) (infer.ReadResponse[AliasArgs, AliasState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[AliasArgs, AliasState]{}, err
	}

	localpart, domain, err := splitAddress(req.ID)
	if err != nil {
		return infer.ReadResponse[AliasArgs, AliasState]{}, err
	}

	dc, err := client.DomainConfig(ctx, domain)
	if err != nil {
		// A missing domain is reported by mox as a user error; treat it as the
		// alias being gone rather than a hard failure.
		if moxadmin.IsUserError(err) {
			return infer.ReadResponse[AliasArgs, AliasState]{}, nil
		}
		return infer.ReadResponse[AliasArgs, AliasState]{}, fmt.Errorf("reading domain config for %q: %w", domain, err)
	}

	srv, found := dc.Alias(localpart)
	if !found {
		return infer.ReadResponse[AliasArgs, AliasState]{}, nil
	}

	inputs := req.Inputs
	inputs.Address = req.ID
	inputs.Members = srv.Addresses
	if req.Inputs.PostPublic != nil {
		v := srv.PostPublic
		inputs.PostPublic = &v
	}
	if req.Inputs.ListMembers != nil {
		v := srv.ListMembers
		inputs.ListMembers = &v
	}
	if req.Inputs.AllowMsgFrom != nil {
		v := srv.AllowMsgFrom
		inputs.AllowMsgFrom = &v
	}

	return infer.ReadResponse[AliasArgs, AliasState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  AliasState{AliasArgs: inputs},
	}, nil
}

// Update reconciles the alias membership and settings. The address is
// replaceOnChanges, so it is unchanged here. It reads the current server state
// once, adds new member addresses before removing dropped ones (mox rejects an
// empty membership), and rewrites the boolean settings only when the merged
// values differ from what the server has.
func (a *Alias) Update(ctx context.Context, req infer.UpdateRequest[AliasArgs, AliasState]) (infer.UpdateResponse[AliasState], error) {
	state := AliasState{AliasArgs: req.Inputs}

	if req.DryRun {
		return infer.UpdateResponse[AliasState]{Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[AliasState]{}, err
	}

	localpart, domain, err := splitAddress(req.Inputs.Address)
	if err != nil {
		return infer.UpdateResponse[AliasState]{}, err
	}

	dc, err := client.DomainConfig(ctx, domain)
	if err != nil {
		return infer.UpdateResponse[AliasState]{}, fmt.Errorf("reading domain config for %q: %w", domain, err)
	}
	srv, found := dc.Alias(localpart)
	if !found {
		return infer.UpdateResponse[AliasState]{}, fmt.Errorf("alias %q no longer exists", req.Inputs.Address)
	}

	// Membership: add new members before removing dropped ones so the alias is
	// never momentarily empty.
	toAdd := stringsDiff(req.Inputs.Members, srv.Addresses)
	toRemove := stringsDiff(srv.Addresses, req.Inputs.Members)
	if len(toAdd) > 0 {
		if err := client.AliasAddressesAdd(ctx, localpart, domain, toAdd); err != nil {
			return infer.UpdateResponse[AliasState]{}, fmt.Errorf("adding members to alias %q: %w", req.Inputs.Address, err)
		}
	}
	if len(toRemove) > 0 {
		if err := client.AliasAddressesRemove(ctx, localpart, domain, toRemove); err != nil {
			return infer.UpdateResponse[AliasState]{}, fmt.Errorf("removing members from alias %q: %w", req.Inputs.Address, err)
		}
	}

	// Settings: merge managed inputs over the current server values and rewrite
	// only when something changed.
	postPublic := boolOr(req.Inputs.PostPublic, srv.PostPublic)
	listMembers := boolOr(req.Inputs.ListMembers, srv.ListMembers)
	allowMsgFrom := boolOr(req.Inputs.AllowMsgFrom, srv.AllowMsgFrom)
	if postPublic != srv.PostPublic || listMembers != srv.ListMembers || allowMsgFrom != srv.AllowMsgFrom {
		if err := client.AliasUpdate(ctx, localpart, domain, postPublic, listMembers, allowMsgFrom); err != nil {
			return infer.UpdateResponse[AliasState]{}, fmt.Errorf("updating settings for alias %q: %w", req.Inputs.Address, err)
		}
	}

	return infer.UpdateResponse[AliasState]{Output: state}, nil
}

// Delete removes the alias. An already-gone alias (reported by mox as a user
// error) is tolerated.
func (a *Alias) Delete(ctx context.Context, req infer.DeleteRequest[AliasState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	localpart, domain, err := splitAddress(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	if err := client.AliasRemove(ctx, localpart, domain); err != nil && !moxadmin.IsUserError(err) {
		return infer.DeleteResponse{}, fmt.Errorf("removing alias %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// splitAddress splits a full email address into its localpart and domain on the
// last "@". Splitting on the last "@" allows quoted localparts that themselves
// contain "@".
func splitAddress(address string) (localpart, domain string, err error) {
	i := strings.LastIndex(address, "@")
	if i <= 0 || i == len(address)-1 {
		return "", "", fmt.Errorf("invalid address %q: expected \"localpart@domain\"", address)
	}
	return address[:i], address[i+1:], nil
}

// stringsDiff returns the elements of a that are not present in b.
func stringsDiff(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(b))
	for _, s := range b {
		set[s] = struct{}{}
	}
	var out []string
	for _, s := range a {
		if _, ok := set[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}
