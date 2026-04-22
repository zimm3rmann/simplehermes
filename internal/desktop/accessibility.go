package desktop

import (
	"io"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type AccessibilityBridge struct {
	mu             sync.Mutex
	lastSpokenText string
	lastSpokenAt   time.Time
}

func NewAccessibilityBridge() *AccessibilityBridge {
	return &AccessibilityBridge{}
}

func (a *AccessibilityBridge) Announce(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	a.mu.Lock()
	if a.lastSpokenText == text && time.Since(a.lastSpokenAt) < 800*time.Millisecond {
		a.mu.Unlock()
		return
	}
	a.lastSpokenText = text
	a.lastSpokenAt = time.Now()
	a.mu.Unlock()

	go func() {
		if err := speakText(text); err != nil {
			log.Printf("accessibility announce: %v", err)
		}
	}()
}

func speakText(text string) error {
	switch runtime.GOOS {
	case "windows":
		return speakTextWindows(text)
	default:
		return speakTextUnix(text)
	}
}

func speakTextUnix(text string) error {
	commands := [][]string{
		{"spd-say", "--wait", text},
		{"espeak-ng", text},
		{"espeak", text},
	}

	for _, args := range commands {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		return exec.Command(args[0], args[1:]...).Run()
	}

	return nil
}

func speakTextWindows(text string) error {
	powershell, err := exec.LookPath("powershell.exe")
	if err != nil {
		powershell, err = exec.LookPath("powershell")
		if err != nil {
			return nil
		}
	}

	script := "[Console]::InputEncoding=[Text.Encoding]::UTF8; $text=[Console]::In.ReadToEnd(); Add-Type -AssemblyName System.Speech; $synth=New-Object System.Speech.Synthesis.SpeechSynthesizer; $synth.Speak($text)"
	cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
