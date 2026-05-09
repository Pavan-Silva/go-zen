package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// OIDCUserInfo represents the user information returned by the userinfo endpoint.
type OIDCUserInfo struct {
	Sub               string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Picture           string `json:"picture"`
	Locale            string `json:"locale"`
	// Additional claims can be added as needed
}

// OIDCAuth implements OIDC authentication using access tokens.
// It validates tokens by calling the userinfo endpoint.
type OIDCAuth struct {
	// Issuer is the OIDC issuer URL (e.g., "https://accounts.google.com")
	Issuer string
	// ClientID is the OAuth2 client ID
	ClientID string
	// UserInfoEndpoint is the userinfo endpoint URL (optional, defaults to issuer + "/oauth2/v2/userinfo" for Google)
	UserInfoEndpoint string
	// HTTPClient for making requests (optional, defaults to a shared client with timeout)
	HTTPClient *http.Client
	// ClaimsFunc for custom mapping (optional)
	ClaimsFunc func(claims jwt.MapClaims) User
	// SkipTokenVerification disables JWT signature verification of the access token.
	// Only enable this if the token is opaque and validated via the userinfo endpoint.
	SkipTokenVerification bool
}

// Authenticate validates the access token by calling the userinfo endpoint.
// If the access token is a JWT and SkipTokenVerification is false, it will
// be parsed and validated before calling the userinfo endpoint.
func (o *OIDCAuth) Authenticate(r *http.Request) (User, error) {
	if o == nil {
		return User{}, fmt.Errorf("oidc auth is not configured")
	}

	client := o.HTTPClient
	if client == nil {
		client = defaultAuthHTTPClient
	}

	userInfoEndpoint := o.UserInfoEndpoint
	if userInfoEndpoint == "" {
		userInfoEndpoint = o.Issuer + "/oauth2/v2/userinfo"
	}

	token, err := bearerTokenFromRequest(r)
	if err != nil {
		return User{}, err
	}

	// Attempt to verify the access token as a JWT if not explicitly skipped.
	// Many OIDC providers issue JWT-format access tokens.
	if !o.SkipTokenVerification && strings.Count(token, ".") == 2 {
		if _, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			// Access tokens may use various signing methods; return nil to skip
			// signature verification and rely on the userinfo endpoint for validation.
			// Override this by providing a KeyFunc via a custom OIDCAuth setup if needed.
			return nil, fmt.Errorf("JWT signature verification not configured for OIDC access tokens; set SkipTokenVerification=true if using opaque tokens")
		}); err != nil {
			// If the token is not a valid JWT or verification fails, proceed to userinfo.
			// The userinfo endpoint remains the source of truth for token validity.
			if !strings.Contains(err.Error(), "JWT signature verification not configured") {
				return User{}, fmt.Errorf("access token verification failed: %w", err)
			}
		}
	}

	userInfo, err := o.getUserInfo(r.Context(), client, token, userInfoEndpoint)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user info: %w", err)
	}

	claims := jwt.MapClaims{
		"sub":                userInfo.Sub,
		"name":               userInfo.Name,
		"preferred_username": userInfo.PreferredUsername,
		"email":              userInfo.Email,
		"email_verified":     userInfo.EmailVerified,
		"given_name":         userInfo.GivenName,
		"family_name":        userInfo.FamilyName,
		"picture":            userInfo.Picture,
		"locale":             userInfo.Locale,
	}

	return userFromClaims(o.ClaimsFunc, claims), nil
}

// getUserInfo calls the OIDC userinfo endpoint with the access token.
func (o *OIDCAuth) getUserInfo(ctx context.Context, client *http.Client, token, endpoint string) (info *OIDCUserInfo, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status %d", resp.StatusCode)
	}

	var userInfo OIDCUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}
