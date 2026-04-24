package hpsdr

import (
	"context"
	"testing"

	"simplehermes/internal/radio"
)

func TestProtocol1WriteTXAudioRequiresActivePTT(t *testing.T) {
	session := &protocol1Session{
		mod: newModulator("usb"),
		snapshot: radio.Snapshot{
			Connected: true,
			TXEnabled: true,
			PTT:       false,
		},
	}

	if err := session.WriteTXAudio(context.Background(), []float32{0.5}); err != nil {
		t.Fatalf("inactive PTT WriteTXAudio returned error: %v", err)
	}
	if got := len(session.mod.frames); got != 0 {
		t.Fatalf("inactive PTT queued %d audio frames, want 0", got)
	}
	if got := session.Diagnostics().TXAudioFrames; got != 0 {
		t.Fatalf("inactive PTT recorded %d TX audio frames, want 0", got)
	}

	session.mu.Lock()
	session.snapshot.PTT = true
	session.mu.Unlock()

	if err := session.WriteTXAudio(context.Background(), []float32{0.5}); err != nil {
		t.Fatalf("active PTT WriteTXAudio returned error: %v", err)
	}
	if got := len(session.mod.frames); got != 1 {
		t.Fatalf("active PTT queued %d audio frames, want 1", got)
	}
	if got := session.Diagnostics().TXAudioFrames; got != 1 {
		t.Fatalf("active PTT recorded %d TX audio frames, want 1", got)
	}
}

func TestProtocol1WriteTXAudioRejectsDisconnectedSession(t *testing.T) {
	session := &protocol1Session{
		mod:      newModulator("usb"),
		snapshot: radio.Snapshot{Connected: false},
	}

	if err := session.WriteTXAudio(context.Background(), []float32{0.5}); err == nil {
		t.Fatalf("expected disconnected WriteTXAudio to fail")
	}
}

func TestProtocol1ProcessEP6FrameDecodesReceiverIQAndSkipsMicBytes(t *testing.T) {
	session := &protocol1Session{
		demod: newDemodulator("am"),
		snapshot: radio.Snapshot{
			RXEnabled: true,
		},
	}

	frame := ozyTestFrame()
	for offset := 8; offset+protocol1SampleGroup <= len(frame); offset += protocol1SampleGroup {
		frame[offset+6] = 0x7f
		frame[offset+7] = 0xff
	}

	packet := session.processEP6Frame(frame, nil)
	if len(packet) != samplesPerFrame {
		t.Fatalf("decoded %d samples, want %d", len(packet), samplesPerFrame)
	}
	for index, sample := range packet {
		if sample != 0 {
			t.Fatalf("sample %d = %f, want silence when only mic bytes are nonzero", index, sample)
		}
	}
}

func TestProtocol1ProcessEP6FramePublishesReceiverIQ(t *testing.T) {
	session := &protocol1Session{
		demod: newDemodulator("am"),
		snapshot: radio.Snapshot{
			RXEnabled: true,
		},
	}

	frame := ozyTestFrame()
	frame[8] = 0x7f
	frame[9] = 0xff
	frame[10] = 0xff

	packet := session.processEP6Frame(frame, nil)
	if len(packet) != samplesPerFrame {
		t.Fatalf("decoded %d samples, want %d", len(packet), samplesPerFrame)
	}
	if !hasNonZeroAudio(packet) {
		t.Fatalf("decoded IQ frame was silent")
	}
}

func TestProtocol1ProcessEP6FrameHonorsRXDisabled(t *testing.T) {
	session := &protocol1Session{
		demod: newDemodulator("am"),
		snapshot: radio.Snapshot{
			RXEnabled: false,
		},
	}

	frame := ozyTestFrame()
	frame[8] = 0x7f
	frame[9] = 0xff
	frame[10] = 0xff

	packet := session.processEP6Frame(frame, nil)
	if len(packet) != 0 {
		t.Fatalf("decoded %d samples with RX disabled, want 0", len(packet))
	}
}

func ozyTestFrame() []byte {
	frame := make([]byte, ozyFrameSize)
	frame[0] = ozySyncByte
	frame[1] = ozySyncByte
	frame[2] = ozySyncByte
	return frame
}

func hasNonZeroAudio(samples []float32) bool {
	for _, sample := range samples {
		if sample != 0 {
			return true
		}
	}
	return false
}
