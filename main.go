package main

import (
	"log"

	"github.com/dauren/monitoring-mcp/client"
	"github.com/dauren/monitoring-mcp/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	cfg := LoadConfig()

	s := server.NewMCPServer("monitoring-mcp", "0.1.0")

	tools.RegisterPrometheus(s, client.NewPrometheusClient(cfg.PrometheusURL, cfg.PrometheusUser, cfg.PrometheusToken))
	tools.RegisterLoki(s, client.NewLokiClient(cfg.LokiURL, cfg.LokiUser, cfg.LokiToken, cfg.LokiOrgID))
	tools.RegisterTempo(s, client.NewTempoClient(cfg.TempoURL, cfg.TempoUser, cfg.TempoToken))

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}