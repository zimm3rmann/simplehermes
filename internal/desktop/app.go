package desktop

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"simplehermes/internal/config"
)

type App struct {
	cfg        config.Config
	apiHandler http.Handler

	mu              sync.Mutex
	localServer     *http.Server
	localAPIBaseURL string
	externalServer  *http.Server
}

func New(cfg config.Config, apiHandler http.Handler) *App {
	cfg.Normalize()
	return &App{
		cfg:        cfg,
		apiHandler: apiHandler,
	}
}

type desktopInfo struct {
	APIBaseURL string `json:"apiBaseUrl"`
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/desktop", a.handleDesktopInfo)
	mux.Handle("/", a.apiHandler)
	return mux
}

func (a *App) Startup(ctx context.Context) {
	if err := a.startLocalServer(); err != nil {
		log.Printf("local desktop api server: %v", err)
	}

	if a.cfg.Mode == config.ModeServer {
		go a.startExternalServer()
	}
}

func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	localServer := a.localServer
	externalServer := a.externalServer
	a.localServer = nil
	a.externalServer = nil
	a.localAPIBaseURL = ""
	a.mu.Unlock()

	shutdownServer("local desktop api", localServer)
	shutdownServer("external api", externalServer)
}

func (a *App) handleDesktopInfo(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	info := desktopInfo{APIBaseURL: a.localAPIBaseURL}
	a.mu.Unlock()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(info)
}

func (a *App) startLocalServer() error {
	a.mu.Lock()
	if a.localServer != nil {
		a.mu.Unlock()
		return nil
	}
	a.mu.Unlock()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	baseURL := "http://" + listener.Addr().String()
	server := &http.Server{
		Handler:           a.apiHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	a.mu.Lock()
	if a.localServer != nil {
		a.mu.Unlock()
		_ = listener.Close()
		return nil
	}
	a.localServer = server
	a.localAPIBaseURL = baseURL
	a.mu.Unlock()

	go func() {
		log.Printf("SimpleHermes desktop API listening on %s", baseURL)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("local desktop api server: %v", err)
		}
	}()

	return nil
}

func shutdownServer(name string, server *http.Server) {
	if server == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("%s shutdown: %v", name, err)
	}
}

func (a *App) startExternalServer() {
	a.mu.Lock()
	if a.externalServer != nil {
		a.mu.Unlock()
		return
	}

	server := &http.Server{
		Addr:              a.cfg.ListenAddress,
		Handler:           remoteAuthHandler(a.apiHandler, a.cfg.RemoteAuthToken),
		ReadHeaderTimeout: 5 * time.Second,
	}
	a.externalServer = server
	a.mu.Unlock()

	log.Printf("SimpleHermes server API listening on %s", a.cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("external api server: %v", err)
	}
}

func remoteAuthHandler(next http.Handler, token string) http.Handler {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		if remoteAuthValid(r, token) {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("WWW-Authenticate", `Bearer realm="SimpleHermes"`)
		http.Error(w, "remote authentication required", http.StatusUnauthorized)
	})
}

func remoteAuthValid(r *http.Request, token string) bool {
	candidate := bearerToken(r.Header.Get("Authorization"))
	if candidate == "" {
		candidate = r.URL.Query().Get("access_token")
	}
	if candidate == "" || len(candidate) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) <= len("Bearer ") || !strings.EqualFold(header[:len("Bearer")], "Bearer") || header[len("Bearer")] != ' ' {
		return ""
	}
	return strings.TrimSpace(header[len("Bearer "):])
}
