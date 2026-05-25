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
}

// OIDCAuth implements OIDC authentication using access tokens.
type OIDCAuth struct {
	Issuer                string
	ClientID              string
	UserInfoEndpoint      string
	HTTPClient            *http.Client
	ClaimsFunc            func(claims jwt.MapClaims) *User // UPDATED: Match pointer signature contract
	SkipTokenVerification bool
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

	if !o.SkipTokenVerification && strings.Count(token, ".") == 2 {
		if _, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			return nil, fmt.Errorf("JWT signature verification not configured for OIDC access tokens; set SkipTokenVerification=true if using opaque tokens")
		}); err != nil {
			if !strings.Contains(err.Error(), "JWT signature verification not configured") {
				return nil, fmt.Errorf("access token verification failed: %w", err)
			}
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
