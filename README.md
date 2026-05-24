# monitoring-mcp

An MCP server that gives Claude direct access to your Grafana observability stack.
When an alert fires, instead of jumping between Prometheus, Loki, and Tempo tabs yourself,
you ask Claude — it queries all three systems, connects the dots, and hands you a root-cause
analysis in under 30 seconds.

---

## Demo — Claude investigates a payments incident

> **User:** payments service has been throwing errors for the last 20 minutes, can you check what's going on?

**Claude:**

I'll start with the error rate in Prometheus.

```
Tool: prometheus_query
Query: rate(payments_requests_total{status="error"}[5m])

Result: 0.184  (18.4% error rate — baseline is <1%)
```

That's a clear spike. Let me check when it started and how latency looks.

```
Tool: prometheus_query_range
Query: histogram_quantile(0.99, rate(payments_request_duration_seconds_bucket[5m]))
Start: 2024-01-15T14:15:00Z  End: 2024-01-15T14:45:00Z  Step: 1m

Result: P99 was ~210ms until 14:23 UTC, then jumped to 3.4s and stayed there.
```

Something happened at 14:23. Let me pull the error logs from that window.

```
Tool: loki_query_range
Query: {app="payments"} |= "error" | json
Start: 2024-01-15T14:20:00Z  End: 2024-01-15T14:45:00Z  Limit: 50

Result (847 matching lines, sample):
  {"level":"error","msg":"payment failed: payment processor timeout: context deadline
   exceeded after 3000ms duration=3.104s","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
  {"level":"error","msg":"payment failed: payment processor timeout: context deadline
   exceeded after 3000ms duration=2.891s","trace_id":"7b3a1f9c2d4e5a6b8c9d0e1f2a3b4c5d"}
```

Every error has the same message: "payment processor timeout". Let me open one of those traces.

```
Tool: tempo_get_trace
Trace ID: 4bf92f3577b34da6a3ce929d0e0e4736

Result:
  process_payment     3.104s   (root span, status=ERROR)
  └── db.query        3.097s   ← 99.8% of the total latency
        db.system:      postgresql
        db.name:        payments_db
        db.statement:   SELECT * FROM payment_methods WHERE user_id = ?
        db.slow_query:  true
        db.rows_examined: 2400000
```

**Root cause:** The `db.query` span consumed 3.097s of a 3.104s request. The span attributes show `db.slow_query=true` and `db.rows_examined=2400000` — the query `SELECT * FROM payment_methods WHERE user_id = ?` is doing a full table scan across 2.4M rows. There is no index on `payment_methods.user_id`.

**Fix:**
```sql
CREATE INDEX CONCURRENTLY idx_payment_methods_user_id ON payment_methods(user_id);
```

`CONCURRENTLY` builds the index without locking the table, so it's safe to run on live traffic. Once it completes (a few minutes on a table this size), latency will drop back to the ~100ms baseline and the error rate will return to normal.

---

## Quick start

```bash
# 1. Clone and build the MCP server
git clone https://github.com/dauren/monitoring-mcp
cd monitoring-mcp
make build          # produces bin/monitoring-mcp

# 2. Spin up the local demo stack
#    Includes Prometheus, Loki, Tempo, Grafana, and a simulated payments service
#    that generates ~15% error rate — ready to investigate immediately
make docker-up

# 3. Register the MCP server with Claude Desktop (see config below)
# 4. Ask Claude: "payments service is throwing errors, what happened?"
```

Grafana is at **http://localhost:3000** (no login required).

---

## Claude Desktop config

`~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "monitoring": {
      "command": "/path/to/bin/monitoring-mcp",
      "env": {
        "PROMETHEUS_URL": "http://localhost:9090",
        "LOKI_URL": "http://localhost:3100",
        "TEMPO_URL": "http://localhost:3200"
      }
    }
  }
}
```

### Claude Code (CLI)

```bash
claude mcp add monitoring /path/to/bin/monitoring-mcp \
  -e PROMETHEUS_URL=http://localhost:9090 \
  -e LOKI_URL=http://localhost:3100 \
  -e TEMPO_URL=http://localhost:3200
```

---

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PROMETHEUS_URL` | `http://localhost:9090` | Prometheus base URL |
| `PROMETHEUS_TOKEN` | — | Bearer token |
| `LOKI_URL` | `http://localhost:3100` | Loki base URL |
| `LOKI_TOKEN` | — | Bearer token |
| `LOKI_ORG_ID` | — | `X-Scope-OrgID` for multi-tenant Loki |
| `TEMPO_URL` | `http://localhost:3200` | Tempo base URL |
| `TEMPO_TOKEN` | — | Bearer token |

---

## Available tools

### Prometheus

| Tool | Description |
|---|---|
| `prometheus_query` | PromQL instant query — current values, alert conditions |
| `prometheus_query_range` | PromQL range query — trends over time |
| `prometheus_metric_names` | List all available metrics |
| `prometheus_label_values` | List values for a label (`job`, `instance`, `namespace`, …) |

### Loki

| Tool | Description |
|---|---|
| `loki_query_range` | LogQL range query with line filters and JSON parsing |
| `loki_label_values` | List values for a log label (`app`, `level`, `namespace`, …) |

### Tempo

| Tool | Description |
|---|---|
| `tempo_search` | Search traces by service, tags, or duration |
| `tempo_get_trace` | Full trace by ID — all spans, timings, attributes |
| `tempo_search_tags` | List searchable tag names |
| `tempo_tag_values` | List values for a tag (`service.name`, `http.status_code`, …) |

---

## Connecting your own services

The MCP server queries standard Grafana APIs — no agents or SDKs required on your side.
Any service that already exports to this stack is immediately queryable:

- **Metrics:** expose a `/metrics` endpoint (Prometheus client library) and add a scrape config
- **Logs:** push structured JSON to Loki's `/loki/api/v1/push`, or route via Promtail/Alloy
- **Traces:** send OTLP to Tempo (port 4317 for gRPC, 4318 for HTTP)

The demo app in `examples/demo-app/` shows the minimal setup in Go using only
`prometheus/client_golang`, the OpenTelemetry SDK, and stdlib HTTP for Loki.

---

## Local demo stack

`make docker-up` starts:

| Service | Port | Notes |
|---|---|---|
| Prometheus | 9090 | Scrapes demo-app metrics every 15s |
| Loki | 3100 | Receives logs from demo-app |
| Tempo | 3200 / 4318 | Receives OTLP traces from demo-app |
| Grafana | 3000 | Pre-configured datasources, no login |
| demo-app | 8080 | Simulated payments service with ~15% slow/error rate |

```bash
make docker-down   # stops and removes volumes
```

---

## Build

```bash
make build    # bin/monitoring-mcp
make test     # go test -race ./...
make lint     # golangci-lint
```