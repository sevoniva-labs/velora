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
	Requests      *prometheus.CounterVec
	Duration      *prometheus.HistogramVec
	ResponseBytes *prometheus.HistogramVec
	InFlight      *prometheus.GaugeVec
	registry      *prometheus.Registry
}

func New() *Metrics {
	r := prometheus.NewRegistry()
	m := &Metrics{
		Requests:      prometheus.NewCounterVec(prometheus.CounterOpts{Name: "forge_http_requests_total", Help: "HTTP requests"}, []string{"method", "route", "status"}),
		Duration:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "forge_http_request_duration_seconds", Help: "HTTP request duration", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
		ResponseBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "forge_http_response_size_bytes", Help: "HTTP response size", Buckets: prometheus.ExponentialBuckets(256, 4, 9)}, []string{"method", "route"}),
		InFlight:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "forge_http_in_flight_requests", Help: "In-flight HTTP requests"}, []string{"method"}),
		registry:      r,
	}
	r.MustRegister(m.Requests, m.Duration, m.ResponseBytes, m.InFlight, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
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
