package zen

import (
	"net/http"
)

// SetCookie sets a cookie on the HTTP response.
// This is a convenience wrapper around http.SetCookie that uses the
// Context's ResponseWriter. For full cookie configuration, create an
// *http.Cookie with all desired fields and pass it here.
//
// Example:
//
//	c.SetCookie(&http.Cookie{
//	    Name:     "session",
//	    Value:    sessionID,
//	    Path:     "/",
//	    HttpOnly: true,
//	    Secure:   true,
//	    SameSite: http.SameSiteLaxMode,
//	})
func (c *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(c.Response, cookie)
}

// Cookie retrieves the value of a named cookie from the request.
// Returns an error if the cookie is not present (http.ErrNoCookie).
//
// Example:
//
//	val, err := c.Cookie("session")
//	if err != nil {
//	    // cookie not found
//	}
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// Cookies returns all cookies sent with the request.
//
// Example:
//
//	for _, cookie := range c.Cookies() {
//	    log.Printf("cookie: %s=%s", cookie.Name, cookie.Value)
//	}
func (c *Context) Cookies() []*http.Cookie {
	return c.Request.Cookies()
}
