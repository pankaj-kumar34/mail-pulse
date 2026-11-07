package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
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
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
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

	// Clicking the red close button hides the window; app stays in the Dock.
	win.RegisterHook(events.Mac.WindowShouldClose, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
	})

	// Clicking the Dock icon brings the window back.
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(e *application.ApplicationEvent) {
		win.Show()
		win.Focus()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
