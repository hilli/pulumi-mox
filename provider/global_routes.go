package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const globalRoutesID = "global-routes"

// GlobalRoutes manages the server-level outgoing routing rules. These are
// evaluated after account-level and domain-level routes.
type GlobalRoutes struct{}

// GlobalRoutesArgs are the user-supplied inputs.
type GlobalRoutesArgs struct {
	// Routes are the global outgoing routing rules.
	Routes []AccountRoute `pulumi:"routes,optional"`
}

// GlobalRoutesState is the checkpointed output state.
type GlobalRoutesState struct {
	GlobalRoutesArgs
}

func (g *GlobalRoutes) Annotate(a infer.Annotator) {
	a.Describe(g, "Server-level outgoing routing rules, evaluated after account and domain routes.")
}

func (a *GlobalRoutesArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Routes, "Global outgoing routing rules.")
}

func (g *GlobalRoutes) Create(ctx context.Context, req infer.CreateRequest[GlobalRoutesArgs]) (infer.CreateResponse[GlobalRoutesState], error) {
	state := GlobalRoutesState{GlobalRoutesArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[GlobalRoutesState]{ID: globalRoutesID, Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[GlobalRoutesState]{}, err
	}
	if err := client.RoutesSave(ctx, toMoxRoutes(req.Inputs.Routes)); err != nil {
		return infer.CreateResponse[GlobalRoutesState]{}, fmt.Errorf("saving global routes: %w", err)
	}
	return infer.CreateResponse[GlobalRoutesState]{ID: globalRoutesID, Output: state}, nil
}

func (g *GlobalRoutes) Read(ctx context.Context, req infer.ReadRequest[GlobalRoutesArgs, GlobalRoutesState]) (infer.ReadResponse[GlobalRoutesArgs, GlobalRoutesState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[GlobalRoutesArgs, GlobalRoutesState]{}, err
	}
	cfg, err := client.DynamicConfig(ctx)
	if err != nil {
		return infer.ReadResponse[GlobalRoutesArgs, GlobalRoutesState]{}, fmt.Errorf("reading dynamic config: %w", err)
	}
	inputs := GlobalRoutesArgs{Routes: fromMoxRoutes(cfg.Routes)}
	return infer.ReadResponse[GlobalRoutesArgs, GlobalRoutesState]{
		ID:     globalRoutesID,
		Inputs: inputs,
		State:  GlobalRoutesState{GlobalRoutesArgs: inputs},
	}, nil
}

func (g *GlobalRoutes) Update(ctx context.Context, req infer.UpdateRequest[GlobalRoutesArgs, GlobalRoutesState]) (infer.UpdateResponse[GlobalRoutesState], error) {
	state := GlobalRoutesState{GlobalRoutesArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[GlobalRoutesState]{Output: state}, nil
	}
	if !routesChanged(req.Inputs.Routes, req.State.Routes) {
		return infer.UpdateResponse[GlobalRoutesState]{Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[GlobalRoutesState]{}, err
	}
	if err := client.RoutesSave(ctx, toMoxRoutes(req.Inputs.Routes)); err != nil {
		return infer.UpdateResponse[GlobalRoutesState]{}, fmt.Errorf("saving global routes: %w", err)
	}
	return infer.UpdateResponse[GlobalRoutesState]{Output: state}, nil
}

func (g *GlobalRoutes) Delete(ctx context.Context, req infer.DeleteRequest[GlobalRoutesState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.RoutesSave(ctx, nil); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("clearing global routes: %w", err)
	}
	return infer.DeleteResponse{}, nil
}
