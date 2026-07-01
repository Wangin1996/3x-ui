package runtime

import (
	"context"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// noopRuntime is the runtime returned for node-assigned inbounds. In the
// agent-only model the master never dials nodes: it updates its own DB and marks
// the node config dirty, and the pull-mode agent fetches the change. So every
// per-node inbound/user operation is a no-op here — propagation is handled by
// MarkNodeDirty + the agent's config pull.
type noopRuntime struct{}

func (noopRuntime) Name() string { return "noop" }

func (noopRuntime) AddInbound(context.Context, *model.Inbound) error { return nil }
func (noopRuntime) DelInbound(context.Context, *model.Inbound) error { return nil }
func (noopRuntime) UpdateInbound(context.Context, *model.Inbound, *model.Inbound) error {
	return nil
}
func (noopRuntime) AddUser(context.Context, *model.Inbound, map[string]any) error { return nil }
func (noopRuntime) RemoveUser(context.Context, *model.Inbound, string) error      { return nil }
func (noopRuntime) UpdateUser(context.Context, *model.Inbound, string, model.Client) error {
	return nil
}
func (noopRuntime) DeleteUser(context.Context, *model.Inbound, string) error      { return nil }
func (noopRuntime) AddClient(context.Context, *model.Inbound, model.Client) error { return nil }
func (noopRuntime) RestartXray(context.Context) error                             { return nil }
func (noopRuntime) ResetClientTraffic(context.Context, *model.Inbound, string) error {
	return nil
}
func (noopRuntime) ResetInboundTraffic(context.Context, *model.Inbound) error { return nil }
func (noopRuntime) ResetAllTraffics(context.Context) error                    { return nil }
