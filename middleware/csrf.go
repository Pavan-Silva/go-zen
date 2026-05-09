package middleware

import (
	"net/http"

	"github.com/Pavan-Silva/go-zen"
)

type CrossOriginProtectionConfig struct {
	Skipper                zen.SkipFunc
	TrustedOrigins         []string
	InsecureBypassPatterns []string
	DenyHandler            func(c *zen.Context)
}

func DefaultCrossOriginProtectionConfig() CrossOriginProtectionConfig {
	return CrossOriginProtectionConfig{}
}

func CrossOriginProtection() zen.MiddlewareFunc {
	return CrossOriginProtectionWithConfig(DefaultCrossOriginProtectionConfig())
}

func CrossOriginProtectionWithConfig(config CrossOriginProtectionConfig) zen.MiddlewareFunc {
	cop := http.NewCrossOriginProtection()
	for _, origin := range config.TrustedOrigins {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			panic("CSRF: unable to add trusted origin: " + err.Error())
		}
	}
	for _, pattern := range config.InsecureBypassPatterns {
		cop.AddInsecureBypassPattern(pattern)
	}

	return func(c *zen.Context, next zen.NextFunc) {
		if config.Skipper != nil && config.Skipper(c.Request) {
			next(c)
			return
		}
		if err := cop.Check(c.Request); err != nil {
			if config.DenyHandler != nil {
				config.DenyHandler(c)
				return
			}
			http.Error(c.Response, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next(c)
	}
}
