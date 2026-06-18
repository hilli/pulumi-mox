package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// Address is an additional email address (alias) routed to an existing mox
// account.
type Address struct{}

// AddressArgs are the user-supplied inputs.
type AddressArgs struct {
	// Address is the email address to add, e.g. "alias@example.com". Changing
	// it forces a replacement: mox identifies an alias by its address, so a new
	// address is a different resource.
	Address string `pulumi:"address" provider:"replaceOnChanges"`
	// Account is the existing account that should receive mail for Address.
	Account string `pulumi:"account"`
}

// AddressState is the checkpointed output state.
type AddressState struct {
	AddressArgs
}

// Annotate documents the resource and its fields.
func (a *Address) Annotate(an infer.Annotator) {
	an.Describe(a, "An additional email address (alias) routed to an existing mox account.")
}

func (a *AddressArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Address, "Email address to add, e.g. \"alias@example.com\".")
	an.Describe(&a.Account, "Existing account that should receive mail for this address.")
}

// Create adds the address to the account.
func (a *Address) Create(ctx context.Context, req infer.CreateRequest[AddressArgs]) (infer.CreateResponse[AddressState], error) {
	state := AddressState{AddressArgs: req.Inputs}

	if req.DryRun {
		return infer.CreateResponse[AddressState]{ID: req.Inputs.Address, Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[AddressState]{}, err
	}

	if err := client.AddressAdd(ctx, req.Inputs.Address, req.Inputs.Account); err != nil && !moxadmin.IsAlreadyExists(err) {
		return infer.CreateResponse[AddressState]{}, fmt.Errorf("adding address %q to account %q: %w", req.Inputs.Address, req.Inputs.Account, err)
	}

	return infer.CreateResponse[AddressState]{ID: req.Inputs.Address, Output: state}, nil
}

// Read reflects the address->account routing from mox. It searches all accounts
// for the address so that drift (the address re-pointed to a different account
// out-of-band) is surfaced rather than reported as gone. If the address no
// longer routes to any account, the resource is reported as gone (empty ID).
func (a *Address) Read(ctx context.Context, req infer.ReadRequest[AddressArgs, AddressState]) (infer.ReadResponse[AddressArgs, AddressState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[AddressArgs, AddressState]{}, err
	}

	account, found, err := client.FindAddressAccount(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[AddressArgs, AddressState]{}, fmt.Errorf("looking up address %q: %w", req.ID, err)
	}
	if !found {
		// The address no longer routes to any account.
		return infer.ReadResponse[AddressArgs, AddressState]{}, nil
	}

	inputs := AddressArgs{Address: req.ID, Account: account}
	return infer.ReadResponse[AddressArgs, AddressState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  AddressState{AddressArgs: inputs},
	}, nil
}

// Update re-points an address to a different account. Changing the address
// itself forces a replacement (see replaceOnChanges on AddressArgs.Address), so
// Update only ever moves the unchanged address to a new account. mox has no
// re-point primitive and rejects adding an address that still routes elsewhere,
// so this removes the old routing and re-adds it to the new account; if the
// re-add fails it best-effort restores the original routing.
func (a *Address) Update(ctx context.Context, req infer.UpdateRequest[AddressArgs, AddressState]) (infer.UpdateResponse[AddressState], error) {
	state := AddressState{AddressArgs: req.Inputs}

	if req.DryRun {
		return infer.UpdateResponse[AddressState]{Output: state}, nil
	}

	if req.Inputs.Account == req.State.Account {
		return infer.UpdateResponse[AddressState]{Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[AddressState]{}, err
	}

	// Address is replaceOnChanges, so it is unchanged here; state and inputs agree.
	address := req.Inputs.Address

	if err := client.AddressRemove(ctx, address); err != nil {
		return infer.UpdateResponse[AddressState]{}, fmt.Errorf("removing address %q from account %q: %w", address, req.State.Account, err)
	}

	if err := client.AddressAdd(ctx, address, req.Inputs.Account); err != nil {
		// Re-add to the new account failed; try to restore the original routing
		// so we do not silently drop the address.
		if rbErr := client.AddressAdd(ctx, address, req.State.Account); rbErr != nil {
			return infer.UpdateResponse[AddressState]{}, fmt.Errorf("re-pointing address %q to account %q failed: %w; restoring it to account %q also failed: %v", address, req.Inputs.Account, err, req.State.Account, rbErr)
		}
		return infer.UpdateResponse[AddressState]{}, fmt.Errorf("re-pointing address %q to account %q (restored to %q): %w", address, req.Inputs.Account, req.State.Account, err)
	}

	return infer.UpdateResponse[AddressState]{Output: state}, nil
}

// Delete removes the address.
func (a *Address) Delete(ctx context.Context, req infer.DeleteRequest[AddressState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	// A not-found error means the address is already gone (the desired end
	// state), so Delete is idempotent and treats it as success.
	if err := client.AddressRemove(ctx, req.ID); err != nil && !moxadmin.IsNotFound(err) {
		return infer.DeleteResponse{}, fmt.Errorf("removing address %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}
