package radio

import "context"

type Device struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Model              string `json:"model"`
	Address            string `json:"address"`
	InterfaceName      string `json:"interfaceName"`
	Protocol           string `json:"protocol"`
	SoftwareVersion    string `json:"softwareVersion"`
	Status             string `json:"status"`
	SupportedReceivers int    `json:"supportedReceivers"`
}

type Capabilities struct {
	DiscoveryReady bool   `json:"discoveryReady"`
	HardwareReady  bool   `json:"hardwareReady"`
	RXAudioReady   bool   `json:"rxAudioReady"`
	TXAudioReady   bool   `json:"txAudioReady"`
	Summary        string `json:"summary"`
}

type Diagnostics struct {
	Connected       bool   `json:"connected"`
	Transport       string `json:"transport"`
	LocalAddress    string `json:"localAddress"`
	RemoteAddress   string `json:"remoteAddress"`
	StartedAt       string `json:"startedAt"`
	LastControl     string `json:"lastControl"`
	LastError       string `json:"lastError"`
	SendPackets     uint64 `json:"sendPackets"`
	StartCommands   uint64 `json:"startCommands"`
	StopCommands    uint64 `json:"stopCommands"`
	ControlFrames   uint64 `json:"controlFrames"`
	FrequencyFrames uint64 `json:"frequencyFrames"`
	LastTXFrequency int64  `json:"lastTxFrequencyHz"`
	LastRXFrequency int64  `json:"lastRxFrequencyHz"`
	RXPackets       uint64 `json:"rxPackets"`
	RXFrames        uint64 `json:"rxFrames"`
	RXAudioFrames   uint64 `json:"rxAudioFrames"`
	RXAudioSamples  uint64 `json:"rxAudioSamples"`
	RXAudioDrops    uint64 `json:"rxAudioDrops"`
	RXSubscribers   uint64 `json:"rxSubscribers"`
	TXAudioFrames   uint64 `json:"txAudioFrames"`
	TXAudioSamples  uint64 `json:"txAudioSamples"`
	TXIQSamples     uint64 `json:"txIqSamples"`
}

type Snapshot struct {
	Connected    bool         `json:"connected"`
	Device       *Device      `json:"device,omitempty"`
	BandID       string       `json:"bandId"`
	ModeID       string       `json:"modeId"`
	FrequencyHz  int64        `json:"frequencyHz"`
	StepHz       int64        `json:"stepHz"`
	PowerPercent int          `json:"powerPercent"`
	RXEnabled    bool         `json:"rxEnabled"`
	TXEnabled    bool         `json:"txEnabled"`
	PTT          bool         `json:"ptt"`
	LastAction   string       `json:"lastAction"`
	Status       string       `json:"status"`
	Capabilities Capabilities `json:"capabilities"`
}

type SessionOptions struct {
	BandID       string
	ModeID       string
	FrequencyHz  int64
	StepHz       int64
	PowerPercent int
	RXEnabled    bool
	TXEnabled    bool
}

type Driver interface {
	Discover(ctx context.Context) ([]Device, error)
	Connect(ctx context.Context, device Device, options SessionOptions) (Session, error)
}

type Session interface {
	Snapshot() Snapshot
	SetBand(ctx context.Context, bandID string) error
	SetMode(ctx context.Context, modeID string) error
	SetFrequency(ctx context.Context, hz int64) error
	SetStep(ctx context.Context, hz int64) error
	SetPower(ctx context.Context, percent int) error
	SetRXEnabled(ctx context.Context, enabled bool) error
	SetTXEnabled(ctx context.Context, enabled bool) error
	SetPTT(ctx context.Context, enabled bool) error
	SubscribeRXAudio(ctx context.Context) (<-chan []float32, error)
	WriteTXAudio(ctx context.Context, samples []float32) error
	Diagnostics() Diagnostics
	Close() error
}
