package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
