package hpsdr

import (
	"net"
	"testing"
)

func TestParseProtocol1HermesLite2(t *testing.T) {
	payload := make([]byte, 64)
	payload[0] = 0xEF
	payload[1] = 0xFE
	payload[2] = 0x02
	payload[9] = 42
	payload[10] = 6
	payload[0x13] = 4

	device, ok := parseProtocol1(payload, &net.UDPAddr{IP: net.ParseIP("192.168.10.55")}, "eth0")
	if !ok {
		t.Fatalf("expected payload to parse")
	}

	if device.Model != "Hermes Lite 2" {
		t.Fatalf("got model %q", device.Model)
	}
	if device.SupportedReceivers != 4 {
		t.Fatalf("got supported receivers %d", device.SupportedReceivers)
	}
	if device.Protocol != "protocol1" {
		t.Fatalf("got protocol %q", device.Protocol)
	}
}

func TestParseProtocol2HermesLite(t *testing.T) {
	payload := make([]byte, 64)
	payload[4] = 0x02
	payload[11] = 6
	payload[13] = 88
	payload[20] = 3

	device, ok := parseProtocol2(payload, &net.UDPAddr{IP: net.ParseIP("10.1.1.44")}, "en0")
	if !ok {
		t.Fatalf("expected payload to parse")
	}

	if device.Model != "Hermes Lite" {
		t.Fatalf("got model %q", device.Model)
	}
	if device.SupportedReceivers != 3 {
		t.Fatalf("got supported receivers %d", device.SupportedReceivers)
	}
	if device.Protocol != "protocol2" {
		t.Fatalf("got protocol %q", device.Protocol)
	}
}
