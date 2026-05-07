package zen

import (
	"net/http"
)

// SetCookie sets a cookie in the HTTP response.
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	http.SetCookie(c.Response, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// Cookie retrieves a cookie value from the HTTP request.
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// Cookies retrieves all cookies from the HTTP request.
func (c *Context) Cookies() []*http.Cookie {
	return c.Request.Cookies()
}
