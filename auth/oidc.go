package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
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
	// HTTPClient for making requests (optional, defaults to http.DefaultClient)
	HTTPClient *http.Client
}

// Authenticate validates the access token by calling the userinfo endpoint.
func (o *OIDCAuth) Authenticate(r *http.Request) (User, error) {
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	userInfoEndpoint := o.UserInfoEndpoint
	if userInfoEndpoint == "" {
		userInfoEndpoint = o.Issuer + "/oauth2/v2/userinfo"
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return User{}, fmt.Errorf("missing authorization header")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return User{}, fmt.Errorf("invalid authorization header format")
	}

	userInfo, err := o.getUserInfo(r.Context(), client, token, userInfoEndpoint)
	if err != nil {
		return User{}, fmt.Errorf("failed to get user info: %w", err)
	}

	// Map OIDC claims to User struct
	user := User{
		ID:       userInfo.Sub,
		Username: userInfo.PreferredUsername,
		Claims: map[string]any{
			"name":           userInfo.Name,
			"email":          userInfo.Email,
			"email_verified": userInfo.EmailVerified,
			"given_name":     userInfo.GivenName,
			"family_name":    userInfo.FamilyName,
			"picture":        userInfo.Picture,
			"locale":         userInfo.Locale,
		},
	}

	if user.Username == "" {
		user.Username = userInfo.Email
	}

	return user, nil
}

// getUserInfo calls the OIDC userinfo endpoint with the access token.
func (o *OIDCAuth) getUserInfo(ctx context.Context, client *http.Client, token, endpoint string) (*OIDCUserInfo, error) {
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status %d", resp.StatusCode)
	}

	var userInfo OIDCUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}