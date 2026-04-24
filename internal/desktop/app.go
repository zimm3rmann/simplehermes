package desktop

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"simplehermes/internal/config"
)

type App struct {
	cfg        config.Config
	apiHandler http.Handler

	mu             sync.Mutex
	externalServer *http.Server
}

func New(cfg config.Config, apiHandler http.Handler) *App {
	cfg.Normalize()
	return &App{
		cfg:        cfg,
		apiHandler: apiHandler,
	}
}

func (a *App) Startup(ctx context.Context) {
	if a.cfg.Mode != config.ModeServer {
		return
	}

	go a.startExternalServer()
}

func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	server := a.externalServer
	a.mu.Unlock()

	if server == nil {
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("external api shutdown: %v", err)
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
