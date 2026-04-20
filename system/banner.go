package system

import (
	"fmt"
)

var Version = "dev" // override at build time using -ldflags

// Banner prints a minimal startup banner.
func Banner(addr string) string {
	orange, dim, reset := "\033[38;2;206;145;120m", "\033[2m", "\033[0m"

	return fmt.Sprintf(
		"\n%s◆ Zen%s %sv%s%s\n"+
			"%s-%s Server listening on:   %s%s%s\n\n",
		orange, reset, dim, Version, reset,
		dim, reset, dim, addr, reset,
	)
}