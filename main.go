package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"gutgd/backend"
)

//go:embed all:frontend/dist
var embeddedAssets embed.FS

func main() {
	service := backend.NewService()
	frontendAssets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "gutgd",
		Description: "Gut graphical debugger for exercising the gut Go rewrite",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(frontendAssets),
		},
		Services: []application.Service{
			application.NewService(service),
		},
	})
	service.SetEventEmitter(func(name string, data any) {
		app.Event.Emit(name, data)
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "gutgd",
		Width:            1480,
		Height:           960,
		MinWidth:         1180,
		MinHeight:        760,
		BackgroundColour: application.NewRGB(17, 24, 39),
		URL:              "/",
	})
	window.Show()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
