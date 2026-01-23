package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/akeyless-community/cred-server/internal/config"
	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/akeyless-community/cred-server/internal/handler"
	"github.com/akeyless-community/cred-server/internal/producer"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configure logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("LOG_FORMAT") != "json" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Load configuration
	cfg := config.Load()

	log.Info().
		Str("port", cfg.Port).
		Str("gateway", cfg.AkeylessGateway).
		Msg("Starting Cred Server")

	// Initialize producer registry (in-memory for demo)
	registry := producer.NewRegistry()

	// Create router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(constants.HTTP__TIMEOUT__REQUEST__SECONDS__DEFAULT))

	// Health endpoints
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Sync endpoints for Akeyless custom producer
	syncHandler := handler.NewSyncHandler(registry, cfg.AllowedAccessIDs)
	r.Post("/sync/create", syncHandler.Create)
	r.Post("/sync/revoke", syncHandler.Revoke)
	r.Post("/sync/rotate", syncHandler.Rotate)

	// Management API
	mgmtHandler := handler.NewManagementHandler(registry, cfg)
	proxyHandler := handler.NewProxyHandler()
	analyzerHandler := handler.NewAnalyzerHandler(registry)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/producers", mgmtHandler.CreateProducer)
		r.Get("/producers", mgmtHandler.ListProducers)
		r.Get("/producers/{id}", mgmtHandler.GetProducer)
		r.Delete("/producers/{id}", mgmtHandler.DeleteProducer)
		r.Post("/producers/{id}/test", mgmtHandler.TestProducer)
		r.Post("/producers/{id}/deploy", mgmtHandler.DeployToAkeyless)

		// Recording proxy session endpoints (SSOT discovery)
		r.Post("/sessions", proxyHandler.StartSession)
		r.Get("/sessions/{sessionID}", proxyHandler.GetSession)
		r.Get("/sessions/{sessionID}/debug", proxyHandler.DebugSession)
		r.Post("/sessions/{sessionID}/stop", proxyHandler.StopSession)

		// HAR file analysis (alternative to proxy recording)
		r.Post("/analyze", analyzerHandler.AnalyzeHAR)
		r.Post("/analyze/create", analyzerHandler.CreateFromAnalysis)
	})

	// Proxy routes - must be outside /api/v1 to capture all paths
	r.HandleFunc("/proxy/{sessionID}/*", proxyHandler.ProxyRequest)
	r.HandleFunc("/proxy/{sessionID}", proxyHandler.ProxyRequest)

	// Serve static UI
	r.Get("/", handler.ServeUI)
	r.Handle("/ui/*", http.StripPrefix("/ui", handler.UIFileServer()))

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	log.Info().Str("port", cfg.Port).Msg("Server started")
	<-done
	log.Info().Msg("Server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), constants.HTTP__TIMEOUT__SHUTDOWN__SECONDS__GRACEFUL)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server shutdown failed")
	}
	log.Info().Msg("Server stopped")
}
