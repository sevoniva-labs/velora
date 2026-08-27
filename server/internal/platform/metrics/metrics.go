package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	Requests         *prometheus.CounterVec
	Duration         *prometheus.HistogramVec
	ResponseBytes    *prometheus.HistogramVec
	InFlight         *prometheus.GaugeVec
	IdentityEvents   *prometheus.CounterVec
	IdentityDuration *prometheus.HistogramVec
	registry         *prometheus.Registry
}

func New() *Metrics {
	r := prometheus.NewRegistry()
	m := &Metrics{
		Requests:         prometheus.NewCounterVec(prometheus.CounterOpts{Name: "forge_http_requests_total", Help: "HTTP requests"}, []string{"method", "route", "status"}),
		Duration:         prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "forge_http_request_duration_seconds", Help: "HTTP request duration", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
		ResponseBytes:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "forge_http_response_size_bytes", Help: "HTTP response size", Buckets: prometheus.ExponentialBuckets(256, 4, 9)}, []string{"method", "route"}),
		InFlight:         prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "forge_http_in_flight_requests", Help: "In-flight HTTP requests"}, []string{"method"}),
		IdentityEvents:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "velora_identity_events_total", Help: "Security-sensitive identity flow outcomes"}, []string{"flow", "result"}),
		IdentityDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "velora_identity_operation_duration_seconds", Help: "Security-sensitive identity operation duration", Buckets: prometheus.DefBuckets}, []string{"flow"}),
		registry:         r,
	}
	r.MustRegister(m.Requests, m.Duration, m.ResponseBytes, m.InFlight, m.IdentityEvents, m.IdentityDuration, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// ObserveIdentity records only fixed, low-cardinality flow/result values chosen
// by the caller. User identifiers, provider subjects, codes and tickets must
// never be used as labels.
func (m *Metrics) ObserveIdentity(flow, result string, start time.Time) {
	if m == nil {
		return
	}
	m.IdentityEvents.WithLabelValues(flow, result).Inc()
	m.IdentityDuration.WithLabelValues(flow).Observe(time.Since(start).Seconds())
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
func (m *Metrics) Begin(method string) func() {
	m.InFlight.WithLabelValues(method).Inc()
	return func() { m.InFlight.WithLabelValues(method).Dec() }
}
func (m *Metrics) Observe(method, route string, status, bytes int, start time.Time) {
	m.Requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.Duration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
	m.ResponseBytes.WithLabelValues(method, route).Observe(float64(bytes))
}
