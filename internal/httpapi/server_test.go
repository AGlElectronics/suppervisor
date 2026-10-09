package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AGlElectronics/suppervisor/internal/config"
)

func TestPublicConfigIsAvailableBeforeOIDCLogin(t *testing.T) {
	server := Server{Config: config.Config{AuthDevBypass: false, OIDCIssuer: "https://id.example.test/realms/home", OIDCClientID: "suppervisor"}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/config", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected public config, got %d", response.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["authRequired"] != true || body["oidcIssuer"] == "" {
		t.Fatalf("unexpected config: %#v", body)
	}
}

func TestProtectedRoutesRejectMissingToken(t *testing.T) {
	server := Server{Config: config.Config{AuthDevBypass: false}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}
