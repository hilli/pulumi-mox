package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// LogLevel manages a single package-specific mox log level override.
type LogLevel struct{}

// LogLevelArgs are the user-supplied inputs.
type LogLevelArgs struct {
	// Package is the mox package/logger name to configure.
	Package string `pulumi:"package" provider:"replaceOnChanges"`
	// Level is the log level string accepted by mox, such as debug, info, warn or error.
	Level string `pulumi:"level"`
}

// LogLevelState is the checkpointed output state.
type LogLevelState struct {
	LogLevelArgs
}

func (l *LogLevel) Annotate(a infer.Annotator) {
	a.Describe(l, "A package-specific mox log level override.")
}

func (a *LogLevelArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Package, "Mox package/logger name to configure.")
	an.Describe(&a.Level, "Log level string accepted by mox, such as debug, info, warn or error.")
}

func validateLogLevelArgs(args LogLevelArgs) error {
	if args.Package == "" {
		return fmt.Errorf("package must not be empty")
	}
	if args.Level == "" {
		return fmt.Errorf("level must not be empty")
	}
	return nil
}

func (l *LogLevel) Create(ctx context.Context, req infer.CreateRequest[LogLevelArgs]) (infer.CreateResponse[LogLevelState], error) {
	state := LogLevelState{LogLevelArgs: req.Inputs}
	if err := validateLogLevelArgs(req.Inputs); err != nil {
		return infer.CreateResponse[LogLevelState]{}, err
	}
	if req.DryRun {
		return infer.CreateResponse[LogLevelState]{ID: req.Inputs.Package, Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[LogLevelState]{}, err
	}
	if err := client.LogLevelSet(ctx, req.Inputs.Package, req.Inputs.Level); err != nil {
		return infer.CreateResponse[LogLevelState]{}, fmt.Errorf("setting log level for package %q: %w", req.Inputs.Package, err)
	}
	return infer.CreateResponse[LogLevelState]{ID: req.Inputs.Package, Output: state}, nil
}

func (l *LogLevel) Read(ctx context.Context, req infer.ReadRequest[LogLevelArgs, LogLevelState]) (infer.ReadResponse[LogLevelArgs, LogLevelState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[LogLevelArgs, LogLevelState]{}, err
	}
	levels, err := client.LogLevels(ctx)
	if err != nil {
		return infer.ReadResponse[LogLevelArgs, LogLevelState]{}, fmt.Errorf("reading log levels: %w", err)
	}
	level, ok := levels[req.ID]
	if !ok {
		return infer.ReadResponse[LogLevelArgs, LogLevelState]{}, nil
	}
	inputs := LogLevelArgs{Package: req.ID, Level: level}
	return infer.ReadResponse[LogLevelArgs, LogLevelState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  LogLevelState{LogLevelArgs: inputs},
	}, nil
}

func (l *LogLevel) Update(ctx context.Context, req infer.UpdateRequest[LogLevelArgs, LogLevelState]) (infer.UpdateResponse[LogLevelState], error) {
	state := LogLevelState{LogLevelArgs: req.Inputs}
	if err := validateLogLevelArgs(req.Inputs); err != nil {
		return infer.UpdateResponse[LogLevelState]{}, err
	}
	if req.DryRun {
		return infer.UpdateResponse[LogLevelState]{Output: state}, nil
	}
	if req.Inputs.Level == req.State.Level {
		return infer.UpdateResponse[LogLevelState]{Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[LogLevelState]{}, err
	}
	if err := client.LogLevelSet(ctx, req.State.Package, req.Inputs.Level); err != nil {
		return infer.UpdateResponse[LogLevelState]{}, fmt.Errorf("setting log level for package %q: %w", req.State.Package, err)
	}
	return infer.UpdateResponse[LogLevelState]{Output: state}, nil
}

func (l *LogLevel) Delete(ctx context.Context, req infer.DeleteRequest[LogLevelState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.LogLevelRemove(ctx, req.ID); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("removing log level for package %q: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}
