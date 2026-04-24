package hpsdr

import (
	"math"
	"testing"
)

func TestDemodulatorSeparatesUpperAndLowerSidebands(t *testing.T) {
	usbUpper := demodToneRMS("usb", 1000, false)
	usbLower := demodToneRMS("usb", -1000, false)
	if usbUpper < 0.08 {
		t.Fatalf("USB upper sideband RMS too low: %.4f", usbUpper)
	}
	if usbUpper < usbLower*3 {
		t.Fatalf("USB sideband rejection too weak: upper %.4f lower %.4f", usbUpper, usbLower)
	}

	lsbLower := demodToneRMS("lsb", -1000, false)
	lsbUpper := demodToneRMS("lsb", 1000, false)
	if lsbLower < 0.08 {
		t.Fatalf("LSB lower sideband RMS too low: %.4f", lsbLower)
	}
	if lsbLower < lsbUpper*3 {
		t.Fatalf("LSB sideband rejection too weak: lower %.4f upper %.4f", lsbLower, lsbUpper)
	}
}

func TestDemodulatorProducesAMAudio(t *testing.T) {
	rms := demodAMRMS()
	if rms < 0.08 {
		t.Fatalf("AM demod RMS too low: %.4f", rms)
	}
}

func TestDemodulatorProducesFMAudio(t *testing.T) {
	rms := demodFMRMS()
	if rms < 0.08 {
		t.Fatalf("FM demod RMS too low: %.4f", rms)
	}
}

func TestDemodulatorOutputIsBounded(t *testing.T) {
	d := newDemodulator("usb")
	for i := 0; i < 2000; i++ {
		out := d.ProcessIQ(100, 100)
		if out > 0.98 || out < -0.98 {
			t.Fatalf("output sample %d out of bounds: %.4f", i, out)
		}
	}
}

func demodToneRMS(modeID string, frequencyHz float64, invertQ bool) float64 {
	d := newDemodulator(modeID)
	const total = 7000
	const skip = 1800

	var sum float64
	var count int
	for n := 0; n < total; n++ {
		phase := 2 * math.Pi * frequencyHz * float64(n) / audioSampleRate
		i := math.Cos(phase) * 0.35
		q := math.Sin(phase) * 0.35
		if invertQ {
			q = -q
		}
		out := float64(d.ProcessIQ(i, q))
		if n >= skip {
			sum += out * out
			count++
		}
	}
	return math.Sqrt(sum / float64(count))
}

func demodAMRMS() float64 {
	d := newDemodulator("am")
	const total = 7000
	const skip = 1200

	var sum float64
	var count int
	for n := 0; n < total; n++ {
		audio := math.Sin(2 * math.Pi * 900 * float64(n) / audioSampleRate)
		envelope := 0.55 + 0.18*audio
		out := float64(d.ProcessIQ(envelope, 0))
		if n >= skip {
			sum += out * out
			count++
		}
	}
	return math.Sqrt(sum / float64(count))
}

func demodFMRMS() float64 {
	d := newDemodulator("fm")
	const total = 7000
	const skip = 1200

	var phase float64
	var sum float64
	var count int
	for n := 0; n < total; n++ {
		audio := math.Sin(2 * math.Pi * 900 * float64(n) / audioSampleRate)
		phase += 0.08 * audio
		out := float64(d.ProcessIQ(math.Cos(phase), math.Sin(phase)))
		if n >= skip {
			sum += out * out
			count++
		}
	}
	return math.Sqrt(sum / float64(count))
}
