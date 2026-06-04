package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

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
}

// AccountState is the checkpointed output state.
type AccountState struct {
	AccountArgs
}

// Annotate documents the resource and its fields.
func (a *Account) Annotate(an infer.Annotator) {
	an.Describe(a, "A mox account, owning one or more email addresses.")
}

func (a *AccountArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Account, "Account name (mox's account identifier).")
	an.Describe(&a.Address, "Initial email address for the account, e.g. \"user@example.com\".")
	an.Describe(&a.Password, "Optional account password. When set, SetPassword is called. Generation is the caller's responsibility.")
}

// Create adds the account and, if a password was supplied, sets it.
func (a *Account) Create(ctx context.Context, req infer.CreateRequest[AccountArgs]) (infer.CreateResponse[AccountState], error) {
	state := AccountState{AccountArgs: req.Inputs}

	if req.DryRun {
		return infer.CreateResponse[AccountState]{ID: req.Inputs.Account, Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[AccountState]{}, err
	}

	if err := client.AccountAdd(ctx, req.Inputs.Account, req.Inputs.Address); err != nil && !moxadmin.IsAlreadyExists(err) {
		return infer.CreateResponse[AccountState]{}, fmt.Errorf("adding account %q: %w", req.Inputs.Account, err)
	}

	if req.Inputs.Password != nil && *req.Inputs.Password != "" {
		if err := client.SetPassword(ctx, req.Inputs.Account, *req.Inputs.Password); err != nil {
			return infer.CreateResponse[AccountState]{}, fmt.Errorf("setting password for account %q: %w", req.Inputs.Account, err)
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

	state := AccountState{AccountArgs: inputs}
	return infer.ReadResponse[AccountArgs, AccountState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

// Update applies in-place changes that don't require replacing the account:
// rotating the password (SetPassword) and migrating the primary address
// (AddressAdd new, then AddressRemove old). It is a no-op during a dry run.
func (a *Account) Update(ctx context.Context, req infer.UpdateRequest[AccountArgs, AccountState]) (infer.UpdateResponse[AccountState], error) {
	state := AccountState{AccountArgs: req.Inputs}

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
