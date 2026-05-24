package main

import "os"

type Config struct {
	PrometheusURL   string
	PrometheusUser  string
	PrometheusToken string
	LokiURL         string
	LokiUser        string
	LokiToken       string
	LokiOrgID       string
	TempoURL        string
	TempoUser       string
	TempoToken      string
}

func LoadConfig() Config {
	return Config{
		PrometheusURL:   getenv("PROMETHEUS_URL", "http://localhost:9090"),
		PrometheusUser:  getenv("PROMETHEUS_USER", ""),
		PrometheusToken: getenv("PROMETHEUS_TOKEN", ""),
		LokiURL:         getenv("LOKI_URL", "http://localhost:3100"),
		LokiUser:        getenv("LOKI_USER", ""),
		LokiToken:       getenv("LOKI_TOKEN", ""),
		LokiOrgID:       getenv("LOKI_ORG_ID", ""),
		TempoURL:        getenv("TEMPO_URL", "http://localhost:3200"),
		TempoUser:       getenv("TEMPO_USER", ""),
		TempoToken:      getenv("TEMPO_TOKEN", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}