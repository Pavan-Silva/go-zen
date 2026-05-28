package system

import (
	"fmt"
)

// Version is the current release version.
// Override at build time with: -ldflags "-X github.com/Pavan-Silva/go-zen/system.Version=v1.0.0"
var Version = "1.2.0"

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
