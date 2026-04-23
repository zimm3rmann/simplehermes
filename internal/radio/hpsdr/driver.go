package hpsdr

import (
	"context"
	"fmt"
	"sync"

	"simplehermes/internal/radio"
)

type Driver struct{}

func NewDriver() *Driver {
	return &Driver{}
}

func (d *Driver) Discover(ctx context.Context) ([]radio.Device, error) {
	return discover(ctx)
}

func (d *Driver) Connect(ctx context.Context, device radio.Device, options radio.SessionOptions) (radio.Session, error) {
	if device.Protocol == "protocol1" {
		return newProtocol1Session(ctx, device, options)
	}

	snapshot := radio.Snapshot{
		Connected:    false,
		Device:       &device,
		BandID:       options.BandID,
		ModeID:       options.ModeID,
		FrequencyHz:  options.FrequencyHz,
		StepHz:       options.StepHz,
		PowerPercent: options.PowerPercent,
		RXEnabled:    options.RXEnabled,
		TXEnabled:    options.TXEnabled,
		PTT:          false,
		LastAction:   "Connect rejected",
		Status:       fmt.Sprintf("Discovery found %s at %s, but protocol %s is not implemented yet.", device.Model, device.Address, device.Protocol),
		Capabilities: radio.Capabilities{
			DiscoveryReady: true,
			HardwareReady:  false,
			RXAudioReady:   false,
			TXAudioReady:   false,
			Summary:        "Only Hermes protocol 1 sessions are implemented in this build.",
		},
	}

	return &stubSession{snapshot: snapshot}, nil
}

type stubSession struct {
	mu       sync.RWMutex
	snapshot radio.Snapshot
}

func (s *stubSession) Snapshot() radio.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := s.snapshot
	if out.Device != nil {
		device := *out.Device
		out.Device = &device
	}
	return out
}

func (s *stubSession) SetBand(_ context.Context, bandID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.BandID = bandID
	s.snapshot.LastAction = "Band preset selected"
	return nil
}

func (s *stubSession) SetMode(_ context.Context, modeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.ModeID = modeID
	s.snapshot.LastAction = "Operating mode updated"
	return nil
}

func (s *stubSession) SetFrequency(_ context.Context, hz int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.FrequencyHz = hz
	s.snapshot.LastAction = "Frequency updated"
	return nil
}

func (s *stubSession) SetStep(_ context.Context, hz int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.StepHz = hz
	s.snapshot.LastAction = "Step size updated"
	return nil
}

func (s *stubSession) SetPower(_ context.Context, percent int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.PowerPercent = percent
	s.snapshot.LastAction = "Power level updated"
	return nil
}

func (s *stubSession) SetRXEnabled(_ context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.RXEnabled = enabled
	s.snapshot.LastAction = "Receive state changed"
	return nil
}

func (s *stubSession) SetTXEnabled(_ context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.TXEnabled = enabled
	if !enabled {
		s.snapshot.PTT = false
	}
	s.snapshot.LastAction = "Transmit armed state changed"
	return nil
}

func (s *stubSession) SetPTT(_ context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled && !s.snapshot.TXEnabled {
		return fmt.Errorf("enable transmit before asserting PTT")
	}
	s.snapshot.PTT = enabled
	s.snapshot.LastAction = "PTT state changed"
	return nil
}

func (s *stubSession) SubscribeRXAudio(ctx context.Context) (<-chan []float32, error) {
	ch := make(chan []float32)
	close(ch)
	if err := ctx.Err(); err != nil {
		return ch, err
	}
	return ch, fmt.Errorf("audio streaming is not available for this session")
}

func (s *stubSession) WriteTXAudio(ctx context.Context, _ []float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("audio streaming is not available for this session")
}

func (s *stubSession) Diagnostics() radio.Diagnostics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return radio.Diagnostics{
		Connected: s.snapshot.Connected,
		Transport: "unsupported",
		LastError: s.snapshot.Status,
	}
}

func (s *stubSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Connected = false
	s.snapshot.PTT = false
	s.snapshot.LastAction = "Session closed"
	s.snapshot.Status = "Disconnected from Hermes session."
	return nil
}
