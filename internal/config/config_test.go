package config

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.json")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := Default()
	if got != want {
		t.Fatalf("Load missing = %#v, want %#v", got, want)
	}
}

func TestSaveAndLoadNormalizeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	input := Config{
		Mode:                Mode("invalid"),
		ListenAddress:       "  ",
		RemoteBaseURL:       " http://example.test:8787/api/ ",
		RemoteAuthToken:     " secret ",
		AccessibilityMode:   false,
		AudioInputDeviceID:  " mic-device ",
		AudioOutputDeviceID: " speaker-device ",
	}

	if err := Save(path, input); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got.Mode != ModeLocal {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeLocal)
	}
	if got.ListenAddress != Default().ListenAddress {
		t.Fatalf("ListenAddress = %q, want %q", got.ListenAddress, Default().ListenAddress)
	}
	if got.RemoteBaseURL != "http://example.test:8787/api" {
		t.Fatalf("RemoteBaseURL = %q", got.RemoteBaseURL)
	}
	if got.RemoteAuthToken != "secret" {
		t.Fatalf("RemoteAuthToken = %q", got.RemoteAuthToken)
	}
	if !got.Public().RemoteAuthConfigured {
		t.Fatalf("RemoteAuthConfigured should be true")
	}
	if got.AccessibilityMode {
		t.Fatalf("AccessibilityMode should remain false")
	}
	if got.AudioInputDeviceID != "mic-device" {
		t.Fatalf("AudioInputDeviceID = %q", got.AudioInputDeviceID)
	}
	if got.AudioOutputDeviceID != "speaker-device" {
		t.Fatalf("AudioOutputDeviceID = %q", got.AudioOutputDeviceID)
	}
}
