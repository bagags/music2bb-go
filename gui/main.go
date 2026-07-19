package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/unit"
	music2bb "github.com/bagags/music2bb-go"
	"github.com/bagags/music2bb-go/m2bb-gui/internal/ui"
)

var version = "dev"

func main() {
	configDir := flag.String("config-dir", "", "portable music2bb state directory")
	browserExecutable := flag.String("browser-executable", "", "path to a system Chromium or Google Chrome executable")
	flag.Parse()

	engine, err := music2bb.New(music2bb.Config{
		ConfigDir:           *configDir,
		HTTPTimeout:         30 * time.Second,
		RatePerSecond:       2,
		SearchRatePerSecond: 2,
		Login:               music2bb.LoginOptions{UseStoredCookies: true, AllowQR: true, Timeout: 3 * time.Minute},
		Browser: music2bb.BrowserOptions{
			Policy:         music2bb.BrowserAuto,
			ExecutablePath: *browserExecutable,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	go func() {
		window := new(app.Window)
		window.Option(
			app.Title("music2bb Desktop"),
			app.Size(unit.Dp(1180), unit.Dp(760)),
			app.MinSize(unit.Dp(860), unit.Dp(560)),
		)
		gui := ui.New(window, engine, version)
		if err := gui.Run(); err != nil {
			log.Print(err)
		}
		_ = engine.Close()
		os.Exit(0)
	}()
	app.Main()
}
