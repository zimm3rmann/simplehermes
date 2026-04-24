package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"

	"simplehermes/internal/app"
	"simplehermes/internal/config"
	"simplehermes/internal/desktop"
	"simplehermes/internal/radio/hpsdr"
	"simplehermes/internal/web"

	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

const appVersion = "0.2.0"

//go:embed frontend/*
var frontendAssets embed.FS

func main() {
	configPathFlag := flag.String("config", "", "path to the config file")
	flag.Parse()

	configPath := *configPathFlag
	if configPath == "" {
		configPath = config.DefaultPath()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	var svc app.Service
	switch cfg.Mode {
	case config.ModeClient:
		svc = app.NewRemoteService(appVersion, cfg, configPath)
	default:
		svc = app.NewLocalService(appVersion, cfg, configPath, hpsdr.NewDriver())
	}

	apiServer := web.NewServer(appVersion, svc)
	desktopApp := desktop.New(cfg, apiServer.Handler())
	accessibilityBridge := desktop.NewAccessibilityBridge()

	assets, err := fs.Sub(frontendAssets, "frontend")
	if err != nil {
		log.Fatalf("frontend assets: %v", err)
	}

	err = wails.Run(&options.App{
		Title:            "SimpleHermes",
		Width:            1380,
		Height:           920,
		MinWidth:         1080,
		MinHeight:        720,
		BackgroundColour: &options.RGBA{R: 246, G: 241, B: 233, A: 255},
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: desktopApp.Handler(),
		},
		OnStartup:                desktopApp.Startup,
		OnShutdown:               desktopApp.Shutdown,
		Bind:                     []interface{}{accessibilityBridge},
		LogLevel:                 logger.INFO,
		LogLevelProduction:       logger.ERROR,
		EnableDefaultContextMenu: true,
	})
	if err != nil {
		log.Fatal(err)
	}
}
