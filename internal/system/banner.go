// Package system provides build-time metadata (version, banner) for the zen framework.
package system

import (
	"fmt"
	"net"
)

// Version is the current release version.
// Override at build time with: -ldflags "-X github.com/Pavan-Silva/go-zen/internal/system.Version=v1.5.1"
var Version = "1.5.1"

// Banner returns the startup banner string for the given listen address.
// A side-accent-bar layout: the brand line and the listen URL (https:// when
// secure) share a left rule, and the URL is rendered as plain text that
// terminals auto-detect and make clickable.
func Banner(addr string, secure bool) string {
	orange, gray, reset := "\033[38;2;206;145;120m", "\033[38;2;140;140;140m", "\033[0m"

	return "\n" +
		orange + "▌" + reset + " ZEN  " + gray + "v" + Version + reset + "\n" +
		orange + "▌" + reset + " " + gray + "Server listening on:" + reset + " " + displayURL(addr, secure) + "\n\n"
}

// displayURL normalizes a listen address into a URL that opens in a browser.
// Wildcard hosts (:8080, 0.0.0.0:8080, [::]:8080) map to localhost.
func displayURL(addr string, secure bool) string {
	scheme := "http"
	if secure {
		scheme = "https"
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return scheme + "://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s:%s", scheme, host, port)
}
