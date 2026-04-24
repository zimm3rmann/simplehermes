package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"simplehermes/internal/app"
	"simplehermes/internal/radio"
)

func TestCommandsWritesStateOnServiceError(t *testing.T) {
	service := &fakeService{
		state: app.ViewState{
			App:     app.AppMeta{Name: "SimpleHermes", ActiveMode: "local"},
			Devices: []radio.Device{},
			Messages: []app.Message{
				{Level: "error", Text: "enable transmit before keying PTT"},
			},
		},
		dispatchErr: errors.New("rejected"),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(`{"type":"setPTT","enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewServer("test", service).Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if service.gotCommand.Type != "setPTT" || !service.gotCommand.Enabled {
		t.Fatalf("unexpected command: %#v", service.gotCommand)
	}

	var got app.ViewState
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Text != "enable transmit before keying PTT" {
		t.Fatalf("unexpected state messages: %#v", got.Messages)
	}
}

func TestCommandsRejectsMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/commands", strings.NewReader(`{`))
	rec := httptest.NewRecorder()

	NewServer("test", &fakeService{}).Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
}

type fakeService struct {
	state       app.ViewState
	dispatchErr error
	gotCommand  app.Command
}

func (f *fakeService) State(context.Context) (app.ViewState, error) {
	return f.state, nil
}

func (f *fakeService) Dispatch(_ context.Context, cmd app.Command) (app.ViewState, error) {
	f.gotCommand = cmd
	return f.state, f.dispatchErr
}

func (f *fakeService) UpdateSettings(context.Context, app.SettingsUpdate) (app.ViewState, error) {
	return f.state, nil
}

func (f *fakeService) HandleRXAudio(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (f *fakeService) HandleTXAudio(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
