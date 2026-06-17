package provider

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi-go-provider/infer"
)

const dnsblMonitoringID = "dnsbl-monitoring"

// DNSBLMonitoring manages extra DNS blocklists that mox monitors for outgoing IP
// listings without using those blocklists for incoming-message rejection.
type DNSBLMonitoring struct{}

// DNSBLMonitoringArgs are the user-supplied inputs.
type DNSBLMonitoringArgs struct {
	// Zones are DNSBL zones to monitor, e.g. sbl.spamhaus.org.
	Zones []string `pulumi:"zones,optional"`
}

// DNSBLMonitoringState is the checkpointed output state.
type DNSBLMonitoringState struct {
	DNSBLMonitoringArgs
}

func (d *DNSBLMonitoring) Annotate(a infer.Annotator) {
	a.Describe(d, "DNS blocklists to monitor for outgoing IP listings without using them for incoming-message rejection.")
}

func (a *DNSBLMonitoringArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Zones, "DNSBL zones to monitor, e.g. sbl.spamhaus.org.")
}

func (d *DNSBLMonitoring) Create(ctx context.Context, req infer.CreateRequest[DNSBLMonitoringArgs]) (infer.CreateResponse[DNSBLMonitoringState], error) {
	state := DNSBLMonitoringState{DNSBLMonitoringArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[DNSBLMonitoringState]{ID: dnsblMonitoringID, Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[DNSBLMonitoringState]{}, err
	}
	if err := client.MonitorDNSBLsSave(ctx, req.Inputs.Zones); err != nil {
		return infer.CreateResponse[DNSBLMonitoringState]{}, fmt.Errorf("saving DNSBL monitoring zones: %w", err)
	}
	return infer.CreateResponse[DNSBLMonitoringState]{ID: dnsblMonitoringID, Output: state}, nil
}

func (d *DNSBLMonitoring) Read(ctx context.Context, req infer.ReadRequest[DNSBLMonitoringArgs, DNSBLMonitoringState]) (infer.ReadResponse[DNSBLMonitoringArgs, DNSBLMonitoringState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[DNSBLMonitoringArgs, DNSBLMonitoringState]{}, err
	}
	cfg, err := client.DynamicConfig(ctx)
	if err != nil {
		return infer.ReadResponse[DNSBLMonitoringArgs, DNSBLMonitoringState]{}, fmt.Errorf("reading dynamic config: %w", err)
	}
	inputs := DNSBLMonitoringArgs{Zones: cfg.MonitorDNSBLs}
	return infer.ReadResponse[DNSBLMonitoringArgs, DNSBLMonitoringState]{
		ID:     dnsblMonitoringID,
		Inputs: inputs,
		State:  DNSBLMonitoringState{DNSBLMonitoringArgs: inputs},
	}, nil
}

func (d *DNSBLMonitoring) Update(ctx context.Context, req infer.UpdateRequest[DNSBLMonitoringArgs, DNSBLMonitoringState]) (infer.UpdateResponse[DNSBLMonitoringState], error) {
	state := DNSBLMonitoringState{DNSBLMonitoringArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[DNSBLMonitoringState]{Output: state}, nil
	}
	if reflect.DeepEqual(req.Inputs.Zones, req.State.Zones) {
		return infer.UpdateResponse[DNSBLMonitoringState]{Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[DNSBLMonitoringState]{}, err
	}
	if err := client.MonitorDNSBLsSave(ctx, req.Inputs.Zones); err != nil {
		return infer.UpdateResponse[DNSBLMonitoringState]{}, fmt.Errorf("saving DNSBL monitoring zones: %w", err)
	}
	return infer.UpdateResponse[DNSBLMonitoringState]{Output: state}, nil
}

func (d *DNSBLMonitoring) Delete(ctx context.Context, req infer.DeleteRequest[DNSBLMonitoringState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.MonitorDNSBLsSave(ctx, nil); err != nil {
		return infer.DeleteResponse{}, fmt.Errorf("clearing DNSBL monitoring zones: %w", err)
	}
	return infer.DeleteResponse{}, nil
}
