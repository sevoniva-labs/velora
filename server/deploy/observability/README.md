# Observability profiles

`otel-collector.yaml` and `prometheus.yml` are local-development examples. They are intentionally plaintext and must not be used for production.

Production uses `otel-collector-production.yaml` and `prometheus-production.yml`:

- OTLP receiver mTLS authenticates Forge clients.
- OTLP exporter mTLS authenticates the central telemetry backend.
- A persistent file-backed queue and bounded retry policy protect short outages.
- Prometheus presents a client certificate and verifies the Forge service certificate.
- No debug exporter is enabled in the production pipeline.

The static repository gate checks these invariants. Before promoting an approved internal Collector image, execute its `otelcol validate --config deploy/observability/otel-collector-production.yaml` command with the required environment variables and archive the output with the image digest. Static validation alone is not evidence that an arbitrary Collector version is certified.
