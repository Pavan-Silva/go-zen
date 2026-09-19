package middleware

import "github.com/Pavan-Silva/go-zen"

// skipIfSkipped runs the skipper against the request and, when it says to
// skip, passes the request down the chain and reports true so the calling
// middleware can return immediately.
func skipIfSkipped(c *zen.Ctx, skipper zen.SkipFunc) bool {
	if skipper != nil && skipper(c.Request) {
		c.Next()
		return true
	}
	return false
}
