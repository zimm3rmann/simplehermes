package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"simplehermes/internal/config"
	"simplehermes/internal/radio"
)

func TestRemoteServiceStateErrorReturnsRenderableFallback(t *testing.T) {
	cfg := config.Default()
	cfg.Mode = config.ModeClient

	service := NewRemoteService("test", cfg, filepath.Join(t.TempDir(), "config.json"))
	service.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		}),
	}

	state, err := service.State(context.Background())
	if err == nil {
		t.Fatalf("expected remote state error")
	}

	if state.App.ProxyHealthy {
		t.Fatalf("expected proxy to be marked unhealthy")
	}
	if state.Devices == nil {
		t.Fatalf("expected fallback devices to be renderable empty slice")
	}
	if len(state.Bands) == 0 {
		t.Fatalf("expected fallback band presets")
	}
	if len(state.Modes) == 0 {
		t.Fatalf("expected fallback mode presets")
	}
	if len(state.PowerLevels) == 0 {
		t.Fatalf("expected fallback power levels")
	}
	if len(state.Shortcuts) == 0 {
		t.Fatalf("expected fallback shortcuts")
	}
	if len(state.Messages) == 0 ||
		!strings.Contains(state.Messages[0].Text, "remote state:") ||
		!strings.Contains(state.Messages[0].Text, "dial failed") {
		t.Fatalf("expected remote state error message, got %#v", state.Messages)
	}
}

func TestRemoteServiceDispatchPreservesRemoteStateOnCommandError(t *testing.T) {
	remoteState := ViewState{
		App: AppMeta{
			Name:       "SimpleHermes",
			Version:    "remote",
			ActiveMode: string(config.ModeServer),
		},
		Devices: []radio.Device{
			{ID: "radio-1", Model: "Hermes Lite 2", Address: "192.0.2.10"},
		},
		Radio: defaultRadioModel().asView(),
		Messages: []Message{
			{Level: "error", Text: "enable transmit before keying PTT"},
		},
	}
	remoteState.Radio.Status = "Transmit is not armed."

	var gotCommand Command
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/commands" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer client-secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotCommand); err != nil {
			t.Fatalf("decode command: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(remoteState); err != nil {
			t.Fatalf("encode remote state: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.Mode = config.ModeClient
	cfg.RemoteBaseURL = server.URL
	cfg.RemoteAuthToken = "client-secret"
	service := NewRemoteService("test", cfg, filepath.Join(t.TempDir(), "config.json"))

	state, err := service.Dispatch(context.Background(), Command{Type: "setPTT", Enabled: true})
	if err == nil {
		t.Fatalf("expected remote command error")
	}
	if gotCommand.Type != "setPTT" || !gotCommand.Enabled {
		t.Fatalf("unexpected command: %#v", gotCommand)
	}

	if state.Radio.Status != "Transmit is not armed." {
		t.Fatalf("expected remote radio status, got %q", state.Radio.Status)
	}
	if len(state.Messages) != 1 || state.Messages[0].Text != "enable transmit before keying PTT" {
		t.Fatalf("expected remote error message, got %#v", state.Messages)
	}
	if state.App.ActiveMode != string(config.ModeClient) {
		t.Fatalf("expected client active mode, got %q", state.App.ActiveMode)
	}
	if state.App.RemoteMode != string(config.ModeServer) {
		t.Fatalf("expected remote server mode, got %q", state.App.RemoteMode)
	}
	if state.App.RemoteEndpoint != server.URL {
		t.Fatalf("expected remote endpoint %q, got %q", server.URL, state.App.RemoteEndpoint)
	}
	if !state.App.ProxyHealthy {
		t.Fatalf("expected proxy to remain healthy for reachable command rejection")
	}
	if len(state.Devices) != 1 || state.Devices[0].ID != "radio-1" {
		t.Fatalf("expected remote devices to be preserved, got %#v", state.Devices)
	}
	if len(state.Bands) == 0 || len(state.Modes) == 0 || len(state.PowerLevels) == 0 || len(state.Shortcuts) == 0 {
		t.Fatalf("expected adapted state to include fallback UI records")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
