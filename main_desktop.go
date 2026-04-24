//go:build desktop

package main

import (
	"embed"
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/zxh326/kite-proxy/desktop"
)

//go:embed all:ui/dist
var assets embed.FS

func main() {
	// 解析命令行参数（用于调试）
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	// 创建应用实例
	app := desktop.NewApp()

	// 应用配置
	err := wails.Run(&options.App{
		Title:  "Kite Proxy",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: *debug,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
