package server

import (
	"slices"

	"github.com/yansircc/llm-broker/internal/domain"
	"github.com/yansircc/llm-broker/internal/driver"
)

type DriverViews struct {
	Catalog map[domain.Provider]driver.Descriptor
	OAuth   map[domain.Provider]driver.OAuthDriver
	Admin   map[domain.Provider]driver.AdminDriver
}

func sortedProviders[T any](drivers map[domain.Provider]T) []domain.Provider {
	providers := make([]domain.Provider, 0, len(drivers))
	for provider := range drivers {
		providers = append(providers, provider)
	}
	slices.Sort(providers)
	return providers
}

func (s *Server) oauthDriverByID(id string) (driver.OAuthDriver, bool) {
	drv, ok := s.oauthDrivers[domain.Provider(id)]
	return drv, ok
}
