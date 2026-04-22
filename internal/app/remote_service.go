package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"simplehermes/internal/bands"
	"simplehermes/internal/config"
	"simplehermes/internal/modes"
)

type RemoteService struct {
	mu             sync.RWMutex
	version        string
	activeMode     config.Mode
	configPath     string
	config         config.Config
	httpClient     *http.Client
	lastState      ViewState
	pendingRestart bool
}

func NewRemoteService(version string, cfg config.Config, configPath string) *RemoteService {
	cfg.Normalize()

	service := &RemoteService{
		version:    version,
		activeMode: cfg.Mode,
		configPath: configPath,
		config:     cfg,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		lastState: ViewState{
			App: AppMeta{
				Name:            "SimpleHermes",
				Version:         version,
				ActiveMode:      string(config.ModeClient),
				RemoteEndpoint:  cfg.RemoteBaseURL,
				ProxyHealthy:    false,
				Accessibility:   "Desktop webview shell with semantic HTML, keyboard-first controls, and live announcement regions.",
				VisualDirection: "Industrial station console with warm neutrals and signal-orange accents.",
			},
			Settings:    cfg.Public(),
			Bands:       bands.All(),
			Modes:       modes.All(),
			PowerLevels: powerLevels(),
			Shortcuts:   shortcuts(),
			Radio:       defaultRadioModel().asView(),
			Messages: []Message{
				{
					Level: "info",
					Text:  "Client mode proxies a remote SimpleHermes server while keeping the same desktop UI and keyboard model.",
				},
			},
		},
	}

	return service
}

func (s *RemoteService) State(ctx context.Context) (ViewState, error) {
	state, err := s.fetchRemoteState(ctx)
	if err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.lastState.App.ProxyHealthy = false
		s.pushMessageLocked("error", fmt.Sprintf("remote state: %v", err))
		return s.lastState, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastState = state
	return state, nil
}

func (s *RemoteService) Dispatch(ctx context.Context, cmd Command) (ViewState, error) {
	body, err := json.Marshal(cmd)
	if err != nil {
		return ViewState{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.RemoteBaseURL+"/api/commands", bytes.NewReader(body))
	if err != nil {
		return ViewState{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ViewState{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ViewState{}, fmt.Errorf("remote command failed with %s", resp.Status)
	}

	var remoteState ViewState
	if err := json.NewDecoder(resp.Body).Decode(&remoteState); err != nil {
		return ViewState{}, err
	}

	state := s.adaptRemoteState(remoteState)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastState = state
	return state, nil
}

func (s *RemoteService) UpdateSettings(_ context.Context, update SettingsUpdate) (ViewState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.config
	next.Mode = update.Mode
	next.ListenAddress = update.ListenAddress
	next.RemoteBaseURL = update.RemoteBaseURL
	next.AccessibilityMode = update.AccessibilityMode
	next.Normalize()

	s.pendingRestart = s.activeMode != next.Mode || s.config.ListenAddress != next.ListenAddress
	s.config = next
	s.lastState.Settings = next.Public()
	s.lastState.App.RemoteEndpoint = next.RemoteBaseURL
	s.lastState.App.PendingRestart = s.pendingRestart

	if err := config.Save(s.configPath, next); err != nil {
		s.pushMessageLocked("error", fmt.Sprintf("save settings: %v", err))
		return s.lastState, err
	}

	if s.pendingRestart {
		s.pushMessageLocked("warning", "Settings saved. Restart the app to apply the new mode or listen address.")
	} else {
		s.pushMessageLocked("info", "Settings saved.")
	}

	return s.lastState, nil
}

func (s *RemoteService) fetchRemoteState(ctx context.Context) (ViewState, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.RemoteBaseURL+"/api/state", nil)
	if err != nil {
		return ViewState{}, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ViewState{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ViewState{}, fmt.Errorf("remote state failed with %s", resp.Status)
	}

	var remoteState ViewState
	if err := json.NewDecoder(resp.Body).Decode(&remoteState); err != nil {
		return ViewState{}, err
	}

	return s.adaptRemoteState(remoteState), nil
}

func (s *RemoteService) adaptRemoteState(remoteState ViewState) ViewState {
	remoteMode := remoteState.App.ActiveMode
	remoteState.App.ActiveMode = string(config.ModeClient)
	remoteState.App.RemoteMode = remoteMode
	remoteState.App.RemoteEndpoint = s.config.RemoteBaseURL
	remoteState.App.ProxyHealthy = true
	remoteState.App.PendingRestart = s.pendingRestart
	remoteState.Settings = s.config.Public()
	return remoteState
}

func (s *RemoteService) pushMessageLocked(level, text string) {
	s.lastState.Messages = append([]Message{{Level: level, Text: text}}, s.lastState.Messages...)
	if len(s.lastState.Messages) > 6 {
		s.lastState.Messages = s.lastState.Messages[:6]
	}
}
