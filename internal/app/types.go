package app

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"simplehermes/internal/bands"
	"simplehermes/internal/config"
	"simplehermes/internal/modes"
	"simplehermes/internal/radio"
)

type Service interface {
	State(ctx context.Context) (ViewState, error)
	Dispatch(ctx context.Context, cmd Command) (ViewState, error)
	UpdateSettings(ctx context.Context, update SettingsUpdate) (ViewState, error)
	HandleRXAudio(w http.ResponseWriter, r *http.Request)
	HandleTXAudio(w http.ResponseWriter, r *http.Request)
}

type Command struct {
	Type         string `json:"type"`
	DeviceID     string `json:"deviceId,omitempty"`
	BandID       string `json:"bandId,omitempty"`
	ModeID       string `json:"modeId,omitempty"`
	FrequencyMHz string `json:"frequencyMHz,omitempty"`
	StepHz       int64  `json:"stepHz,omitempty"`
	Steps        int    `json:"steps,omitempty"`
	PowerPercent int    `json:"powerPercent,omitempty"`
	Enabled      bool   `json:"enabled,omitempty"`
}

type SettingsUpdate struct {
	Mode              config.Mode `json:"mode"`
	ListenAddress     string      `json:"listenAddress"`
	RemoteBaseURL     string      `json:"remoteBaseUrl"`
	AccessibilityMode bool        `json:"accessibilityMode"`
}

type ViewState struct {
	App         AppMeta        `json:"app"`
	Settings    config.Public  `json:"settings"`
	Devices     []radio.Device `json:"devices"`
	Bands       []bands.Preset `json:"bands"`
	Modes       []modes.Preset `json:"modes"`
	PowerLevels []PowerLevel   `json:"powerLevels"`
	Shortcuts   []Shortcut     `json:"shortcuts"`
	Radio       RadioView      `json:"radio"`
	Messages    []Message      `json:"messages"`
}

type AppMeta struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ActiveMode      string `json:"activeMode"`
	RemoteMode      string `json:"remoteMode,omitempty"`
	RemoteEndpoint  string `json:"remoteEndpoint,omitempty"`
	ProxyHealthy    bool   `json:"proxyHealthy"`
	PendingRestart  bool   `json:"pendingRestart"`
	Accessibility   string `json:"accessibility"`
	VisualDirection string `json:"visualDirection"`
}

type Message struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

type PowerLevel struct {
	Percent int    `json:"percent"`
	Label   string `json:"label"`
}

type Shortcut struct {
	Keys        string `json:"keys"`
	Description string `json:"description"`
}

type RadioView struct {
	Connected         bool               `json:"connected"`
	Device            *radio.Device      `json:"device,omitempty"`
	BandID            string             `json:"bandId"`
	BandLabel         string             `json:"bandLabel"`
	ModeID            string             `json:"modeId"`
	ModeLabel         string             `json:"modeLabel"`
	FrequencyHz       int64              `json:"frequencyHz"`
	FrequencyMHz      string             `json:"frequencyMHz"`
	StepHz            int64              `json:"stepHz"`
	PowerPercent      int                `json:"powerPercent"`
	PowerLabel        string             `json:"powerLabel"`
	RXEnabled         bool               `json:"rxEnabled"`
	TXEnabled         bool               `json:"txEnabled"`
	PTT               bool               `json:"ptt"`
	Status            string             `json:"status"`
	LastAction        string             `json:"lastAction"`
	Capabilities      radio.Capabilities `json:"capabilities"`
	HardwareReadyText string             `json:"hardwareReadyText"`
}

type radioModel struct {
	device       *radio.Device
	connected    bool
	bandID       string
	modeID       string
	frequencyHz  int64
	stepHz       int64
	powerPercent int
	rxEnabled    bool
	txEnabled    bool
	ptt          bool
	status       string
	lastAction   string
	capabilities radio.Capabilities
}

func defaultRadioModel() radioModel {
	defaultBand, _ := bands.ByID("20m")

	return radioModel{
		bandID:       defaultBand.ID,
		modeID:       modes.Default().ID,
		frequencyHz:  defaultBand.DefaultHz,
		stepHz:       100,
		powerPercent: 10,
		rxEnabled:    true,
		txEnabled:    false,
		ptt:          false,
		status:       "Select a device and connect.",
		lastAction:   "Idle",
		capabilities: radio.Capabilities{
			DiscoveryReady: true,
			HardwareReady:  false,
			RXAudioReady:   false,
			TXAudioReady:   false,
			Summary:        "Discovery is available. Connect to a Hermes protocol 1 device to start live hardware transport and audio streaming.",
		},
	}
}

func (m radioModel) asView() RadioView {
	bandLabel := "Unassigned"
	if preset, ok := bands.ByID(m.bandID); ok {
		bandLabel = preset.Label
	}

	modeLabel := "Unassigned"
	if preset, ok := modes.ByID(m.modeID); ok {
		modeLabel = preset.Label
	}

	hardwareReadyText := "Discovery only"
	if m.capabilities.HardwareReady {
		hardwareReadyText = "Hardware transport live"
	}

	view := RadioView{
		Connected:         m.connected,
		BandID:            m.bandID,
		BandLabel:         bandLabel,
		ModeID:            m.modeID,
		ModeLabel:         modeLabel,
		FrequencyHz:       m.frequencyHz,
		FrequencyMHz:      formatMHz(m.frequencyHz),
		StepHz:            m.stepHz,
		PowerPercent:      m.powerPercent,
		PowerLabel:        powerLabelForPercent(m.powerPercent),
		RXEnabled:         m.rxEnabled,
		TXEnabled:         m.txEnabled,
		PTT:               m.ptt,
		Status:            m.status,
		LastAction:        m.lastAction,
		Capabilities:      m.capabilities,
		HardwareReadyText: hardwareReadyText,
	}

	if m.device != nil {
		device := *m.device
		view.Device = &device
	}

	return view
}

func formatMHz(hz int64) string {
	return fmt.Sprintf("%.6f", float64(hz)/1_000_000.0)
}

func parseMHz(input string) (int64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(input), 64)
	if err != nil {
		return 0, fmt.Errorf("enter frequency as MHz, for example 14.200000")
	}
	if value <= 0 {
		return 0, fmt.Errorf("frequency must be positive")
	}
	return int64(value * 1_000_000), nil
}

func clampPower(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
