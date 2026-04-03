package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SiteConfig represents configuration for a single site/domain
type SiteConfig struct {
	ID             string   `json:"id"`
	Origins        []string `json:"origins"`
	RecipientEmail string   `json:"recipientEmail"`
	SenderEmail    string   `json:"senderEmail"`
	SenderName     string   `json:"senderName"`
}

// SitesConfig holds configuration for all sites
type SitesConfig struct {
	Sites []SiteConfig `json:"sites"`

	// originMap is built at load time for fast lookups
	originMap map[string]*SiteConfig
}

// LoadSites loads site configurations from a JSON file
func LoadSites(path string) (*SitesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read sites config file: %w", err)
	}

	var sc SitesConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("failed to parse sites config: %w", err)
	}

	if err := sc.validate(); err != nil {
		return nil, err
	}

	// Build origin lookup map
	sc.originMap = make(map[string]*SiteConfig)
	for i := range sc.Sites {
		site := &sc.Sites[i]
		for _, origin := range site.Origins {
			// Normalize origin (lowercase, no trailing slash)
			normalized := normalizeOrigin(origin)
			if _, exists := sc.originMap[normalized]; exists {
				return nil, fmt.Errorf("duplicate origin %q in sites config", origin)
			}
			sc.originMap[normalized] = site
		}
	}

	return &sc, nil
}

// FindByOrigin looks up a site by its origin (e.g., "https://example.com")
// Returns nil if no matching site is found
func (sc *SitesConfig) FindByOrigin(origin string) *SiteConfig {
	normalized := normalizeOrigin(origin)
	return sc.originMap[normalized]
}

// AllOrigins returns all configured origins (useful for logging)
func (sc *SitesConfig) AllOrigins() []string {
	origins := make([]string, 0, len(sc.originMap))
	for origin := range sc.originMap {
		origins = append(origins, origin)
	}
	return origins
}

// SiteIDs returns all configured site IDs (useful for logging)
func (sc *SitesConfig) SiteIDs() []string {
	ids := make([]string, len(sc.Sites))
	for i, site := range sc.Sites {
		ids[i] = site.ID
	}
	return ids
}

// IsLocalhost checks if an origin is a localhost origin
func IsLocalhost(origin string) bool {
	normalized := normalizeOrigin(origin)
	return strings.Contains(normalized, "://localhost") ||
		strings.Contains(normalized, "://127.0.0.1") ||
		strings.Contains(normalized, "://[::1]")
}

// validate checks that all required fields are present
func (sc *SitesConfig) validate() error {
	if len(sc.Sites) == 0 {
		return fmt.Errorf("no sites configured")
	}

	for i, site := range sc.Sites {
		if site.ID == "" {
			return fmt.Errorf("site %d: id is required", i)
		}
		if len(site.Origins) == 0 {
			return fmt.Errorf("site %q: at least one origin is required", site.ID)
		}
		if site.RecipientEmail == "" {
			return fmt.Errorf("site %q: recipientEmail is required", site.ID)
		}
		if site.SenderEmail == "" {
			return fmt.Errorf("site %q: senderEmail is required", site.ID)
		}
		if site.SenderName == "" {
			return fmt.Errorf("site %q: senderName is required", site.ID)
		}
	}

	return nil
}

// normalizeOrigin normalizes an origin for consistent lookup
func normalizeOrigin(origin string) string {
	// Lowercase and remove trailing slash
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(origin)), "/")
}
