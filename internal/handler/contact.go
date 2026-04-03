package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/M-Tier/webform-service/internal/config"
	"github.com/M-Tier/webform-service/internal/email"
	"github.com/M-Tier/webform-service/internal/metrics"
	"github.com/M-Tier/webform-service/internal/security"
)

const maxBodySize = 10 * 1024 // 10KB

type ContactHandler struct {
	cfg         *config.Config
	sites       *config.SitesConfig
	emailSender *email.Sender
	rateLimiter *security.RateLimiter
	validator   *security.Validator
	logger      *slog.Logger
}

type response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func NewContactHandler(
	cfg *config.Config,
	sites *config.SitesConfig,
	emailSender *email.Sender,
	rateLimiter *security.RateLimiter,
	validator *security.Validator,
	logger *slog.Logger,
) *ContactHandler {
	return &ContactHandler{
		cfg:         cfg,
		sites:       sites,
		emailSender: emailSender,
		rateLimiter: rateLimiter,
		validator:   validator,
		logger:      logger,
	}
}

func (h *ContactHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	origin := r.Header.Get("Origin")

	// Resolve site from origin
	site := h.resolveSite(origin)

	// Set CORS headers if site is valid
	if site != nil {
		h.setCORSHeaders(w, origin)
	}

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		if site == nil {
			// Unknown origin - don't allow preflight
			w.WriteHeader(http.StatusForbidden)
			metrics.RecordRequest("/api/contact", r.Method, http.StatusForbidden, time.Since(start).Seconds())
			return
		}
		w.WriteHeader(http.StatusOK)
		metrics.RecordRequest("/api/contact", r.Method, http.StatusOK, time.Since(start).Seconds())
		return
	}

	// Only allow POST
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		metrics.RecordRequest("/api/contact", r.Method, http.StatusMethodNotAllowed, time.Since(start).Seconds())
		return
	}

	// Reject unknown origins
	if site == nil {
		h.logger.Warn("request from unknown origin", "origin", origin)
		h.sendError(w, http.StatusForbidden, "Unknown origin")
		metrics.RecordRequest("/api/contact", r.Method, http.StatusForbidden, time.Since(start).Seconds())
		return
	}

	// Get client IP
	clientIP := h.getClientIP(r)

	// Check rate limit
	if !h.rateLimiter.Allow(clientIP) {
		h.logger.Warn("rate limit exceeded", "ip", clientIP, "site", site.ID)
		h.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded. Please try again later.")
		metrics.RecordRequest("/api/contact", r.Method, http.StatusTooManyRequests, time.Since(start).Seconds())
		metrics.RecordRateLimitHit()
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	// Parse JSON body
	var req security.ContactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Debug("failed to decode request", "error", err)
		h.sendError(w, http.StatusBadRequest, "Invalid request format")
		metrics.RecordRequest("/api/contact", r.Method, http.StatusBadRequest, time.Since(start).Seconds())
		metrics.RecordValidationFailure("invalid_json")
		return
	}

	// Validate request
	validated, err := h.validator.Validate(req)
	if err != nil {
		h.logger.Info("validation failed",
			"ip", clientIP,
			"site", site.ID,
			"error", err,
		)

		// Return generic error for spam detection
		if errors.Is(err, security.ErrHoneypotTriggered) {
			h.sendError(w, http.StatusBadRequest, "Invalid request")
			metrics.RecordRequest("/api/contact", r.Method, http.StatusBadRequest, time.Since(start).Seconds())
			metrics.RecordValidationFailure("honeypot")
			return
		}
		if errors.Is(err, security.ErrFormTooFast) {
			h.sendError(w, http.StatusBadRequest, "Invalid request")
			metrics.RecordRequest("/api/contact", r.Method, http.StatusBadRequest, time.Since(start).Seconds())
			metrics.RecordValidationFailure("form_too_fast")
			return
		}

		h.sendError(w, http.StatusBadRequest, err.Error())
		metrics.RecordRequest("/api/contact", r.Method, http.StatusBadRequest, time.Since(start).Seconds())
		metrics.RecordValidationFailure("validation_error")
		return
	}

	// Send email using site-specific configuration
	emailStart := time.Now()
	if err := h.emailSender.SendContactEmail(email.ContactForm{
		Name:    validated.Name,
		Email:   validated.Email,
		Message: validated.Message,
	}, site); err != nil {
		h.logger.Error("failed to send email",
			"error", err,
			"site", site.ID,
			"name", validated.Name,
			"email", validated.Email,
		)
		h.sendError(w, http.StatusInternalServerError, "Failed to send message. Please try again later.")
		metrics.RecordRequest("/api/contact", r.Method, http.StatusInternalServerError, time.Since(start).Seconds())
		metrics.RecordContactSubmission(site.ID, false)
		metrics.RecordEmailSend(time.Since(emailStart).Seconds())
		return
	}
	metrics.RecordEmailSend(time.Since(emailStart).Seconds())

	h.logger.Info("contact form submitted",
		"site", site.ID,
		"name", validated.Name,
		"email", validated.Email,
		"ip", clientIP,
	)

	h.sendSuccess(w, "Message sent successfully")
	metrics.RecordRequest("/api/contact", r.Method, http.StatusOK, time.Since(start).Seconds())
	metrics.RecordContactSubmission(site.ID, true)
}

// resolveSite finds the site configuration for the given origin.
// In dev mode, localhost origins are allowed and use the first configured site.
func (h *ContactHandler) resolveSite(origin string) *config.SiteConfig {
	// First, try exact match
	if site := h.sites.FindByOrigin(origin); site != nil {
		return site
	}

	// In dev mode, allow localhost origins using the first site's config
	if h.cfg.DevMode && config.IsLocalhost(origin) && len(h.sites.Sites) > 0 {
		h.logger.Debug("dev mode: allowing localhost origin", "origin", origin, "site", h.sites.Sites[0].ID)
		return &h.sites.Sites[0]
	}

	return nil
}

func (h *ContactHandler) setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Vary", "Origin")
}

func (h *ContactHandler) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for reverse proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the first IP in the list
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to remote address
	// Remove port if present
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Check if it's IPv6
		if strings.Count(addr, ":") > 1 {
			// IPv6 address
			if strings.HasPrefix(addr, "[") {
				// [::1]:port format
				if bracketIdx := strings.Index(addr, "]"); bracketIdx != -1 {
					return addr[1:bracketIdx]
				}
			}
			return addr
		}
		return addr[:idx]
	}
	return addr
}

func (h *ContactHandler) sendSuccess(w http.ResponseWriter, message string) {
	h.sendJSON(w, http.StatusOK, response{
		Success: true,
		Message: message,
	})
}

func (h *ContactHandler) sendError(w http.ResponseWriter, status int, errMsg string) {
	h.sendJSON(w, status, response{
		Success: false,
		Error:   errMsg,
	})
}

func (h *ContactHandler) sendJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

// HealthHandler returns a simple health check response
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
