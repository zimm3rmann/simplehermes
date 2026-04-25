package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"simplehermes/internal/bands"
	"simplehermes/internal/config"
	"simplehermes/internal/modes"
	"simplehermes/internal/radio"
)

type LocalService struct {
	mu             sync.RWMutex
	version        string
	activeMode     config.Mode
	configPath     string
	config         config.Config
	driver         radio.Driver
	devices        []radio.Device
	session        radio.Session
	radio          radioModel
	messages       []Message
	pendingRestart bool
}

func NewLocalService(version string, cfg config.Config, configPath string, driver radio.Driver) *LocalService {
	cfg.Normalize()

	return &LocalService{
		version:    version,
		activeMode: cfg.Mode,
		configPath: configPath,
		config:     cfg,
		driver:     driver,
		radio:      defaultRadioModel(),
		messages: []Message{
			{
				Level: "info",
				Text:  "Desktop shell is the primary target. Hermes discovery, protocol 1 transport, and basic RX/TX audio streaming are implemented; real station validation is the next milestone.",
			},
		},
	}
}

func (s *LocalService) State(_ context.Context) (ViewState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applySessionSnapshotLocked()
	return s.snapshotLocked(), nil
}

func (s *LocalService) Dispatch(ctx context.Context, cmd Command) (ViewState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch cmd.Type {
	case "discover":
		devices, err := s.driver.Discover(ctx)
		if err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
		s.devices = devices
		if len(devices) == 0 {
			s.pushMessage("warning", "No Hermes Lite devices were found on the active network interfaces.")
		} else {
			s.pushMessage("info", fmt.Sprintf("Discovery completed. Found %d Hermes device(s).", len(devices)))
		}
	case "connect":
		device, ok := s.findDeviceLocked(cmd.DeviceID)
		if !ok {
			err := fmt.Errorf("selected device was not found")
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
		if s.session != nil {
			_ = s.session.Close()
			s.session = nil
		}

		session, err := s.driver.Connect(ctx, device, radio.SessionOptions{
			BandID:       s.radio.bandID,
			ModeID:       s.radio.modeID,
			FrequencyHz:  s.radio.frequencyHz,
			StepHz:       s.radio.stepHz,
			PowerPercent: s.radio.powerPercent,
			RXEnabled:    s.radio.rxEnabled,
			TXEnabled:    s.radio.txEnabled,
		})
		if err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}

		s.session = session
		s.applySessionSnapshotLocked()
		s.pushMessage("info", fmt.Sprintf("Connected to %s at %s.", device.Model, device.Address))
	case "disconnect":
		if s.session != nil {
			_ = s.session.Close()
			s.session = nil
		}
		s.radio.connected = false
		s.radio.device = nil
		s.radio.ptt = false
		s.radio.capabilities = disconnectedCapabilities()
		s.radio.status = "Disconnected from device."
		s.radio.lastAction = "Disconnected"
	case "setBand":
		if err := s.setBandLocked(ctx, cmd.BandID); err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
	case "cycleBand":
		if err := s.setBandLocked(ctx, bands.Next(s.radio.bandID).ID); err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
	case "setMode":
		if err := s.setModeLocked(ctx, cmd.ModeID); err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
	case "cycleMode":
		if err := s.setModeLocked(ctx, modes.Next(s.radio.modeID).ID); err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
	case "setFrequency":
		hz, err := parseMHz(cmd.FrequencyMHz)
		if err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
		if err := s.setFrequencyLocked(ctx, hz); err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
	case "nudgeFrequency":
		delta := int64(cmd.Steps) * s.radio.stepHz
		if err := s.setFrequencyLocked(ctx, s.radio.frequencyHz+delta); err != nil {
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
	case "setStep":
		if cmd.StepHz <= 0 {
			err := fmt.Errorf("step size must be positive")
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
		s.radio.stepHz = cmd.StepHz
		s.radio.lastAction = "Step size updated"
		if s.session != nil {
			if err := s.session.SetStep(ctx, cmd.StepHz); err != nil {
				s.pushMessage("error", err.Error())
				return s.snapshotLocked(), err
			}
			s.applySessionSnapshotLocked()
		}
	case "setPower":
		s.radio.powerPercent = clampPower(cmd.PowerPercent)
		s.radio.lastAction = "Power level updated"
		if s.session != nil {
			if err := s.session.SetPower(ctx, s.radio.powerPercent); err != nil {
				s.pushMessage("error", err.Error())
				return s.snapshotLocked(), err
			}
			s.applySessionSnapshotLocked()
		}
	case "cyclePower":
		next := nextPowerLevel(s.radio.powerPercent)
		s.radio.powerPercent = next.Percent
		s.radio.lastAction = "Power level updated"
		if s.session != nil {
			if err := s.session.SetPower(ctx, s.radio.powerPercent); err != nil {
				s.pushMessage("error", err.Error())
				return s.snapshotLocked(), err
			}
			s.applySessionSnapshotLocked()
		}
	case "setRX":
		s.radio.rxEnabled = cmd.Enabled
		s.radio.lastAction = "Receive state changed"
		if s.session != nil {
			if err := s.session.SetRXEnabled(ctx, cmd.Enabled); err != nil {
				s.pushMessage("error", err.Error())
				return s.snapshotLocked(), err
			}
			s.applySessionSnapshotLocked()
		}
	case "setTX":
		s.radio.txEnabled = cmd.Enabled
		if !cmd.Enabled {
			s.radio.ptt = false
		}
		s.radio.lastAction = "Transmit armed state changed"
		if s.session != nil {
			if err := s.session.SetTXEnabled(ctx, cmd.Enabled); err != nil {
				s.pushMessage("error", err.Error())
				return s.snapshotLocked(), err
			}
			s.applySessionSnapshotLocked()
		}
	case "setPTT":
		if cmd.Enabled && !s.radio.txEnabled {
			err := fmt.Errorf("enable transmit before keying PTT")
			s.pushMessage("error", err.Error())
			return s.snapshotLocked(), err
		}
		s.radio.ptt = cmd.Enabled
		s.radio.lastAction = "PTT state changed"
		if s.session != nil {
			if err := s.session.SetPTT(ctx, cmd.Enabled); err != nil {
				s.pushMessage("error", err.Error())
				return s.snapshotLocked(), err
			}
			s.applySessionSnapshotLocked()
		}
	default:
		err := fmt.Errorf("unknown command type %q", cmd.Type)
		s.pushMessage("error", err.Error())
		return s.snapshotLocked(), err
	}

	return s.snapshotLocked(), nil
}

func (s *LocalService) UpdateSettings(_ context.Context, update SettingsUpdate) (ViewState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.config
	next.Mode = update.Mode
	next.ListenAddress = update.ListenAddress
	next.RemoteBaseURL = update.RemoteBaseURL
	if update.ClearRemoteAuthToken {
		next.RemoteAuthToken = ""
	} else if token := strings.TrimSpace(update.RemoteAuthToken); token != "" {
		next.RemoteAuthToken = token
	}
	next.AccessibilityMode = update.AccessibilityMode
	next.AudioInputDeviceID = update.AudioInputDeviceID
	next.AudioOutputDeviceID = update.AudioOutputDeviceID
	next.Normalize()

	authChanged := s.config.RemoteAuthToken != next.RemoteAuthToken
	s.pendingRestart = s.activeMode != next.Mode ||
		s.config.ListenAddress != next.ListenAddress ||
		(s.activeMode == config.ModeServer && authChanged)
	s.config = next

	if err := config.Save(s.configPath, next); err != nil {
		s.pushMessage("error", fmt.Sprintf("save settings: %v", err))
		return s.snapshotLocked(), err
	}

	if s.pendingRestart {
		s.pushMessage("warning", "Settings saved. Restart the app to apply the new mode or listen address.")
	} else {
		s.pushMessage("info", "Settings saved.")
	}

	return s.snapshotLocked(), nil
}

func (s *LocalService) setFrequencyLocked(ctx context.Context, hz int64) error {
	if hz <= 0 {
		return fmt.Errorf("frequency must be positive")
	}

	s.radio.frequencyHz = hz
	if preset, ok := bands.ForFrequency(hz); ok {
		s.radio.bandID = preset.ID
	}
	s.radio.lastAction = "Frequency updated"

	if s.session != nil {
		if err := s.session.SetFrequency(ctx, hz); err != nil {
			return err
		}
		s.applySessionSnapshotLocked()
	}

	return nil
}

func (s *LocalService) setBandLocked(ctx context.Context, bandID string) error {
	preset, ok := bands.ByID(bandID)
	if !ok {
		return fmt.Errorf("unknown band preset")
	}

	s.radio.bandID = preset.ID
	s.radio.frequencyHz = preset.DefaultHz
	s.radio.lastAction = "Band preset selected"
	s.radio.status = fmt.Sprintf("%s selected.", preset.Label)

	if s.session != nil {
		if err := s.session.SetBand(ctx, preset.ID); err != nil {
			return err
		}
		if err := s.session.SetFrequency(ctx, preset.DefaultHz); err != nil {
			return err
		}
		s.applySessionSnapshotLocked()
	}

	return nil
}

func (s *LocalService) setModeLocked(ctx context.Context, modeID string) error {
	preset, ok := modes.ByID(modeID)
	if !ok {
		return fmt.Errorf("unknown operating mode")
	}

	s.radio.modeID = preset.ID
	s.radio.lastAction = "Operating mode updated"
	s.radio.status = fmt.Sprintf("%s mode selected.", preset.Label)

	if s.session != nil {
		if err := s.session.SetMode(ctx, preset.ID); err != nil {
			return err
		}
		s.applySessionSnapshotLocked()
	}

	return nil
}

func (s *LocalService) findDeviceLocked(id string) (radio.Device, bool) {
	for _, device := range s.devices {
		if device.ID == id {
			return device, true
		}
	}
	return radio.Device{}, false
}

func (s *LocalService) applySessionSnapshotLocked() {
	if s.session == nil {
		return
	}

	snapshot := s.session.Snapshot()
	s.radio.connected = snapshot.Connected
	s.radio.device = snapshot.Device
	s.radio.bandID = snapshot.BandID
	s.radio.modeID = snapshot.ModeID
	s.radio.frequencyHz = snapshot.FrequencyHz
	s.radio.stepHz = snapshot.StepHz
	s.radio.powerPercent = snapshot.PowerPercent
	s.radio.rxEnabled = snapshot.RXEnabled
	s.radio.txEnabled = snapshot.TXEnabled
	s.radio.ptt = snapshot.PTT
	s.radio.status = snapshot.Status
	s.radio.lastAction = snapshot.LastAction
	s.radio.capabilities = snapshot.Capabilities
}

func (s *LocalService) snapshotLocked() ViewState {
	devices := make([]radio.Device, len(s.devices))
	copy(devices, s.devices)

	messages := make([]Message, len(s.messages))
	copy(messages, s.messages)

	return ViewState{
		App: AppMeta{
			Name:            "SimpleHermes",
			Version:         s.version,
			ActiveMode:      string(s.activeMode),
			ProxyHealthy:    true,
			PendingRestart:  s.pendingRestart,
			Accessibility:   "Desktop webview shell with semantic HTML, keyboard-first controls, and live announcement regions.",
			VisualDirection: "Industrial station console with warm neutrals and signal-orange accents.",
		},
		Settings:    s.config.Public(),
		Devices:     devices,
		Bands:       bands.All(),
		Modes:       modes.All(),
		PowerLevels: powerLevels(),
		Shortcuts:   shortcuts(),
		Radio:       s.radio.asView(),
		Diagnostics: s.diagnosticsLocked(),
		Messages:    messages,
	}
}

func (s *LocalService) diagnosticsLocked() radio.Diagnostics {
	if s.session == nil {
		return radio.Diagnostics{
			Connected: false,
			Transport: "none",
		}
	}
	return s.session.Diagnostics()
}

func (s *LocalService) pushMessage(level, text string) {
	s.messages = append([]Message{{Level: level, Text: text}}, s.messages...)
	if len(s.messages) > 6 {
		s.messages = s.messages[:6]
	}
}
