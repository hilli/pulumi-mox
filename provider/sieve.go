package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// Sieve is a Sieve script stored on a mox account. A script can be marked as the
// account's single active script, which mox runs on incoming delivery (used,
// e.g., for `redirect`-based forwarding). An account may hold several scripts
// but at most one is active at a time.
type Sieve struct{}

// SieveArgs are the user-supplied inputs.
type SieveArgs struct {
	// Account is the existing mox account that owns the script. Changing it
	// forces a replacement: a script belongs to exactly one account.
	Account string `pulumi:"account" provider:"replaceOnChanges"`
	// Name is the script name within the account. Changing it forces a
	// replacement (mox identifies a script by account+name).
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// Content is the Sieve script source. mox validates it on store; an invalid
	// script is rejected.
	Content string `pulumi:"content"`
	// Active, when true, makes this the account's active script. When false (or
	// unset) the script is stored but not activated; if it was previously active
	// it is deactivated, leaving the account with no active script unless another
	// resource activates one.
	Active *bool `pulumi:"active,optional"`
}

// SieveState is the checkpointed output state.
type SieveState struct {
	SieveArgs
}

// Annotate documents the resource and its fields.
func (s *Sieve) Annotate(an infer.Annotator) {
	an.Describe(s, "A Sieve script stored on a mox account, optionally the account's active script (used for redirect-based forwarding).")
}

func (a *SieveArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Account, "Existing mox account that owns the script.")
	an.Describe(&a.Name, "Script name within the account.")
	an.Describe(&a.Content, "Sieve script source. Validated by mox when stored.")
	an.Describe(&a.Active, "When true, make this the account's active script. When false or unset, the script is stored but not activated (and deactivated if it was previously active).")
}

// sieveID joins an account and script name into the resource ID.
func sieveID(account, name string) string {
	return account + "/" + name
}

// splitSieveID parses an "account/name" resource ID. Account names and Sieve
// script names never contain "/", so a single split is unambiguous.
func splitSieveID(id string) (account, name string, err error) {
	account, name, found := strings.Cut(id, "/")
	if !found || account == "" || name == "" {
		return "", "", fmt.Errorf("invalid sieve id %q, expected \"account/name\"", id)
	}
	return account, name, nil
}

// Create stores the script and, when Active is true, activates it.
func (s *Sieve) Create(ctx context.Context, req infer.CreateRequest[SieveArgs]) (infer.CreateResponse[SieveState], error) {
	state := SieveState{SieveArgs: req.Inputs}
	id := sieveID(req.Inputs.Account, req.Inputs.Name)

	if req.DryRun {
		return infer.CreateResponse[SieveState]{ID: id, Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[SieveState]{}, err
	}

	if _, err := client.AccountSievePutScript(ctx, req.Inputs.Account, req.Inputs.Name, req.Inputs.Content); err != nil {
		return infer.CreateResponse[SieveState]{}, fmt.Errorf("storing sieve script %q on account %q: %w", req.Inputs.Name, req.Inputs.Account, err)
	}

	if boolOr(req.Inputs.Active, false) {
		if err := client.AccountSieveSetActive(ctx, req.Inputs.Account, req.Inputs.Name); err != nil {
			return infer.CreateResponse[SieveState]{}, fmt.Errorf("activating sieve script %q on account %q: %w", req.Inputs.Name, req.Inputs.Account, err)
		}
	}

	return infer.CreateResponse[SieveState]{ID: id, Output: state}, nil
}

// Read refreshes state from mox and detects drift. It returns an empty ID when
// the script (or its account) no longer exists, so Pulumi treats it as deleted.
// It also supports import: the ID is "account/name".
func (s *Sieve) Read(ctx context.Context, req infer.ReadRequest[SieveArgs, SieveState]) (infer.ReadResponse[SieveArgs, SieveState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[SieveArgs, SieveState]{}, err
	}

	account, name, err := splitSieveID(req.ID)
	if err != nil {
		return infer.ReadResponse[SieveArgs, SieveState]{}, err
	}

	scripts, activeName, err := client.AccountSieveScripts(ctx, account)
	if err != nil {
		// A missing account surfaces as a sherpa user error; treat the script as
		// gone rather than failing the refresh.
		if moxadmin.IsUserError(err) {
			return infer.ReadResponse[SieveArgs, SieveState]{}, nil
		}
		return infer.ReadResponse[SieveArgs, SieveState]{}, fmt.Errorf("listing sieve scripts for account %q: %w", account, err)
	}

	found := false
	for _, sc := range scripts {
		if sc.Name == name {
			found = true
			break
		}
	}
	if !found {
		return infer.ReadResponse[SieveArgs, SieveState]{}, nil
	}

	content, err := client.AccountSieveScript(ctx, account, name)
	if err != nil {
		return infer.ReadResponse[SieveArgs, SieveState]{}, fmt.Errorf("reading sieve script %q on account %q: %w", name, account, err)
	}

	isActive := activeName == name
	inputs := SieveArgs{Account: account, Name: name, Content: content}
	// Reflect the active flag when the user manages it or when the script is
	// actually active (so an import captures it), but leave it unset when it is
	// both unmanaged and inactive to avoid nil-vs-false churn.
	if req.Inputs.Active != nil || isActive {
		v := isActive
		inputs.Active = &v
	}

	return infer.ReadResponse[SieveArgs, SieveState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  SieveState{SieveArgs: inputs},
	}, nil
}

// Update applies in-place changes. Account and Name are replaceOnChanges, so
// only Content and Active ever change here: a changed Content re-stores the
// script (mox replaces an existing script of the same name), and a changed
// Active (de)activates it.
func (s *Sieve) Update(ctx context.Context, req infer.UpdateRequest[SieveArgs, SieveState]) (infer.UpdateResponse[SieveState], error) {
	state := SieveState{SieveArgs: req.Inputs}

	if req.DryRun {
		return infer.UpdateResponse[SieveState]{Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[SieveState]{}, err
	}

	if req.Inputs.Content != req.State.Content {
		if _, err := client.AccountSievePutScript(ctx, req.State.Account, req.State.Name, req.Inputs.Content); err != nil {
			return infer.UpdateResponse[SieveState]{}, fmt.Errorf("updating sieve script %q on account %q: %w", req.State.Name, req.State.Account, err)
		}
	}

	if boolOr(req.Inputs.Active, false) != boolOr(req.State.Active, false) {
		if boolOr(req.Inputs.Active, false) {
			if err := client.AccountSieveSetActive(ctx, req.State.Account, req.State.Name); err != nil {
				return infer.UpdateResponse[SieveState]{}, fmt.Errorf("activating sieve script %q on account %q: %w", req.State.Name, req.State.Account, err)
			}
		} else if err := client.AccountSieveSetActive(ctx, req.State.Account, ""); err != nil {
			// Deactivate: clear the account's active script (this script was the
			// active one, per the recorded state).
			return infer.UpdateResponse[SieveState]{}, fmt.Errorf("deactivating sieve script %q on account %q: %w", req.State.Name, req.State.Account, err)
		}
	}

	return infer.UpdateResponse[SieveState]{Output: state}, nil
}

// Delete removes the script. mox refuses to delete the active script, so an
// active script is deactivated first. A script (or account) already gone is
// treated as success so teardown stays idempotent.
func (s *Sieve) Delete(ctx context.Context, req infer.DeleteRequest[SieveState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	account, name, err := splitSieveID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}

	exists, active, err := client.SieveScriptExists(ctx, account, name)
	if err != nil {
		// Account gone -> nothing to delete.
		if moxadmin.IsUserError(err) {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("checking sieve script %q on account %q: %w", name, account, err)
	}
	if !exists {
		return infer.DeleteResponse{}, nil
	}

	if active {
		if err := client.AccountSieveSetActive(ctx, account, ""); err != nil {
			return infer.DeleteResponse{}, fmt.Errorf("deactivating sieve script %q on account %q before delete: %w", name, account, err)
		}
	}

	if err := client.AccountSieveDeleteScript(ctx, account, name); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("deleting sieve script %q on account %q: %w", name, account, err)
	}
	return infer.DeleteResponse{}, nil
}
