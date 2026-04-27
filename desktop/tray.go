//go:build desktop

package desktop

import (
	goruntime "runtime"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"k8s.io/klog/v2"
)

// startTray 在后台 goroutine 中启动系统托盘
func (a *App) startTray() {
	go func() {
		goruntime.LockOSThread()
		systray.Run(a.onTrayReady, a.onTrayExit)
	}()
}

// onTrayReady 托盘就绪回调
func (a *App) onTrayReady() {
	if len(a.trayIcon) > 0 {
		systray.SetIcon(a.trayIcon)
	}
	systray.SetTooltip("Kite Proxy - Kubernetes API Forwarding Proxy")

	mShow := systray.AddMenuItem("显示窗口", "打开 Kite Proxy 主窗口")
	mHide := systray.AddMenuItem("隐藏窗口", "隐藏 Kite Proxy 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "完全退出 Kite Proxy")

	mShow.Click(func() {
		go func() {
			if a.ctx != nil {
				runtime.WindowShow(a.ctx)
				// 唤醒前端组件自动刷新数据
				runtime.EventsEmit(a.ctx, "app:wakeup")
			}
		}()
	})

	mHide.Click(func() {
		go func() {
			if a.ctx != nil {
				runtime.WindowHide(a.ctx)
			}
		}()
	})

	mQuit.Click(func() {
		go func() {
			systray.Quit()
			if a.ctx != nil {
				runtime.Quit(a.ctx)
			}
		}()
	})
}

// onTrayExit 托盘退出回调
func (a *App) onTrayExit() {
	klog.Info("System tray exited")
}
