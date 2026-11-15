package main

import (
	_ "embed"
	"log"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"fyne.io/systray"
	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"

	"github.com/mariance95/lj-buddy/artnet-translator/internal/engine"
)

//go:embed assets/icon.ico
var trayIcon []byte

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	pGetSystemMetrics            = user32.NewProc("GetSystemMetrics")
	pGetDpiForSystem             = user32.NewProc("GetDpiForSystem")
	pLoadImage                   = user32.NewProc("LoadImageW")
	pSendMessage                 = user32.NewProc("SendMessageW")
	pShowWindow                  = user32.NewProc("ShowWindow")
	pIsIconic                    = user32.NewProc("IsIconic")
	pSetForegroundWindow         = user32.NewProc("SetForegroundWindow")
	pAllowSetForegroundWindow    = user32.NewProc("AllowSetForegroundWindow")
	pGetModuleHandle             = kernel32.NewProc("GetModuleHandleW")
	windowWidth, windowHeight    = 1140, 820 // tamaño inicial, en píxeles a 96 ppp
	minWidth, minHeight          = 560, 420
	screenMarginX, screenMarginY = 60, 100
)

// runUI muestra el panel en una ventana propia y el icono en la bandeja del
// sistema. Cerrar la ventana no detiene el puente: sigue en la bandeja hasta
// que el usuario elige Salir.
func runUI(ui uiOptions) {
	// La ventana y su bucle de mensajes viven siempre en este hilo.
	runtime.LockOSThread()

	trayDone := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(trayDone)
		runTray(ui)
	}()

	open := !ui.hidden
	for {
		if !open {
			select {
			case <-ui.show:
			case <-ui.quit:
			}
		}
		open = false
		select {
		case <-ui.quit:
			<-trayDone
			return
		default:
		}
		if !runWindow(ui) {
			// Sin WebView2 (Windows antiguos sin actualizar): panel en el navegador.
			openBrowser(ui.url)
		}
	}
}

func runTray(ui uiOptions) {
	systray.Run(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle(appName)
		systray.SetTooltip(appName)
		open := systray.AddMenuItem("Abrir", "Muestra la ventana de "+appName)
		systray.AddSeparator()
		exit := systray.AddMenuItem("Salir", "Detiene el puente y cierra "+appName)
		go func() {
			for {
				select {
				case <-open.ClickedCh:
					ui.showPanel()
				case <-exit.ClickedCh:
					ui.requestQuit()
				case <-ui.quit:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}

// runWindow abre la ventana con el panel y bloquea hasta que se cierra.
// Devuelve false si no se pudo crear (falta el runtime WebView2).
func runWindow(ui uiOptions) bool {
	if v, err := webviewloader.GetInstalledVersion(); err != nil || v == "" {
		log.Printf("WebView2 no está disponible (%v): el panel se abre en el navegador.", err)
		return false
	}

	scale := 96
	if pGetDpiForSystem.Find() == nil {
		if dpi, _, _ := pGetDpiForSystem.Call(); dpi > 0 {
			scale = int(dpi)
		}
	}
	px := func(v int) int { return v * scale / 96 }
	screenW, _, _ := pGetSystemMetrics.Call(0) // SM_CXSCREEN
	screenH, _, _ := pGetSystemMetrics.Call(1) // SM_CYSCREEN
	width := min(px(windowWidth), int(screenW)-px(screenMarginX))
	height := min(px(windowHeight), int(screenH)-px(screenMarginY))

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:  filepath.Join(engine.ConfigDir(), "webview"),
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  appName,
			Width:  uint(width),
			Height: uint(height),
			Center: true,
		},
	})
	if w == nil {
		log.Print("No se pudo crear la ventana: el panel se abre en el navegador.")
		return false
	}
	hwnd := uintptr(w.Window())
	setWindowIcon(hwnd)
	w.SetSize(min(px(minWidth), width), min(px(minHeight), height), webview2.HintMin)
	w.Navigate(ui.url)

	closed := make(chan struct{})
	go func() {
		for {
			select {
			case <-ui.show: // segunda instancia o "Abrir" en la bandeja
				w.Dispatch(func() { raiseWindow(hwnd) })
			case <-ui.quit:
				w.Destroy()
				return
			case <-closed:
				return
			}
		}
	}()
	w.Run()
	close(closed)
	return true
}

// setWindowIcon usa el icono incrustado en el .exe (recurso "APP" de go-winres).
// En compilaciones de desarrollo sin recursos se queda el icono por defecto.
func setWindowIcon(hwnd uintptr) {
	hinst, _, _ := pGetModuleHandle.Call(0)
	name, _ := syscall.UTF16PtrFromString("APP")
	const imageIcon, wmSetIcon = 1, 0x0080
	for _, ic := range []struct{ kind, metricX, metricY uintptr }{
		{0, 49, 50}, // ICON_SMALL: SM_CXSMICON, SM_CYSMICON
		{1, 11, 12}, // ICON_BIG:   SM_CXICON, SM_CYICON
	} {
		cx, _, _ := pGetSystemMetrics.Call(ic.metricX)
		cy, _, _ := pGetSystemMetrics.Call(ic.metricY)
		if h, _, _ := pLoadImage.Call(hinst, uintptr(unsafe.Pointer(name)), imageIcon, cx, cy, 0); h != 0 {
			pSendMessage.Call(hwnd, wmSetIcon, ic.kind, h)
		}
	}
}

func raiseWindow(hwnd uintptr) {
	if iconic, _, _ := pIsIconic.Call(hwnd); iconic != 0 {
		pShowWindow.Call(hwnd, 9) // SW_RESTORE
	}
	pSetForegroundWindow.Call(hwnd)
}

// allowForeground deja que la instancia ya abierta traiga su ventana al frente
// cuando el usuario vuelve a ejecutar el programa.
func allowForeground() {
	pAllowSetForegroundWindow.Call(^uintptr(0)) // ASFW_ANY
}
