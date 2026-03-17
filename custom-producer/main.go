package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/akeyless-community/ansible-credential-rotation/internal/producer"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	validationURL = "https://auth.akeyless.io/validate-producer-credentials"
	credsHeader   = "AkeylessCreds"
)

func main() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().Timestamp().Caller().Logger()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	allowedAccessID := os.Getenv("AKEYLESS_ACCESS_ID")
	skipAuth := os.Getenv("SKIP_AUTH") == "true"

	p := producer.New()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Akeyless custom producer sync endpoints
	r.Route("/sync", func(r chi.Router) {
		if !skipAuth {
			r.Use(akeylessAuthMiddleware(allowedAccessID))
		} else {
			log.Warn().Msg("SKIP_AUTH=true - authentication disabled (testing mode)")
		}
		r.Post("/create", handleCreate(p))
		r.Post("/revoke", handleRevoke(p))
		r.Post("/rotate", handleRotate(p))
	})

	// Webhook receiver for Akeyless Event Center notifications
	// In production, this would be the Ansible EDA endpoint
	r.Post("/webhook/rotation-event", handleWebhookEvent())

	log.Info().Str("port", port).Str("access_id", allowedAccessID).Msg("starting ansible credential rotation producer")
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}

// akeylessAuthMiddleware validates the AkeylessCreds JWT header against
// the Akeyless auth service.
func akeylessAuthMiddleware(allowedAccessID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds := r.Header.Get(credsHeader)
			if creds == "" {
				http.Error(w, "missing AkeylessCreds header", http.StatusUnauthorized)
				return
			}

			// Skip validation for dry-run requests (access ID "p-custom")
			if allowedAccessID == "" {
				log.Warn().Msg("AKEYLESS_ACCESS_ID not set, skipping auth validation")
				next.ServeHTTP(w, r)
				return
			}

			if err := validateCreds(r.Context(), creds, allowedAccessID); err != nil {
				log.Error().Err(err).Msg("authentication failed")
				http.Error(w, "invalid credentials", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validateCreds(ctx context.Context, creds, expectedAccessID string) error {
	body, _ := json.Marshal(map[string]string{
		"creds":              creds,
		"expected_access_id": expectedAccessID,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validationURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create validation request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validation failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func handleRotate(p *producer.Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req producer.RotateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		resp, err := p.Rotate(r.Context(), &req)
		if err != nil {
			log.Error().Err(err).Msg("rotate failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleCreate(p *producer.Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req producer.CreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		resp, err := p.Create(r.Context(), &req)
		if err != nil {
			log.Error().Err(err).Msg("create failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleWebhookEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		var event map[string]interface{}
		if err := json.Unmarshal(body, &event); err != nil {
			log.Warn().Str("raw_body", string(body)).Msg("received non-JSON webhook event")
		} else {
			log.Info().Interface("event", event).Msg("received Akeyless rotation event")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "received"}`))
	}
}

func handleRevoke(p *producer.Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req producer.RevokeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		resp, err := p.Revoke(r.Context(), &req)
		if err != nil {
			log.Error().Err(err).Msg("revoke failed")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
