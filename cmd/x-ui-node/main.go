package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"github.com/mhsanaei/3x-ui/v3/internal/agent"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"

	"github.com/op/go-logging"
)

func main() {
	logger.InitLogger(logging.INFO)

	master := flag.String("master", "", "master panel base URL (env XUI_NODE_MASTER_URL)")
	guid := flag.String("guid", "", "this node's guid (env XUI_NODE_GUID)")
	flag.Parse()

	cfg := agent.ConfigFromEnv()
	if *master != "" {
		cfg.MasterURL = *master
	}
	if *guid != "" {
		cfg.NodeGuid = *guid
	}

	a, err := agent.New(cfg)
	if err != nil {
		log.Fatalf("x-ui-node: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("x-ui-node starting, master:", cfg.MasterURL)
	if err := a.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("x-ui-node run: %v", err)
	}
	logger.Info("x-ui-node stopped")
}
