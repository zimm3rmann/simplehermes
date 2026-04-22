package app

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

var audioUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool { return true },
}

func (s *LocalService) HandleRXAudio(w http.ResponseWriter, r *http.Request) {
	session := s.currentSession()
	if session == nil {
		http.Error(w, "radio session is not connected", http.StatusConflict)
		return
	}

	conn, err := audioUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stream, err := session.SubscribeRXAudio(ctx)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-stream:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, floatsToBytes(frame)); err != nil {
				return
			}
		}
	}
}

func (s *LocalService) HandleTXAudio(w http.ResponseWriter, r *http.Request) {
	session := s.currentSession()
	if session == nil {
		http.Error(w, "radio session is not connected", http.StatusConflict)
		return
	}

	conn, err := audioUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage {
			continue
		}

		samples, err := bytesToFloats(payload)
		if err != nil {
			continue
		}
		if err := session.WriteTXAudio(r.Context(), samples); err != nil {
			return
		}
	}
}

func (s *RemoteService) HandleRXAudio(w http.ResponseWriter, r *http.Request) {
	s.proxyAudio(w, r, "/api/audio/rx", false)
}

func (s *RemoteService) HandleTXAudio(w http.ResponseWriter, r *http.Request) {
	s.proxyAudio(w, r, "/api/audio/tx", true)
}

func (s *RemoteService) proxyAudio(w http.ResponseWriter, r *http.Request, path string, localToRemote bool) {
	localConn, err := audioUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer localConn.Close()

	remoteURL, err := websocketURL(s.config.RemoteBaseURL, path)
	if err != nil {
		_ = localConn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}

	remoteConn, _, err := websocket.DefaultDialer.DialContext(r.Context(), remoteURL, nil)
	if err != nil {
		_ = localConn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("remote audio connect failed: %v", err)))
		return
	}
	defer remoteConn.Close()

	if localToRemote {
		_ = pipeWebsocket(localConn, remoteConn)
		return
	}

	_ = pipeWebsocket(remoteConn, localConn)
}

func (s *LocalService) currentSession() interface {
	SubscribeRXAudio(ctx context.Context) (<-chan []float32, error)
	WriteTXAudio(ctx context.Context, samples []float32) error
} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.session
}

func pipeWebsocket(src, dst *websocket.Conn) error {
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			return err
		}
		if err := dst.WriteMessage(messageType, payload); err != nil {
			return err
		}
	}
}

func websocketURL(base, path string) (string, error) {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported remote URL scheme %q", u.Scheme)
	}

	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func floatsToBytes(samples []float32) []byte {
	payload := make([]byte, len(samples)*4)
	for i, sample := range samples {
		binary.LittleEndian.PutUint32(payload[i*4:], math.Float32bits(sample))
	}
	return payload
}

func bytesToFloats(payload []byte) ([]float32, error) {
	if len(payload)%4 != 0 {
		return nil, fmt.Errorf("audio frame length %d is not aligned to float32 samples", len(payload))
	}

	samples := make([]float32, len(payload)/4)
	for i := range samples {
		samples[i] = math.Float32frombits(binary.LittleEndian.Uint32(payload[i*4:]))
	}
	return samples, nil
}
