package provider

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

// DomainDKIMSelector manages one DKIM selector on a mox domain. It preserves
// other selectors on the same domain when updating settings or signing state.
type DomainDKIMSelector struct{}

// DomainDKIMSelectorArgs are the user-supplied inputs.
type DomainDKIMSelectorArgs struct {
	// Domain is the mox domain that owns the selector.
	Domain string `pulumi:"domain" provider:"replaceOnChanges"`
	// Selector is the DKIM selector label. Changing it forces replacement.
	Selector string `pulumi:"selector" provider:"replaceOnChanges"`
	// Algorithm controls the generated key for a new selector, e.g. ed25519 or rsa-2048.
	// It cannot change an existing selector's key; changing it forces replacement.
	Algorithm string `pulumi:"algorithm,optional" provider:"replaceOnChanges"`
	// Hash is the DKIM hash algorithm, normally sha256.
	Hash string `pulumi:"hash,optional"`
	// HeaderRelaxed enables relaxed header canonicalization.
	HeaderRelaxed *bool `pulumi:"headerRelaxed,optional"`
	// BodyRelaxed enables relaxed body canonicalization.
	BodyRelaxed *bool `pulumi:"bodyRelaxed,optional"`
	// SealHeaders, when true, prevents duplicate headers from being added.
	SealHeaders *bool `pulumi:"sealHeaders,optional"`
	// Headers are the headers to sign. Empty uses mox's default set.
	Headers []string `pulumi:"headers,optional"`
	// LifetimeSeconds is the DKIM signature lifetime for a newly generated
	// selector. It is stored by mox as Expiration on the selector.
	LifetimeSeconds *int `pulumi:"lifetimeSeconds,optional"`
	// Sign enables this selector for signing outgoing mail from the domain.
	Sign *bool `pulumi:"sign,optional"`
}

// DomainDKIMSelectorState is the checkpointed output state.
type DomainDKIMSelectorState struct {
	DomainDKIMSelectorArgs
	// PrivateKeyFile is the server-side key file generated/stored by mox.
	PrivateKeyFile string `pulumi:"privateKeyFile"`
	// EffectiveHash is the hash mox uses after applying defaults.
	EffectiveHash string `pulumi:"effectiveHash"`
}

func (d *DomainDKIMSelector) Annotate(a infer.Annotator) {
	a.Describe(d, "A DKIM selector on a mox domain, optionally enabled for signing.")
}

func (a *DomainDKIMSelectorArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Domain, "Mox domain that owns the selector.")
	an.Describe(&a.Selector, "DKIM selector label.")
	an.Describe(&a.Algorithm, "Key algorithm used when creating the selector, e.g. ed25519 or rsa-2048. Changing it replaces the selector.")
	an.Describe(&a.Hash, "DKIM hash algorithm, normally sha256.")
	an.Describe(&a.HeaderRelaxed, "Enable relaxed header canonicalization.")
	an.Describe(&a.BodyRelaxed, "Enable relaxed body canonicalization.")
	an.Describe(&a.SealHeaders, "Prevent duplicate headers from being added. Defaults to true.")
	an.Describe(&a.Headers, "Headers to sign. Empty uses mox's default set.")
	an.Describe(&a.LifetimeSeconds, "DKIM signature lifetime for a newly generated selector.")
	an.Describe(&a.Sign, "Enable this selector for signing outgoing mail.")
}

func (s *DomainDKIMSelectorState) Annotate(an infer.Annotator) {
	an.Describe(&s.PrivateKeyFile, "Server-side key file generated/stored by mox.")
	an.Describe(&s.EffectiveHash, "Hash mox uses after applying defaults.")
}

func dkimID(domain, selector string) string {
	return domain + "/" + selector
}

func splitDKIMID(id string) (domain, selector string, err error) {
	domain, selector, found := strings.Cut(id, "/")
	if !found || domain == "" || selector == "" {
		return "", "", fmt.Errorf("invalid DKIM selector id %q, expected \"domain/selector\"", id)
	}
	return domain, selector, nil
}

func dkimAlgorithm(args DomainDKIMSelectorArgs) string {
	if args.Algorithm != "" {
		return args.Algorithm
	}
	return "ed25519"
}

func dkimHash(args DomainDKIMSelectorArgs) string {
	if args.Hash != "" {
		return args.Hash
	}
	return "sha256"
}

func dkimLifetimeNanos(args DomainDKIMSelectorArgs) int64 {
	if args.LifetimeSeconds == nil {
		return 0
	}
	return toNanos(*args.LifetimeSeconds)
}

func validateDKIMSelectorArgs(args DomainDKIMSelectorArgs) error {
	if args.Domain == "" {
		return fmt.Errorf("domain must not be empty")
	}
	if args.Selector == "" {
		return fmt.Errorf("selector must not be empty")
	}
	if args.LifetimeSeconds != nil && *args.LifetimeSeconds < 0 {
		return fmt.Errorf("lifetimeSeconds must not be negative")
	}
	return nil
}

func selectorToMox(args DomainDKIMSelectorArgs, prior moxadmin.DKIMSelector) moxadmin.DKIMSelector {
	selector := prior
	selector.Hash = dkimHash(args)
	selector.Canonicalization = moxadmin.Canonicalization{
		HeaderRelaxed: boolOr(args.HeaderRelaxed, false),
		BodyRelaxed:   boolOr(args.BodyRelaxed, false),
	}
	selector.Headers = args.Headers
	selector.DontSealHeaders = !boolOr(args.SealHeaders, true)
	if args.LifetimeSeconds != nil {
		selector.Expiration = fmt.Sprintf("%ds", *args.LifetimeSeconds)
	}
	return selector
}

func selectorFromMox(domain, selector string, moxSel moxadmin.DKIMSelector, sign []string, inputs *DomainDKIMSelectorArgs) DomainDKIMSelectorState {
	sealHeaders := !moxSel.DontSealHeaders
	isSigning := slices.Contains(sign, selector)
	var args DomainDKIMSelectorArgs
	if inputs != nil {
		args = *inputs
		args.Domain = domain
		args.Selector = selector
	} else {
		args = DomainDKIMSelectorArgs{
			Domain:        domain,
			Selector:      selector,
			Algorithm:     moxSel.Algorithm,
			Hash:          moxSel.Hash,
			HeaderRelaxed: &moxSel.Canonicalization.HeaderRelaxed,
			BodyRelaxed:   &moxSel.Canonicalization.BodyRelaxed,
			SealHeaders:   &sealHeaders,
			Headers:       moxSel.Headers,
			Sign:          &isSigning,
		}
	}
	return DomainDKIMSelectorState{DomainDKIMSelectorArgs: args, PrivateKeyFile: moxSel.PrivateKeyFile, EffectiveHash: moxSel.HashEffective}
}

func setSigning(sign []string, selector string, enabled bool) []string {
	out := slices.Clone(sign)
	has := slices.Contains(out, selector)
	switch {
	case enabled && !has:
		out = append(out, selector)
	case !enabled && has:
		out = slices.DeleteFunc(out, func(s string) bool { return s == selector })
	}
	return out
}

func dkimManagedChanged(inputs DomainDKIMSelectorArgs, state DomainDKIMSelectorState) bool {
	return inputs.Hash != state.Hash ||
		boolOr(inputs.HeaderRelaxed, false) != boolOr(state.HeaderRelaxed, false) ||
		boolOr(inputs.BodyRelaxed, false) != boolOr(state.BodyRelaxed, false) ||
		boolOr(inputs.SealHeaders, true) != boolOr(state.SealHeaders, true) ||
		!reflect.DeepEqual(inputs.Headers, state.Headers) ||
		!reflect.DeepEqual(inputs.LifetimeSeconds, state.LifetimeSeconds) ||
		boolOr(inputs.Sign, false) != boolOr(state.Sign, false)
}

func (d *DomainDKIMSelector) Create(ctx context.Context, req infer.CreateRequest[DomainDKIMSelectorArgs]) (infer.CreateResponse[DomainDKIMSelectorState], error) {
	state := DomainDKIMSelectorState{DomainDKIMSelectorArgs: req.Inputs}
	if err := validateDKIMSelectorArgs(req.Inputs); err != nil {
		return infer.CreateResponse[DomainDKIMSelectorState]{}, err
	}
	if req.DryRun {
		return infer.CreateResponse[DomainDKIMSelectorState]{ID: dkimID(req.Inputs.Domain, req.Inputs.Selector), Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[DomainDKIMSelectorState]{}, err
	}
	cfg, err := client.DomainConfig(ctx, req.Inputs.Domain)
	if err != nil {
		return infer.CreateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("reading domain %q DKIM config: %w", req.Inputs.Domain, err)
	}

	sel, exists := cfg.DKIM.Selectors[req.Inputs.Selector]
	if !exists {
		if err := client.DomainDKIMAdd(ctx, req.Inputs.Domain, req.Inputs.Selector, dkimAlgorithm(req.Inputs), dkimHash(req.Inputs), boolOr(req.Inputs.HeaderRelaxed, false), boolOr(req.Inputs.BodyRelaxed, false), boolOr(req.Inputs.SealHeaders, true), req.Inputs.Headers, dkimLifetimeNanos(req.Inputs)); err != nil {
			return infer.CreateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("adding DKIM selector %q on domain %q: %w", req.Inputs.Selector, req.Inputs.Domain, err)
		}
		cfg, err = client.DomainConfig(ctx, req.Inputs.Domain)
		if err != nil {
			return infer.CreateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("reading domain %q DKIM config after add: %w", req.Inputs.Domain, err)
		}
		sel = cfg.DKIM.Selectors[req.Inputs.Selector]
	}

	cfg.DKIM.Selectors[req.Inputs.Selector] = selectorToMox(req.Inputs, sel)
	cfg.DKIM.Sign = setSigning(cfg.DKIM.Sign, req.Inputs.Selector, boolOr(req.Inputs.Sign, false))
	if err := client.DomainDKIMSave(ctx, req.Inputs.Domain, cfg.DKIM.Selectors, cfg.DKIM.Sign); err != nil {
		return infer.CreateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("saving DKIM selector %q on domain %q: %w", req.Inputs.Selector, req.Inputs.Domain, err)
	}

	cfg, err = client.DomainConfig(ctx, req.Inputs.Domain)
	if err != nil {
		return infer.CreateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("reading domain %q DKIM config after save: %w", req.Inputs.Domain, err)
	}
	state = selectorFromMox(req.Inputs.Domain, req.Inputs.Selector, cfg.DKIM.Selectors[req.Inputs.Selector], cfg.DKIM.Sign, &req.Inputs)
	return infer.CreateResponse[DomainDKIMSelectorState]{ID: dkimID(req.Inputs.Domain, req.Inputs.Selector), Output: state}, nil
}

func (d *DomainDKIMSelector) Read(ctx context.Context, req infer.ReadRequest[DomainDKIMSelectorArgs, DomainDKIMSelectorState]) (infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState]{}, err
	}
	domain, selector, err := splitDKIMID(req.ID)
	if err != nil {
		return infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState]{}, err
	}
	if req.State.Domain != "" && req.State.Selector != "" {
		domain = req.State.Domain
		selector = req.State.Selector
	}
	cfg, err := client.DomainConfig(ctx, domain)
	if err != nil {
		if moxadmin.IsUserError(err) {
			return infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState]{}, nil
		}
		return infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState]{}, fmt.Errorf("reading domain %q DKIM config: %w", domain, err)
	}
	sel, ok := cfg.DKIM.Selectors[selector]
	if !ok {
		return infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState]{}, nil
	}
	inputs := req.Inputs
	if inputs.Domain == "" && inputs.Selector == "" {
		inputs = req.State.DomainDKIMSelectorArgs
	}
	var inputPtr *DomainDKIMSelectorArgs
	if inputs.Domain != "" || inputs.Selector != "" {
		inputPtr = &inputs
	}
	state := selectorFromMox(domain, selector, sel, cfg.DKIM.Sign, inputPtr)
	return infer.ReadResponse[DomainDKIMSelectorArgs, DomainDKIMSelectorState]{ID: dkimID(domain, selector), Inputs: state.DomainDKIMSelectorArgs, State: state}, nil
}

func (d *DomainDKIMSelector) Update(ctx context.Context, req infer.UpdateRequest[DomainDKIMSelectorArgs, DomainDKIMSelectorState]) (infer.UpdateResponse[DomainDKIMSelectorState], error) {
	state := DomainDKIMSelectorState{DomainDKIMSelectorArgs: req.Inputs, PrivateKeyFile: req.State.PrivateKeyFile, EffectiveHash: req.State.EffectiveHash}
	if err := validateDKIMSelectorArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[DomainDKIMSelectorState]{}, err
	}
	if req.DryRun {
		return infer.UpdateResponse[DomainDKIMSelectorState]{Output: state}, nil
	}
	if !dkimManagedChanged(req.Inputs, req.State) {
		return infer.UpdateResponse[DomainDKIMSelectorState]{Output: state}, nil
	}

	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[DomainDKIMSelectorState]{}, err
	}
	cfg, err := client.DomainConfig(ctx, req.State.Domain)
	if err != nil {
		return infer.UpdateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("reading domain %q DKIM config: %w", req.State.Domain, err)
	}
	sel, ok := cfg.DKIM.Selectors[req.State.Selector]
	if !ok {
		return infer.UpdateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("DKIM selector %q no longer exists on domain %q", req.State.Selector, req.State.Domain)
	}
	cfg.DKIM.Selectors[req.State.Selector] = selectorToMox(req.Inputs, sel)
	cfg.DKIM.Sign = setSigning(cfg.DKIM.Sign, req.State.Selector, boolOr(req.Inputs.Sign, false))
	if err := client.DomainDKIMSave(ctx, req.State.Domain, cfg.DKIM.Selectors, cfg.DKIM.Sign); err != nil {
		return infer.UpdateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("saving DKIM selector %q on domain %q: %w", req.State.Selector, req.State.Domain, err)
	}
	cfg, err = client.DomainConfig(ctx, req.State.Domain)
	if err != nil {
		return infer.UpdateResponse[DomainDKIMSelectorState]{}, fmt.Errorf("reading domain %q DKIM config after save: %w", req.State.Domain, err)
	}
	state = selectorFromMox(req.State.Domain, req.State.Selector, cfg.DKIM.Selectors[req.State.Selector], cfg.DKIM.Sign, &req.Inputs)
	return infer.UpdateResponse[DomainDKIMSelectorState]{Output: state}, nil
}

func (d *DomainDKIMSelector) Delete(ctx context.Context, req infer.DeleteRequest[DomainDKIMSelectorState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	domain, selector, err := splitDKIMID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if req.State.Domain != "" && req.State.Selector != "" {
		domain = req.State.Domain
		selector = req.State.Selector
	}
	if err := client.DomainDKIMRemove(ctx, domain, selector); err != nil && !moxadmin.IsUserError(err) {
		return infer.DeleteResponse{}, fmt.Errorf("removing DKIM selector %q from domain %q: %w", selector, domain, err)
	}
	return infer.DeleteResponse{}, nil
}
