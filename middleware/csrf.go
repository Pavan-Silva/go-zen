package middleware

import (
	"net/http"

	"github.com/Pavan-Silva/go-zen"
)

// CrossOriginProtectionConfig configures the native Go Cross-Origin CSRF protection.
type CrossOriginProtectionConfig struct {
	Skipper                zen.SkipFunc     // Optional function to skip CSRF checks for certain requests.
	TrustedOrigins         []string         // Origins trusted to make cross-origin requests.
	InsecureBypassPatterns []string         // URL patterns that bypass CSRF protection.
	DenyHandler            func(c *zen.Ctx) // Custom handler invoked when a request is denied.
}

// DefaultCrossOriginProtectionConfig returns an empty configuration structure.
func DefaultCrossOriginProtectionConfig() CrossOriginProtectionConfig {
	return CrossOriginProtectionConfig{}
}

// CrossOriginProtection returns a native CSRF defensive firewall using Go's modern http engine.
func CrossOriginProtection() zen.HandlerFunc {
	return CrossOriginProtectionWithConfig(DefaultCrossOriginProtectionConfig())
}

// CrossOriginProtectionWithConfig mounts the Go native CrossOriginProtection middleware.
func CrossOriginProtectionWithConfig(config CrossOriginProtectionConfig) zen.HandlerFunc {
	// Initialize the official standard library engine
	cop := http.NewCrossOriginProtection()

	for _, origin := range config.TrustedOrigins {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			panic("CSRF: unable to add trusted origin: " + err.Error())
		}
	}
	for _, pattern := range config.InsecureBypassPatterns {
		cop.AddInsecureBypassPattern(pattern)
	}

	return func(c *zen.Ctx) {
		// Respect the custom framework skipper guard
		if config.Skipper != nil && config.Skipper(c.Request) {
			c.Next()
			return
		}

		// Let the standard library inspect browser-native context headers (Sec-Fetch-Site / Origin)
		if err := cop.Check(c.Request); err != nil {
			if config.DenyHandler != nil {
				config.DenyHandler(c)
				return
			}

			// Framework Aware Error Write: Ensures your console Logger captures the 403 Forbidden state
			c.Response.WriteHeader(http.StatusForbidden)
			_, _ = c.Response.Write(zen.StringToBytes(http.StatusText(http.StatusForbidden)))
			return
		}

		c.Next()
	}
}
