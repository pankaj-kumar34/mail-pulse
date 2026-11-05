package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logSvc := NewLogService()
	mailQSvc := NewMailQueueService()
	cosSvc := NewCOSService(logSvc)

	app := application.New(application.Options{
		Name:        "Mail Pulse",
		Description: "Postfix log monitoring dashboard",
		Services: []application.Service{
			application.NewService(logSvc),
			application.NewService(mailQSvc),
			application.NewService(cosSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Mail Pulse",
		Width:           1440,
		Height:          900,
		MinWidth:        1100,
		MinHeight:       700,
		InitialPosition: application.WindowCentered,
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarDefault,
			Backdrop:   application.MacBackdropNormal,
			Appearance: application.NSAppearanceNameAqua,
		},
		BackgroundColour: application.NewRGB(28, 28, 30),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
