package hpsdr

import (
	"math"
	"sync"
	"sync/atomic"
)

const (
	audioSampleRate = 48_000
	cwToneHz        = 700.0
)

const (
	voiceLowCutHz    = 120.0
	voiceHighCutHz   = 3200.0
	digitalLowCutHz  = 40.0
	digitalHighCutHz = 3600.0
	cwLowCutHz       = 300.0
	cwHighCutHz      = 1200.0
	amLowCutHz       = 40.0
	amHighCutHz      = 5000.0
	fmLowCutHz       = 80.0
	fmHighCutHz      = 5000.0
)

const (
	modeUSB uint32 = iota
	modeLSB
	modeCW
	modeAM
	modeFM
	modeDIGU
)

func modeCode(id string) uint32 {
	switch id {
	case "lsb":
		return modeLSB
	case "cw":
		return modeCW
	case "am":
		return modeAM
	case "fm":
		return modeFM
	case "digu":
		return modeDIGU
	default:
		return modeUSB
	}
}

type demodulator struct {
	mode     atomic.Uint32
	lastMode uint32

	ssbIDelay   *hilbertFilter
	ssbQHilbert *hilbertFilter

	voiceHP   *biquad
	voiceLP   *biquad
	digitalHP *biquad
	digitalLP *biquad
	cwHP      *biquad
	cwLP      *biquad
	amHP      *biquad
	amLP      *biquad
	fmHP      *biquad
	fmLP      *biquad

	amAverage float64
	agc       *audioAGC
	cwPhase   float64
	fmPrevI   float64
	fmPrevQ   float64
	fmHaveRef bool
}

func newDemodulator(modeID string) *demodulator {
	code := modeCode(modeID)
	d := &demodulator{
		lastMode: code,

		ssbIDelay:   newHilbertFilter(63),
		ssbQHilbert: newHilbertFilter(63),

		voiceHP:   newHighpass(voiceLowCutHz),
		voiceLP:   newLowpass(voiceHighCutHz),
		digitalHP: newHighpass(digitalLowCutHz),
		digitalLP: newLowpass(digitalHighCutHz),
		cwHP:      newHighpass(cwLowCutHz),
		cwLP:      newLowpass(cwHighCutHz),
		amHP:      newHighpass(amLowCutHz),
		amLP:      newLowpass(amHighCutHz),
		fmHP:      newHighpass(fmLowCutHz),
		fmLP:      newLowpass(fmHighCutHz),

		agc: newAudioAGC(),
	}
	d.mode.Store(code)
	return d
}

func (d *demodulator) SetMode(modeID string) {
	d.mode.Store(modeCode(modeID))
}

func (d *demodulator) ProcessIQ(i, q float64) float32 {
	mode := d.mode.Load()
	if mode != d.lastMode {
		d.resetForMode(mode)
	}

	var raw float64

	switch mode {
	case modeAM:
		raw = d.processAM(i, q)
	case modeFM:
		raw = d.processFM(i, q)
	case modeCW:
		raw = d.processCW(i, q)
	case modeLSB:
		raw = d.processSSB(i, q, true)
	case modeDIGU:
		raw = d.processDigital(i, q)
	default:
		raw = d.processSSB(i, q, false)
	}

	return float32(d.agc.Process(raw))
}

func (d *demodulator) resetForMode(mode uint32) {
	d.lastMode = mode
	d.amAverage = 0
	d.cwPhase = 0
	d.fmPrevI = 0
	d.fmPrevQ = 0
	d.fmHaveRef = false

	d.ssbIDelay.Reset()
	d.ssbQHilbert.Reset()
	d.voiceHP.Reset()
	d.voiceLP.Reset()
	d.digitalHP.Reset()
	d.digitalLP.Reset()
	d.cwHP.Reset()
	d.cwLP.Reset()
	d.amHP.Reset()
	d.amLP.Reset()
	d.fmHP.Reset()
	d.fmLP.Reset()
	d.agc.Reset()
}

func (d *demodulator) processSSB(i, q float64, lower bool) float64 {
	iDelayed, _ := d.ssbIDelay.Process(i)
	_, qHilbert := d.ssbQHilbert.Process(q)

	audio := iDelayed - qHilbert
	if lower {
		audio = iDelayed + qHilbert
	}
	audio *= 0.5

	audio = d.voiceHP.Process(audio)
	audio = d.voiceLP.Process(audio)
	return audio
}

func (d *demodulator) processDigital(i, q float64) float64 {
	iDelayed, _ := d.ssbIDelay.Process(i)
	_, qHilbert := d.ssbQHilbert.Process(q)
	audio := (iDelayed - qHilbert) * 0.5
	audio = d.digitalHP.Process(audio)
	audio = d.digitalLP.Process(audio)
	return audio
}

func (d *demodulator) processCW(i, q float64) float64 {
	phaseStep := 2 * math.Pi * cwToneHz / audioSampleRate
	cosine := math.Cos(d.cwPhase)
	sine := math.Sin(d.cwPhase)
	audio := i*cosine + q*sine
	d.cwPhase += phaseStep
	if d.cwPhase >= 2*math.Pi {
		d.cwPhase -= 2 * math.Pi
	}

	audio = d.cwHP.Process(audio)
	audio = d.cwLP.Process(audio)
	return audio
}

func (d *demodulator) processAM(i, q float64) float64 {
	envelope := math.Hypot(i, q)
	if d.amAverage == 0 {
		d.amAverage = envelope
	}
	d.amAverage = 0.9995*d.amAverage + 0.0005*envelope

	audio := envelope - d.amAverage
	audio = d.amHP.Process(audio)
	audio = d.amLP.Process(audio)
	return audio
}

func (d *demodulator) processFM(i, q float64) float64 {
	if !d.fmHaveRef {
		d.fmPrevI = i
		d.fmPrevQ = q
		d.fmHaveRef = true
		return 0
	}

	audio := math.Atan2(i*d.fmPrevQ-q*d.fmPrevI, i*d.fmPrevI+q*d.fmPrevQ) * 6.0
	d.fmPrevI = i
	d.fmPrevQ = q
	audio = d.fmHP.Process(audio)
	audio = d.fmLP.Process(audio)
	return audio
}

type biquad struct {
	b0 float64
	b1 float64
	b2 float64
	a1 float64
	a2 float64
	z1 float64
	z2 float64
}

func newLowpass(cutoffHz float64) *biquad {
	return newBiquad(cutoffHz, 1.0/math.Sqrt2, "lowpass")
}

func newHighpass(cutoffHz float64) *biquad {
	return newBiquad(cutoffHz, 1.0/math.Sqrt2, "highpass")
}

func newBiquad(cutoffHz, q float64, kind string) *biquad {
	if cutoffHz < 1 {
		cutoffHz = 1
	}
	nyquist := float64(audioSampleRate) / 2
	if cutoffHz > nyquist-1 {
		cutoffHz = nyquist - 1
	}

	omega := 2 * math.Pi * cutoffHz / float64(audioSampleRate)
	sine := math.Sin(omega)
	cosine := math.Cos(omega)
	alpha := sine / (2 * q)

	var b0, b1, b2 float64
	switch kind {
	case "highpass":
		b0 = (1 + cosine) / 2
		b1 = -(1 + cosine)
		b2 = (1 + cosine) / 2
	default:
		b0 = (1 - cosine) / 2
		b1 = 1 - cosine
		b2 = (1 - cosine) / 2
	}

	a0 := 1 + alpha
	a1 := -2 * cosine
	a2 := 1 - alpha

	return &biquad{
		b0: b0 / a0,
		b1: b1 / a0,
		b2: b2 / a0,
		a1: a1 / a0,
		a2: a2 / a0,
	}
}

func (b *biquad) Process(sample float64) float64 {
	out := b.b0*sample + b.z1
	b.z1 = b.b1*sample - b.a1*out + b.z2
	b.z2 = b.b2*sample - b.a2*out
	return out
}

func (b *biquad) Reset() {
	b.z1 = 0
	b.z2 = 0
}

type audioAGC struct {
	envelope float64
	gain     float64
}

func newAudioAGC() *audioAGC {
	return &audioAGC{
		envelope: 0.05,
		gain:     4.0,
	}
}

func (a *audioAGC) Reset() {
	a.envelope = 0.05
	a.gain = 4.0
}

func (a *audioAGC) Process(sample float64) float64 {
	level := math.Abs(sample)

	attack := 0.995
	release := 0.99995
	coeff := release
	if level > a.envelope {
		coeff = attack
	}
	a.envelope = coeff*a.envelope + (1-coeff)*level

	desiredGain := 0.35 / math.Max(a.envelope, 0.01)
	if desiredGain > 28 {
		desiredGain = 28
	} else if desiredGain < 0.2 {
		desiredGain = 0.2
	}

	gainCoeff := 0.9995
	if desiredGain < a.gain {
		gainCoeff = 0.995
	}
	a.gain = gainCoeff*a.gain + (1-gainCoeff)*desiredGain

	out := sample * a.gain
	if out > 0.98 {
		return 0.98
	}
	if out < -0.98 {
		return -0.98
	}
	return out
}

type modulator struct {
	mode atomic.Uint32

	hilbert *hilbertFilter
	frames  chan []float32

	mu      sync.Mutex
	pending []float32
	index   int
	phase   float64
}

func newModulator(modeID string) *modulator {
	m := &modulator{
		hilbert: newHilbertFilter(31),
		frames:  make(chan []float32, 128),
	}
	m.mode.Store(modeCode(modeID))
	return m
}

func (m *modulator) SetMode(modeID string) {
	m.mode.Store(modeCode(modeID))
}

func (m *modulator) Reset() {
	m.mu.Lock()
	m.pending = nil
	m.index = 0
	m.phase = 0
	m.mu.Unlock()
	m.hilbert.Reset()

	for {
		select {
		case <-m.frames:
		default:
			return
		}
	}
}

func (m *modulator) Push(samples []float32) {
	if len(samples) == 0 {
		return
	}

	frame := make([]float32, len(samples))
	copy(frame, samples)

	select {
	case m.frames <- frame:
	default:
		select {
		case <-m.frames:
		default:
		}
		select {
		case m.frames <- frame:
		default:
		}
	}
}

func (m *modulator) NextIQ() (int16, int16) {
	sample := m.nextSample()

	switch m.mode.Load() {
	case modeCW:
		if math.Abs(sample) < 0.005 {
			sample = 0.6
		}
		m.phase += 2 * math.Pi * cwToneHz / audioSampleRate
		if m.phase >= 2*math.Pi {
			m.phase -= 2 * math.Pi
		}
		i := sample * math.Cos(m.phase) * 0.65
		q := sample * math.Sin(m.phase) * 0.65
		return floatToPCM16(i), floatToPCM16(q)
	case modeAM:
		carrier := 0.28
		i := carrier + sample*0.25
		return floatToPCM16(i), 0
	case modeFM:
		m.phase += sample * 0.08
		i := math.Cos(m.phase) * 0.45
		q := math.Sin(m.phase) * 0.45
		return floatToPCM16(i), floatToPCM16(q)
	case modeLSB:
		delayed, imag := m.hilbert.Process(float64(sample))
		return floatToPCM16(delayed * 0.75), floatToPCM16(-imag * 0.75)
	default:
		delayed, imag := m.hilbert.Process(float64(sample))
		return floatToPCM16(delayed * 0.75), floatToPCM16(imag * 0.75)
	}
}

func (m *modulator) nextSample() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	for len(m.pending) == 0 || m.index >= len(m.pending) {
		select {
		case frame := <-m.frames:
			m.pending = frame
			m.index = 0
		default:
			return 0
		}
	}

	value := float64(m.pending[m.index])
	m.index++
	if m.index >= len(m.pending) {
		m.pending = nil
		m.index = 0
	}

	if value > 1 {
		value = 1
	} else if value < -1 {
		value = -1
	}
	return value
}

type hilbertFilter struct {
	coeffs []float64
	buffer []float64
	index  int
}

func newHilbertFilter(taps int) *hilbertFilter {
	if taps < 7 {
		taps = 7
	}
	if taps%2 == 0 {
		taps++
	}

	coeffs := make([]float64, taps)
	half := taps / 2

	for n := 0; n < taps; n++ {
		k := n - half
		if k == 0 || k%2 == 0 {
			coeffs[n] = 0
			continue
		}

		value := 2.0 / (math.Pi * float64(k))
		window := 0.54 - 0.46*math.Cos(2*math.Pi*float64(n)/float64(taps-1))
		coeffs[n] = value * window
	}

	return &hilbertFilter{
		coeffs: coeffs,
		buffer: make([]float64, taps),
	}
}

func (h *hilbertFilter) Reset() {
	for i := range h.buffer {
		h.buffer[i] = 0
	}
	h.index = 0
}

func (h *hilbertFilter) Process(sample float64) (float64, float64) {
	h.buffer[h.index] = sample

	var imag float64
	index := h.index
	for _, coeff := range h.coeffs {
		imag += coeff * h.buffer[index]
		index--
		if index < 0 {
			index = len(h.buffer) - 1
		}
	}

	delayIndex := h.index - len(h.buffer)/2
	if delayIndex < 0 {
		delayIndex += len(h.buffer)
	}
	delayed := h.buffer[delayIndex]

	h.index++
	if h.index >= len(h.buffer) {
		h.index = 0
	}

	return delayed, imag
}

func floatToPCM16(value float64) int16 {
	if value > 0.98 {
		value = 0.98
	} else if value < -0.98 {
		value = -0.98
	}
	return int16(value * 32767.0)
}
