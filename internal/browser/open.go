package browser

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Open tries to open a URL in the user's default browser.
// CLIAMP_NO_BROWSER=1 disables the launch entirely: a headless daemon
// spawning browsers from a unit environment can take the session browser
// down, and the URL is logged for the operator regardless.
func Open(u string) error {
	if os.Getenv("CLIAMP_NO_BROWSER") == "1" {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Start()
	case "linux":
		return exec.Command("xdg-open", u).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
