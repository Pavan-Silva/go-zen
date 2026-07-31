package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

// ErrOIDCKeyFuncNotConfigured is returned when SkipTokenVerification is false
// but no KeyFunc has been configured to verify access token signatures.
var ErrOIDCKeyFuncNotConfigured = errors.New("oidc: SkipTokenVerification is false but no KeyFunc is configured; set SkipTokenVerification=true to use opaque tokens or provide KeyFunc")

// OIDCUserInfo represents the user information returned by the userinfo endpoint.
type OIDCUserInfo struct {
	Sub               string `json:"sub"`                // Subject identifier (unique user ID).
	Name              string `json:"name"`               // Full display name of the user.
	PreferredUsername string `json:"preferred_username"` // Preferred username.
	Email             string `json:"email"`              // Email address.
	EmailVerified     bool   `json:"email_verified"`     // Whether the email has been verified.
	GivenName         string `json:"given_name"`         // Given (first) name.
	FamilyName        string `json:"family_name"`        // Family (last) name.
	Picture           string `json:"picture"`            // URL of the user's profile picture.
	Locale            string `json:"locale"`             // User's locale (e.g. "en-US").
}

// OIDCAuth implements OIDC authentication using access tokens.
type OIDCAuth struct {
	Issuer                string                           // OIDC issuer URL.
	ClientID              string                           // Client ID for the application.
	UserInfoEndpoint      string                           // URL of the userinfo endpoint (defaults to issuer + "/oauth2/v2/userinfo").
	HTTPClient            *http.Client                     // HTTP client for userinfo requests.
	ClaimsFunc            func(claims jwt.MapClaims) *User // Optional function to map JWT claims to a User struct.
	SkipTokenVerification bool                             // When true, skips JWT signature verification of the access token.
	KeyFunc               jwt.Keyfunc                      // Verification key for access token signatures. Required when SkipTokenVerification is false.
}

// Authenticate validates the access token by calling the userinfo endpoint.
func (o *OIDCAuth) Authenticate(r *http.Request) (*User, error) {
	if o == nil {
		return nil, fmt.Errorf("oidc auth is not configured")
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
		return nil, err
	}

	if !o.SkipTokenVerification {
		if o.KeyFunc == nil {
			return nil, ErrOIDCKeyFuncNotConfigured
		}
		if _, err := jwt.Parse(token, o.KeyFunc); err != nil {
			return nil, fmt.Errorf("access token verification failed: %w", err)
		}
	}

	userInfo, err := o.getUserInfo(r.Context(), client, token, userInfoEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
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
