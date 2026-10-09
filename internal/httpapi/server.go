package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AGlElectronics/suppervisor/internal/app"
	"github.com/AGlElectronics/suppervisor/internal/config"
	"github.com/AGlElectronics/suppervisor/internal/domain"
	"github.com/AGlElectronics/suppervisor/internal/store"
)

type VerifyToken func(context.Context, string) error

type Server struct {
	Service     app.Service
	Store       *store.Postgres
	Config      config.Config
	VerifyToken VerifyToken
	Logger      *slog.Logger
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.Ready(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database is not ready")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/public/config", s.publicConfig)
	mux.HandleFunc("GET /api/v1/status", s.status)
	mux.HandleFunc("GET /api/v1/bootstrap", s.bootstrap)
	mux.HandleFunc("POST /api/v1/plans", s.plan)
	mux.HandleFunc("PATCH /api/v1/decisions/{id}", s.decision)
	mux.HandleFunc("POST /api/v1/ratings", s.rate)
	mux.HandleFunc("POST /api/v1/preferences/events/{id}/revert", s.revertPreference)
	return s.logging(s.cors(s.auth(mux)))
}

func (s Server) publicConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authRequired": !s.Config.AuthDevBypass,
		"oidcIssuer":   s.Config.OIDCIssuer,
		"oidcClientId": s.Config.OIDCClientID,
	})
}

func (s Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "Suppervisor", "tagline": "Dinner, handled.",
		"environment": s.Config.Environment,
		"ollama":      s.Config.OllamaURL != "",
		"telegram":    s.Config.TelegramToken != "",
		"auth":        map[bool]string{true: "development bypass", false: "Keycloak OIDC"}[s.Config.AuthDevBypass],
	})
}

func (s Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.Store.Profiles(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	recipes, err := s.Store.Recipes(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	decisions, err := s.Store.Decisions(r.Context(), 50)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	decisionEvents, err := s.Store.DecisionEvents(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	events, err := s.Store.PreferenceEvents(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	ratings, err := s.Store.Ratings(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"profiles": profiles, "recipes": recipes, "decisions": decisions, "decisionEvents": decisionEvents, "preferenceEvents": events, "ratings": ratings})
}

func (s Server) plan(w http.ResponseWriter, r *http.Request) {
	var input app.PlanInput
	if !decode(w, r, &input) {
		return
	}
	decisions, err := s.Service.Plan(r.Context(), input)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"decisions": decisions})
}

func (s Server) decision(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status     string `json:"status"`
		Correction string `json:"correction"`
	}
	if !decode(w, r, &input) {
		return
	}
	if err := s.Service.SetStatus(r.Context(), r.PathValue("id"), input.Status, input.Correction); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s Server) rate(w http.ResponseWriter, r *http.Request) {
	var rating domain.Rating
	if !decode(w, r, &rating) {
		return
	}
	events, err := s.Service.Rate(r.Context(), rating)
	if err != nil {
		writeError(w, 422, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"preferenceEvents": events})
}

func (s Server) revertPreference(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.RevertPreferenceEvent(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/health/") || r.URL.Path == "/api/v1/public/config" || r.Method == http.MethodOptions || s.Config.AuthDevBypass {
			next.ServeHTTP(w, r)
			return
		}
		header := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if header == "" || header == r.Header.Get("Authorization") {
			writeError(w, http.StatusUnauthorized, "valid bearer token required")
			return
		}
		if s.VerifyToken == nil {
			writeError(w, http.StatusServiceUnavailable, "OIDC verifier unavailable")
			return
		}
		if s.VerifyToken(r.Context(), header) != nil {
			writeError(w, http.StatusUnauthorized, "valid bearer token required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == s.Config.WebOrigin {
			w.Header().Set("Access-Control-Allow-Origin", s.Config.WebOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s Server) logging(next http.Handler) http.Handler {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.InfoContext(r.Context(), "http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	if message == "" {
		message = "request failed"
	}
	writeJSON(w, status, map[string]string{"error": message})
}
