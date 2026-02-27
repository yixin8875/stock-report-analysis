package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func initLogger() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	// In `wails dev`, backend stdout is visible in the terminal.
	log.SetOutput(os.Stdout)
	log.Printf("[BOOT] logger ready: stdout")
}

func main() {
	initLogger()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "股票报告 AI 解读",
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
