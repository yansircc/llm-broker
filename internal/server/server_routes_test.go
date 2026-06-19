package server

import (
	"net/http"
	"testing"

	"github.com/yansircc/llm-broker/internal/auth"
	"github.com/yansircc/llm-broker/internal/config"
	"github.com/yansircc/llm-broker/internal/store"
)

func TestRegisterRoutesDoesNotPanic(t *testing.T) {
	st := store.NewMockStore()
	srv := &Server{
		cfg:    &config.Config{},
		store:  st,
		authMw: auth.NewMiddleware("test-token", st),
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registerRoutes panic: %v", r)
		}
	}()
	srv.registerRoutes(http.NewServeMux())
}
