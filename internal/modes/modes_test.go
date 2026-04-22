package modes

import "testing"

func TestNext(t *testing.T) {
	tests := []struct {
		current string
		want    string
	}{
		{current: "lsb", want: "usb"},
		{current: "fm", want: "digu"},
		{current: "digu", want: "lsb"},
		{current: "unknown", want: "lsb"},
	}

	for _, tt := range tests {
		if got := Next(tt.current).ID; got != tt.want {
			t.Fatalf("Next(%q) = %q, want %q", tt.current, got, tt.want)
		}
	}
}
