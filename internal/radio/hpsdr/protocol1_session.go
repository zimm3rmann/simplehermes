package hpsdr

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"simplehermes/internal/radio"
)

const (
	protocol1DataPort      = 1024
	metisPacketSize        = 1032
	metisHeaderSize        = 8
	ozyFrameSize           = 512
	ozyFramePayloadSize    = ozyFrameSize - 8
	ozySyncByte            = 0x7F
	protocol1ReceiverCount = 1
	protocol1SampleGroup   = protocol1ReceiverCount*6 + 2
	samplesPerFrame        = ozyFramePayloadSize / protocol1SampleGroup
	samplesPerPacket       = samplesPerFrame * 2
	packetInterval         = time.Duration(int64(time.Second) * samplesPerPacket / audioSampleRate)
)

type protocol1Session struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	conn       *net.UDPConn
	remoteAddr *net.UDPAddr

	mu           sync.RWMutex
	snapshot     radio.Snapshot
	sendSequence uint32
	commandIndex int

	diagMu      sync.Mutex
	diagnostics radio.Diagnostics

	demod *demodulator
	mod   *modulator

	subMu       sync.Mutex
	subscribers map[chan []float32]struct{}
	firstPacket bool
}

func newProtocol1Session(parent context.Context, device radio.Device, options radio.SessionOptions) (radio.Session, error) {
	localIP, err := interfaceIPv4(device.InterfaceName)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: localIP, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("open radio socket: %w", err)
	}

	_ = conn.SetReadBuffer(1 << 20)
	_ = conn.SetWriteBuffer(1 << 20)

	ctx, cancel := context.WithCancel(parent)
	session := &protocol1Session{
		ctx:         ctx,
		cancel:      cancel,
		conn:        conn,
		remoteAddr:  &net.UDPAddr{IP: net.ParseIP(device.Address), Port: protocol1DataPort},
		demod:       newDemodulator(options.ModeID),
		mod:         newModulator(options.ModeID),
		subscribers: make(map[chan []float32]struct{}),
		snapshot: radio.Snapshot{
			Connected:    true,
			Device:       &device,
			BandID:       options.BandID,
			ModeID:       options.ModeID,
			FrequencyHz:  options.FrequencyHz,
			StepHz:       options.StepHz,
			PowerPercent: options.PowerPercent,
			RXEnabled:    options.RXEnabled,
			TXEnabled:    options.TXEnabled,
			PTT:          false,
			LastAction:   "Session opened",
			Status:       fmt.Sprintf("Connecting to %s at %s.", device.Model, device.Address),
			Capabilities: radio.Capabilities{
				DiscoveryReady: true,
				HardwareReady:  false,
				RXAudioReady:   false,
				TXAudioReady:   false,
				Summary:        "Opening Hermes protocol 1 transport and waiting for stream data.",
			},
		},
	}
	session.diagnostics = radio.Diagnostics{
		Connected:     true,
		Transport:     "hpsdr-protocol1-udp",
		LocalAddress:  conn.LocalAddr().String(),
		RemoteAddress: session.remoteAddr.String(),
		StartedAt:     time.Now().Format(time.RFC3339),
		LastControl:   "session-open",
	}

	session.wg.Add(1)
	go session.receiveLoop()

	if err := session.primeAndStart(); err != nil {
		session.cancel()
		_ = session.conn.Close()
		session.wg.Wait()
		return nil, err
	}

	session.wg.Add(1)
	go session.sendLoop()

	return session, nil
}

func (s *protocol1Session) Snapshot() radio.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := s.snapshot
	if out.Device != nil {
		device := *out.Device
		out.Device = &device
	}
	return out
}

func (s *protocol1Session) SetBand(_ context.Context, bandID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.BandID = bandID
	s.snapshot.LastAction = "Band preset selected"
	return nil
}

func (s *protocol1Session) SetMode(_ context.Context, modeID string) error {
	s.mu.Lock()
	s.snapshot.ModeID = modeID
	s.snapshot.LastAction = "Operating mode updated"
	s.mu.Unlock()

	s.demod.SetMode(modeID)
	s.mod.SetMode(modeID)
	return nil
}

func (s *protocol1Session) SetFrequency(_ context.Context, hz int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.FrequencyHz = hz
	s.snapshot.LastAction = "Frequency updated"
	return nil
}

func (s *protocol1Session) SetStep(_ context.Context, hz int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.StepHz = hz
	s.snapshot.LastAction = "Step size updated"
	return nil
}

func (s *protocol1Session) SetPower(_ context.Context, percent int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.PowerPercent = percent
	s.snapshot.LastAction = "Power level updated"
	return nil
}

func (s *protocol1Session) SetRXEnabled(_ context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.RXEnabled = enabled
	s.snapshot.LastAction = "Receive state changed"
	return nil
}

func (s *protocol1Session) SetTXEnabled(_ context.Context, enabled bool) error {
	s.mu.Lock()
	s.snapshot.TXEnabled = enabled
	if !enabled {
		s.snapshot.PTT = false
	}
	s.snapshot.LastAction = "Transmit armed state changed"
	s.mu.Unlock()

	if !enabled {
		s.mod.Reset()
	}
	return nil
}

func (s *protocol1Session) SetPTT(_ context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled && !s.snapshot.TXEnabled {
		return fmt.Errorf("enable transmit before asserting PTT")
	}
	s.snapshot.PTT = enabled
	s.snapshot.LastAction = "PTT state changed"
	if !enabled {
		s.mod.Reset()
	}
	return nil
}

func (s *protocol1Session) SubscribeRXAudio(ctx context.Context) (<-chan []float32, error) {
	s.mu.RLock()
	connected := s.snapshot.Connected
	s.mu.RUnlock()
	if !connected {
		return nil, fmt.Errorf("radio session is not connected")
	}

	ch := make(chan []float32, 32)

	s.subMu.Lock()
	s.subscribers[ch] = struct{}{}
	s.subMu.Unlock()

	go func() {
		<-ctx.Done()
		s.removeSubscriber(ch)
	}()

	return ch, nil
}

func (s *protocol1Session) WriteTXAudio(ctx context.Context, samples []float32) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.RLock()
	connected := s.snapshot.Connected
	s.mu.RUnlock()
	if !connected {
		return fmt.Errorf("radio session is not connected")
	}

	s.mod.Push(samples)
	s.recordTXAudio(len(samples))
	return nil
}

func (s *protocol1Session) Diagnostics() radio.Diagnostics {
	s.diagMu.Lock()
	out := s.diagnostics
	s.diagMu.Unlock()

	s.subMu.Lock()
	out.RXSubscribers = uint64(len(s.subscribers))
	s.subMu.Unlock()
	return out
}

func (s *protocol1Session) Close() error {
	s.cancel()
	_ = s.sendStartStop(0)
	_ = s.conn.Close()
	s.wg.Wait()

	s.closeSubscribers()

	s.mu.Lock()
	s.snapshot.Connected = false
	s.snapshot.PTT = false
	s.snapshot.LastAction = "Session closed"
	s.snapshot.Status = "Disconnected from Hermes session."
	s.snapshot.Capabilities.HardwareReady = false
	s.snapshot.Capabilities.RXAudioReady = false
	s.snapshot.Capabilities.TXAudioReady = false
	s.snapshot.Capabilities.Summary = "Hermes protocol 1 session is closed."
	s.mu.Unlock()

	s.recordConnected(false)
	return nil
}

func (s *protocol1Session) primeAndStart() error {
	for i := 0; i < 4; i++ {
		if err := s.sendPacket(); err != nil {
			return fmt.Errorf("prime transport: %w", err)
		}
	}

	if err := s.sendStartStop(1); err != nil {
		return fmt.Errorf("start stream: %w", err)
	}

	return nil
}

func (s *protocol1Session) sendLoop() {
	defer s.wg.Done()

	nextTick := time.Now()
	for {
		if err := s.ctx.Err(); err != nil {
			return
		}

		if err := s.sendPacket(); err != nil {
			if !errors.Is(err, net.ErrClosed) && s.ctx.Err() == nil {
				s.updateStatus(fmt.Sprintf("Radio send error: %v", err))
				s.recordError(err.Error())
			}
			return
		}

		nextTick = nextTick.Add(packetInterval)
		delay := time.Until(nextTick)
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		} else if delay < -packetInterval {
			nextTick = time.Now()
		}
	}
}

func (s *protocol1Session) receiveLoop() {
	defer s.wg.Done()

	buffer := make([]byte, 2048)
	for {
		if err := s.conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
			s.updateStatus(fmt.Sprintf("Radio read deadline failed: %v", err))
			return
		}

		n, _, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if s.ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}

			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}

			s.updateStatus(fmt.Sprintf("Radio receive error: %v", err))
			s.recordError(err.Error())
			return
		}

		if n < metisPacketSize {
			continue
		}

		if buffer[0] != 0xEF || buffer[1] != 0xFE || buffer[2] != 0x01 || buffer[3] != 0x06 {
			continue
		}
		s.recordRXPacket()

		packet := make([]float32, 0, samplesPerPacket)
		packet = s.processEP6Frame(buffer[8:520], packet)
		packet = s.processEP6Frame(buffer[520:1032], packet)

		if len(packet) > 0 {
			s.publishRXAudio(packet)
		}

		s.markFirstPacket()
	}
}

func (s *protocol1Session) processEP6Frame(frame []byte, packet []float32) []float32 {
	if len(frame) != ozyFrameSize {
		return packet
	}
	if frame[0] != ozySyncByte || frame[1] != ozySyncByte || frame[2] != ozySyncByte {
		return packet
	}
	s.recordRXFrame()

	s.mu.RLock()
	rxEnabled := s.snapshot.RXEnabled
	s.mu.RUnlock()
	if !rxEnabled {
		return packet
	}

	offset := 8
	for offset+protocol1SampleGroup <= len(frame) {
		iSample := decodeInt24(frame[offset], frame[offset+1], frame[offset+2])
		qSample := decodeInt24(frame[offset+3], frame[offset+4], frame[offset+5])
		offset += protocol1SampleGroup

		audio := s.demod.ProcessIQ(float64(iSample)/8388607.0, float64(qSample)/8388607.0)
		packet = append(packet, audio)
	}

	return packet
}

func (s *protocol1Session) sendPacket() error {
	packet := s.buildPacket()
	_, err := s.conn.WriteToUDP(packet, s.remoteAddr)
	if err == nil {
		s.recordSendPacket()
	}
	return err
}

func (s *protocol1Session) buildPacket() []byte {
	packet := make([]byte, metisPacketSize)
	packet[0] = 0xEF
	packet[1] = 0xFE
	packet[2] = 0x01
	packet[3] = 0x02

	s.mu.Lock()
	state := s.snapshot
	command := s.nextCommandLocked()
	sequence := s.sendSequence
	s.sendSequence++
	s.mu.Unlock()

	binary.BigEndian.PutUint32(packet[4:8], sequence)
	s.fillFrame(packet[8:520], 0, state)
	s.fillFrame(packet[520:1032], command, state)
	return packet
}

func (s *protocol1Session) fillFrame(frame []byte, command int, state radio.Snapshot) {
	for i := range frame {
		frame[i] = 0
	}

	frame[0] = ozySyncByte
	frame[1] = ozySyncByte
	frame[2] = ozySyncByte

	control := s.controlBytes(command, state)
	copy(frame[3:8], control[:])

	if state.PTT && state.TXEnabled {
		s.fillTXPayload(frame[8:])
	}
}

func (s *protocol1Session) fillTXPayload(payload []byte) {
	written := 0
	offset := 0
	for sample := 0; sample < samplesPerFrame && offset+8 <= len(payload); sample++ {
		iSample, qSample := s.mod.NextIQ()
		payload[offset+4] = byte(iSample >> 8)
		payload[offset+5] = byte(iSample)
		payload[offset+6] = byte(qSample >> 8)
		payload[offset+7] = byte(qSample)
		offset += 8
		written++
	}
	s.recordTXIQSamples(written)
}

func (s *protocol1Session) controlBytes(command int, state radio.Snapshot) [5]byte {
	var control [5]byte
	pttBit := byte(0)
	if state.PTT && state.TXEnabled {
		pttBit = 0x01
	}

	switch command {
	case 0:
		control[0] = pttBit
		control[1] = 0x00 // 48 kHz sample rate, single receiver
		control[4] = byte((protocol1ReceiverCount - 1) << 3)
		s.recordControl("base-config", 0)
	case 1:
		control[0] = 0x02 | pttBit
		putFrequency(control[1:], state.FrequencyHz)
		s.recordControl("tx-frequency", state.FrequencyHz)
	case 2:
		control[0] = 0x04 | pttBit
		putFrequency(control[1:], state.FrequencyHz)
		s.recordControl("rx1-frequency", state.FrequencyHz)
	case 3:
		control[0] = 0x12 | pttBit
		control[1] = byte(clampDrive(state.PowerPercent))
		if state.TXEnabled {
			control[2] |= 0x08
		}
		if state.PTT {
			control[2] |= 0x10
		}
		s.recordControl("drive-and-hl2-pa", 0)
	case 4:
		control[0] = 0x14 | pttBit
		control[4] = 0x40 | 26 // HL2 gain encoding, 26 ~= nominal zero attenuation.
		s.recordControl("hl2-rx-gain", 0)
	case 5:
		control[0] = 0x1C | pttBit
		s.recordControl("adc-selection", 0)
	default:
		control[0] = pttBit
		s.recordControl("unknown", 0)
	}

	return control
}

func (s *protocol1Session) nextCommandLocked() int {
	command := s.commandIndex
	s.commandIndex = (s.commandIndex + 1) % 6
	return command
}

func (s *protocol1Session) sendStartStop(command byte) error {
	buffer := make([]byte, 64)
	buffer[0] = 0xEF
	buffer[1] = 0xFE
	buffer[2] = 0x04
	buffer[3] = command
	_, err := s.conn.WriteToUDP(buffer, s.remoteAddr)
	if err == nil {
		s.recordStartStop(command)
	}
	return err
}

func (s *protocol1Session) publishRXAudio(samples []float32) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	for subscriber := range s.subscribers {
		select {
		case subscriber <- samples:
		default:
			s.recordRXAudioDrop()
		}
	}
	s.recordRXAudio(len(samples))
}

func (s *protocol1Session) removeSubscriber(ch chan []float32) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if _, ok := s.subscribers[ch]; !ok {
		return
	}

	delete(s.subscribers, ch)
	close(ch)
}

func (s *protocol1Session) closeSubscribers() {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	for subscriber := range s.subscribers {
		close(subscriber)
	}
	s.subscribers = map[chan []float32]struct{}{}
}

func (s *protocol1Session) markFirstPacket() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.firstPacket {
		return
	}

	s.firstPacket = true
	s.snapshot.Status = fmt.Sprintf("Streaming from %s at %s.", s.snapshot.Device.Model, s.snapshot.Device.Address)
	s.snapshot.Capabilities.HardwareReady = true
	s.snapshot.Capabilities.RXAudioReady = true
	s.snapshot.Capabilities.TXAudioReady = true
	s.snapshot.Capabilities.Summary = "Hermes protocol 1 transport is live. RX audio playback and TX audio capture are active."
	s.snapshot.LastAction = "Stream started"
}

func (s *protocol1Session) updateStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.Status = status
}

func (s *protocol1Session) recordConnected(connected bool) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.Connected = connected
}

func (s *protocol1Session) recordSendPacket() {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.SendPackets++
}

func (s *protocol1Session) recordStartStop(command byte) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	if command == 0 {
		s.diagnostics.StopCommands++
		s.diagnostics.LastControl = "stop-stream"
		return
	}
	s.diagnostics.StartCommands++
	s.diagnostics.LastControl = "start-stream"
}

func (s *protocol1Session) recordControl(name string, frequencyHz int64) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.ControlFrames++
	s.diagnostics.LastControl = name
	switch name {
	case "tx-frequency":
		s.diagnostics.FrequencyFrames++
		s.diagnostics.LastTXFrequency = frequencyHz
	case "rx1-frequency":
		s.diagnostics.FrequencyFrames++
		s.diagnostics.LastRXFrequency = frequencyHz
	}
}

func (s *protocol1Session) recordRXPacket() {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.RXPackets++
}

func (s *protocol1Session) recordRXFrame() {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.RXFrames++
}

func (s *protocol1Session) recordRXAudio(samples int) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.RXAudioFrames++
	s.diagnostics.RXAudioSamples += uint64(samples)
}

func (s *protocol1Session) recordRXAudioDrop() {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.RXAudioDrops++
}

func (s *protocol1Session) recordTXAudio(samples int) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.TXAudioFrames++
	s.diagnostics.TXAudioSamples += uint64(samples)
}

func (s *protocol1Session) recordTXIQSamples(samples int) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.TXIQSamples += uint64(samples)
}

func (s *protocol1Session) recordError(message string) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diagnostics.LastError = message
}

func decodeInt24(b0, b1, b2 byte) int32 {
	return int32(int8(b0))<<16 | int32(b1)<<8 | int32(b2)
}

func putFrequency(target []byte, hz int64) {
	value := uint32(hz)
	target[0] = byte(value >> 24)
	target[1] = byte(value >> 16)
	target[2] = byte(value >> 8)
	target[3] = byte(value)
}

func clampDrive(percent int) int {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return int(float64(percent) * 255.0 / 100.0)
}

func interfaceIPv4(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("lookup interface %q: %w", name, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("list interface addresses for %q: %w", name, err)
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 != nil {
			return ip4, nil
		}
	}

	return nil, fmt.Errorf("interface %q does not have an IPv4 address", name)
}
