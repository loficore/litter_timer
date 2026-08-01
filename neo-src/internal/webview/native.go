//go:build webview

// Native webview implementation — compiled only when `-tags webview`
// is passed.  Without the tag, `stub.go` provides the same `run()`
// function with a friendly error so the binary still boots.

package webview

import (
	"fmt"

	"github.com/webview/webview_go"
)

func run() error {
	w := webview.New(false)
	if w == nil {
		return fmt.Errorf("webview: failed to create window — on Linux, install one of: gtk4+webkitgtk-6.0, gtk+-3.0+webkit2gtk-4.1, gtk+-3.0+webkit2gtk-4.0\n%s", missingDepsHint())
	}
	defer w.Destroy()

	w.SetTitle(Title)
	w.SetSize(DefaultWidth, DefaultHeight, webview.HintNone)
	w.Navigate(appURL)
	w.Run()
	return nil
}