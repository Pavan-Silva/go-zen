package system

import (
	"fmt"
	"strings"
)

// Banner returns the ASCII art for startup.
func Banner(addr string) string {
	display := strings.TrimPrefix(addr, ":")
	if display == "" {
		display = "80"
	}

	url := "http://localhost:" + display
	orange, green, gray, bold, reset := "\033[38;2;206;145;120m", "\033[32m", "\033[90m", "\033[1m", "\033[0m"

	art := `
   ███████╗███████╗███╗   ██╗
   ╚══███╔╝██╔════╝████╗  ██║
     ███╔╝ █████╗  ██╔██╗ ██║
    ███╔╝  ██╔══╝  ██║╚██╗██║
   ███████╗███████╗██║ ╚████║
   ╚══════╝╚══════╝╚═╝  ╚═══╝`

	return fmt.Sprintf("%s%s%s%s\n  %sDeveloped by Pavan Silva%s %s(@Pavan-Silva)%s\n  %sServer running at:%s %s%s%s\n\n",
		bold, orange, art, reset, bold, reset, gray, reset, bold, reset, green, url, reset)
}
