package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"simplehermes/internal/config"
)

func TestRemoteAuthHandlerAllowsRequestsWhenTokenUnset(t *testing.T) {
	handler := remoteAuthHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "")

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRemoteAuthHandlerRequiresBearerTokenForAPI(t *testing.T) {
	handler := remoteAuthHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status without token = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status with token = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRemoteAuthHandlerAllowsRootWithoutToken(t *testing.T) {
	handler := remoteAuthHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "secret")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRemoteAuthValidAcceptsQueryTokenForWebsocketHandshake(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/audio/rx?access_token=secret", nil)
	if !remoteAuthValid(req, "secret") {
		t.Fatalf("expected query token to be accepted")
	}
}

func TestDesktopHandlerReportsLocalAPIBaseURL(t *testing.T) {
	app := New(config.Config{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	if err := app.startLocalServer(); err != nil {
		t.Fatalf("start local server: %v", err)
	}
	defer app.Shutdown(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/api/desktop", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}

	var info desktopInfo
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("decode desktop info: %v", err)
	}
	if !strings.HasPrefix(info.APIBaseURL, "http://127.0.0.1:") {
		t.Fatalf("apiBaseUrl = %q, want loopback URL", info.APIBaseURL)
	}
}

func TestStartupStartsLocalLoopbackAPI(t *testing.T) {
	app := New(config.Config{Mode: config.ModeLocal}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/state" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	app.Startup(context.Background())
	defer app.Shutdown(context.Background())

	app.mu.Lock()
	apiBaseURL := app.localAPIBaseURL
	app.mu.Unlock()
	if apiBaseURL == "" {
		t.Fatalf("local API URL was not set")
	}

	resp, err := http.Get(apiBaseURL + "/api/state")
	if err != nil {
		t.Fatalf("get local API state: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}
