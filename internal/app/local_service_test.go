package app

import (
	"context"
	"path/filepath"
	"testing"

	"simplehermes/internal/config"
	"simplehermes/internal/radio"
)

func TestLocalServiceDisconnectClearsHardwareCapabilities(t *testing.T) {
	device := radio.Device{
		ID:            "radio-1",
		Model:         "Hermes Lite 2",
		Address:       "192.0.2.15",
		InterfaceName: "eth0",
		Protocol:      "protocol1",
	}
	service := NewLocalService("test", config.Default(), filepath.Join(t.TempDir(), "config.json"), localTestDriver{
		session: &localTestSession{
			snapshot: radio.Snapshot{
				Connected: true,
				Device:    &device,
				BandID:    "20m",
				ModeID:    "usb",
				Capabilities: radio.Capabilities{
					DiscoveryReady: true,
					HardwareReady:  true,
					RXAudioReady:   true,
					TXAudioReady:   true,
					Summary:        "live",
				},
			},
		},
	})
	service.devices = []radio.Device{device}

	state, err := service.Dispatch(context.Background(), Command{Type: "connect", DeviceID: device.ID})
	if err != nil {
		t.Fatalf("connect returned error: %v", err)
	}
	if !state.Radio.Capabilities.HardwareReady {
		t.Fatalf("expected connected state to report hardware ready")
	}

	state, err = service.Dispatch(context.Background(), Command{Type: "disconnect"})
	if err != nil {
		t.Fatalf("disconnect returned error: %v", err)
	}

	if state.Radio.Connected {
		t.Fatalf("expected disconnected radio state")
	}
	if state.Radio.Capabilities.HardwareReady || state.Radio.Capabilities.RXAudioReady || state.Radio.Capabilities.TXAudioReady {
		t.Fatalf("disconnect left live capabilities: %#v", state.Radio.Capabilities)
	}
	if state.Radio.HardwareReadyText != "Discovery only" {
		t.Fatalf("HardwareReadyText = %q", state.Radio.HardwareReadyText)
	}
}

type localTestDriver struct {
	session radio.Session
}

func (d localTestDriver) Discover(context.Context) ([]radio.Device, error) {
	return nil, nil
}

func (d localTestDriver) Connect(context.Context, radio.Device, radio.SessionOptions) (radio.Session, error) {
	return d.session, nil
}

type localTestSession struct {
	snapshot radio.Snapshot
}

func (s *localTestSession) Snapshot() radio.Snapshot                  { return s.snapshot }
func (s *localTestSession) SetBand(context.Context, string) error     { return nil }
func (s *localTestSession) SetMode(context.Context, string) error     { return nil }
func (s *localTestSession) SetFrequency(context.Context, int64) error { return nil }
func (s *localTestSession) SetStep(context.Context, int64) error      { return nil }
func (s *localTestSession) SetPower(context.Context, int) error       { return nil }
func (s *localTestSession) SetRXEnabled(context.Context, bool) error  { return nil }
func (s *localTestSession) SetTXEnabled(context.Context, bool) error  { return nil }
func (s *localTestSession) SetPTT(context.Context, bool) error        { return nil }
func (s *localTestSession) SubscribeRXAudio(context.Context) (<-chan []float32, error) {
	ch := make(chan []float32)
	close(ch)
	return ch, nil
}
func (s *localTestSession) WriteTXAudio(context.Context, []float32) error { return nil }
func (s *localTestSession) Diagnostics() radio.Diagnostics {
	return radio.Diagnostics{Connected: s.snapshot.Connected, Transport: "test"}
}
func (s *localTestSession) Close() error {
	s.snapshot.Connected = false
	return nil
}
