package server

import (
	"net/http"
)

func (s *Server) handlePublicConfig(w http.ResponseWriter, r *http.Request) {
	enabled := s != nil && s.cfg != nil && s.cfg.TurnstileEnabled
	siteKey := ""
	if enabled {
		siteKey = s.cfg.TurnstileSiteKey
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"turnstile_enabled":  enabled,
		"turnstile_site_key": siteKey,
	})
}

func (s *Server) handlePublicModelPrices(w http.ResponseWriter, r *http.Request) {
	prices, err := s.store.ListModelPrices(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", "failed to load model prices")
		return
	}
	out := make([]map[string]any, 0, len(prices))
	for _, price := range prices {
		out = append(out, map[string]any{
			"model":                           price.Model,
			"input_usd_per_million":           microsToUSD(price.InputMicrosPerMillion),
			"output_usd_per_million":          microsToUSD(price.OutputMicrosPerMillion),
			"cache_read_usd_per_million":      microsToUSD(price.CacheReadMicrosPerMillion),
			"cache_create_usd_per_million":    microsToUSD(price.CacheCreateMicrosPerMillion),
			"input_micros_per_million":        price.InputMicrosPerMillion,
			"output_micros_per_million":       price.OutputMicrosPerMillion,
			"cache_read_micros_per_million":   price.CacheReadMicrosPerMillion,
			"cache_create_micros_per_million": price.CacheCreateMicrosPerMillion,
			"updated_at":                      price.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
