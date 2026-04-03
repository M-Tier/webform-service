package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts total HTTP requests by endpoint, method, and status code
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webform_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"endpoint", "method", "status"},
	)

	// HTTPRequestDuration measures HTTP request duration by endpoint
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webform_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method"},
	)

	// ContactSubmissionsTotal counts contact form submissions by site and status
	ContactSubmissionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webform_contact_submissions_total",
			Help: "Total number of contact form submissions",
		},
		[]string{"site_id", "status"},
	)

	// RateLimitHitsTotal counts rate limit hits
	RateLimitHitsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "webform_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
	)

	// ValidationFailuresTotal counts validation failures by reason
	ValidationFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webform_validation_failures_total",
			Help: "Total number of validation failures",
		},
		[]string{"reason"},
	)

	// EmailSendDuration measures email sending duration
	EmailSendDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "webform_email_send_duration_seconds",
			Help:    "Email send duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		},
	)
)

// RecordRequest records an HTTP request with the given parameters
func RecordRequest(endpoint, method string, statusCode int, durationSeconds float64) {
	status := statusCodeToCategory(statusCode)
	HTTPRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
	HTTPRequestDuration.WithLabelValues(endpoint, method).Observe(durationSeconds)
}

// RecordContactSubmission records a contact form submission
func RecordContactSubmission(siteID string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	ContactSubmissionsTotal.WithLabelValues(siteID, status).Inc()
}

// RecordRateLimitHit records a rate limit hit
func RecordRateLimitHit() {
	RateLimitHitsTotal.Inc()
}

// RecordValidationFailure records a validation failure with reason
func RecordValidationFailure(reason string) {
	ValidationFailuresTotal.WithLabelValues(reason).Inc()
}

// RecordEmailSend records email send duration
func RecordEmailSend(durationSeconds float64) {
	EmailSendDuration.Observe(durationSeconds)
}

// statusCodeToCategory converts HTTP status codes to categories
func statusCodeToCategory(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}
