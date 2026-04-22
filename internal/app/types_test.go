package app

import "testing"

func TestParseMHz(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantHz  int64
		wantErr bool
	}{
		{name: "valid", input: "14.200000", wantHz: 14_200_000},
		{name: "trimmed", input: " 7.055 ", wantHz: 7_055_000},
		{name: "zero", input: "0", wantErr: true},
		{name: "bad", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMHz(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantHz {
				t.Fatalf("got %d, want %d", got, tt.wantHz)
			}
		})
	}
}

func TestClampPower(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{input: -4, want: 0},
		{input: 18, want: 18},
		{input: 150, want: 100},
	}

	for _, tt := range tests {
		if got := clampPower(tt.input); got != tt.want {
			t.Fatalf("clampPower(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNextPowerLevel(t *testing.T) {
	tests := []struct {
		current int
		want    int
	}{
		{current: 0, want: 10},
		{current: 10, want: 20},
		{current: 40, want: 50},
		{current: 95, want: 100},
		{current: 100, want: 10},
	}

	for _, tt := range tests {
		if got := nextPowerLevel(tt.current).Percent; got != tt.want {
			t.Fatalf("nextPowerLevel(%d) = %d, want %d", tt.current, got, tt.want)
		}
	}
}

func TestPowerLabelForPercent(t *testing.T) {
	if got := powerLabelForPercent(30); got != "30 percent drive" {
		t.Fatalf("got %q", got)
	}
	if got := powerLabelForPercent(33); got != "33 percent drive" {
		t.Fatalf("got %q", got)
	}
}
