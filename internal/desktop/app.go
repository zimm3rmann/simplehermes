package desktop

import (
	"context"
	"log"
	"net/http"
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
		Handler:           a.apiHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	a.externalServer = server
	a.mu.Unlock()

	log.Printf("SimpleHermes server API listening on %s", a.cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("external api server: %v", err)
	}
}
