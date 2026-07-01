package agent

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

type Agent struct {
	cfg           Config
	client        *Client
	applier       *Applier
	reporter      *reporter
	configChanged chan struct{}
	lastSha       string
}

func New(cfg Config) (*Agent, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	applier := &Applier{}
	return &Agent{
		cfg:           cfg,
		client:        client,
		applier:       applier,
		reporter:      newReporter(applier),
		configChanged: make(chan struct{}, 1),
	}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	pull := time.NewTicker(a.cfg.PollInterval)
	defer pull.Stop()
	report := time.NewTicker(a.cfg.ReportInterval)
	defer report.Stop()

	wsCtx, wsCancel := context.WithCancel(ctx)
	defer wsCancel()
	go a.runWS(wsCtx)

	a.pullOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			a.reporter.close()
			_ = a.applier.Stop()
			return ctx.Err()
		case <-pull.C:
			a.pullOnce(ctx)
		case <-report.C:
			a.reportOnce(ctx)
		case <-a.configChanged:
			a.pullOnce(ctx)
		}
	}
}

func (a *Agent) signalConfigChanged() {
	select {
	case a.configChanged <- struct{}{}:
	default:
	}
}

func (a *Agent) reportOnce(ctx context.Context) {
	report, ok := a.reporter.build()
	if !ok {
		return
	}
	resp, err := a.client.Report(ctx, report)
	if err != nil {
		logger.Warning("agent: report failed:", err)
		return
	}
	if resp.ConfigDirty {
		a.signalConfigChanged()
	}
}

func (a *Agent) pullOnce(ctx context.Context) {
	resp, err := a.client.PullConfig(ctx, a.lastSha)
	if err != nil {
		logger.Warning("agent: pull config failed:", err)
		return
	}
	if resp.Unchanged || resp.Config == nil {
		return
	}
	if err := a.applier.Apply(resp.Config); err != nil {
		logger.Warning("agent: apply config failed:", err)
		return
	}
	a.lastSha = resp.Sha256
	logger.Info("agent: applied config", resp.Sha256)
}
