package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/golang-jwt/jwt/v5"
)

// defaultAuthHTTPClient is the shared HTTP client used by auth providers.
var defaultAuthHTTPClient = &http.Client{Timeout: 10 * time.Second}

// bearerTokenFromRequest extracts the Bearer token from the Authorization header.
// The auth-scheme is matched case-insensitively per RFC 7235 section 5.1
// (e.g. "Bearer", "bearer", and "BEARER" are all accepted).
func bearerTokenFromRequest(r *http.Request) (string, error) {
	ah := r.Header["Authorization"]
	if len(ah) == 0 {
		return "", errors.New("missing authorization header")
	}

	const scheme = "bearer "
	if len(ah[0]) > len(scheme) && strings.EqualFold(ah[0][:len(scheme)], scheme) {
		return ah[0][len(scheme):], nil
	}

	return "", errors.New("invalid authorization header format")
}

// fetchJSON performs an HTTP request and decodes a 200 OK JSON response into out.
func fetchJSON(client *http.Client, req *http.Request, out any) (err error) {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

// userFromClaims maps JWT claims to a User using the optional claims function.
func userFromClaims(claimsFunc func(jwt.MapClaims) *zen.User, claims jwt.MapClaims) *zen.User {
	if claimsFunc != nil {
		return claimsFunc(claims)
	}
	return defaultUserMapper(claims)
}

// stringRoles converts a heterogeneous claim array (e.g. JSON-decoded) into a
// compact []string role list, skipping non-string and empty entries instead of
// leaving empty holes that could match empty-role checks.
func stringRoles(values []any) []string {
	roles := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			roles = append(roles, s)
		}
	}
	return roles
}
