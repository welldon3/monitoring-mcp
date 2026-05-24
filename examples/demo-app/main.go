package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "payments"

var (
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "payments_request_duration_seconds",
		Help:    "Payment request latency",
		Buckets: []float64{.05, .1, .25, .5, 1, 2, 3, 5},
	}, []string{"status"})

	requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_requests_total",
		Help: "Total payment requests processed",
	}, []string{"status"})
)

func main() {
	lokiURL := getenv("LOKI_URL", "http://localhost:3100")
	tempoEndpoint := getenv("TEMPO_ENDPOINT", "localhost:4318")

	shutdownTracer := initTracer(tempoEndpoint)
	defer shutdownTracer()

	logger := &lokiLogger{url: lokiURL, client: &http.Client{Timeout: 3 * time.Second}}
	tracer := otel.Tracer(serviceName)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/pay", payHandler(tracer, logger))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	go generateLoad(tracer, logger)

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		slog.Info("payments service started", "addr", ":8080")
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func payHandler(tracer trace.Tracer, logger *lokiLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "process_payment")
		defer span.End()

		traceID := span.SpanContext().TraceID().String()
		amount := r.URL.Query().Get("amount")
		if amount == "" {
			amount = "99.99"
		}

		start := time.Now()
		err := processPayment(ctx, tracer)
		duration := time.Since(start)

		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(attribute.String("error.message", err.Error()))
			requestTotal.WithLabelValues("error").Inc()
			requestDuration.WithLabelValues("error").Observe(duration.Seconds())
			logger.push("error", traceID, fmt.Sprintf("payment failed: %v amount=%s duration=%s", err, amount, duration))
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		requestTotal.WithLabelValues("success").Inc()
		requestDuration.WithLabelValues("success").Observe(duration.Seconds())
		logger.push("info", traceID, fmt.Sprintf("payment processed amount=%s duration=%s", amount, duration))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Trace-ID", traceID)
		fmt.Fprintf(w, `{"status":"ok","trace_id":%q}`, traceID)
	}
}

func processPayment(ctx context.Context, tracer trace.Tracer) error {
	_, span := tracer.Start(ctx, "db.query",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.name", "payments_db"),
			attribute.String("db.statement", "SELECT * FROM payment_methods WHERE user_id = ?"),
			attribute.String("db.operation", "SELECT"),
		),
	)
	defer span.End()

	if rand.Float64() < 0.15 {
		d := time.Duration(2500+rand.Intn(700)) * time.Millisecond
		time.Sleep(d)
		span.SetStatus(codes.Error, "context deadline exceeded after 3000ms")
		span.SetAttributes(
			attribute.Bool("db.slow_query", true),
			attribute.String("db.rows_examined", "2400000"),
		)
		return fmt.Errorf("payment processor timeout: context deadline exceeded after 3000ms")
	}

	time.Sleep(time.Duration(80+rand.Intn(100)) * time.Millisecond)
	return nil
}

func generateLoad(tracer trace.Tracer, logger *lokiLogger) {
	ticker := time.NewTicker(500 * time.Millisecond)
	for range ticker.C {
		ctx, span := tracer.Start(context.Background(), "process_payment")
		traceID := span.SpanContext().TraceID().String()
		start := time.Now()
		err := processPayment(ctx, tracer)
		duration := time.Since(start)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			requestTotal.WithLabelValues("error").Inc()
			requestDuration.WithLabelValues("error").Observe(duration.Seconds())
			logger.push("error", traceID, fmt.Sprintf("payment failed: %v duration=%s", err, duration))
		} else {
			requestTotal.WithLabelValues("success").Inc()
			requestDuration.WithLabelValues("success").Observe(duration.Seconds())
			logger.push("info", traceID, fmt.Sprintf("payment processed duration=%s", duration))
		}
		span.End()
	}
}

func initTracer(endpoint string) func() {
	ctx := context.Background()
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		slog.Error("OTLP exporter init failed, traces disabled", "err", err)
		return func() {}
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String("1.0.0"),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	return func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutCtx)
	}
}

// lokiLogger sends log lines to Loki's push API.
type lokiLogger struct {
	url    string
	client *http.Client
}

func (l *lokiLogger) push(level, traceID, msg string) {
	payload := map[string]any{
		"streams": []map[string]any{
			{
				"stream": map[string]string{"app": serviceName, "level": level},
				"values": [][]string{
					{
						fmt.Sprintf("%d", time.Now().UnixNano()),
						fmt.Sprintf(`{"level":%q,"msg":%q,"trace_id":%q,"service":%q}`, level, msg, traceID, serviceName),
					},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, l.url+"/loki/api/v1/push", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := l.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
