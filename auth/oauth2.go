package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OAuth2TokenInfo represents token information from introspection.
type OAuth2TokenInfo struct {
	Active    bool   `json:"active"`     // Whether the token is currently active.
	ClientID  string `json:"client_id"`  // Client identifier for the token.
	Subject   string `json:"sub"`        // Subject identifier (user ID).
	Username  string `json:"username"`   // Username associated with the token.
	Scope     string `json:"scope"`      // Space-separated list of scopes.
	TokenType string `json:"token_type"` // Type of token (e.g. "Bearer").
	ExpiresAt int64  `json:"exp"`        // Expiration timestamp (Unix epoch).
	IssuedAt  int64  `json:"iat"`        // Issuance timestamp (Unix epoch).
}

// OAuth2Auth implements OAuth2 token introspection authentication.
type OAuth2Auth struct {
	TokenIntrospectionEndpoint string                           // URL of the OAuth2 token introspection endpoint.
	ClientID                   string                           // Client ID for authenticating to the introspection endpoint.
	ClientSecret               string                           // Client secret for authenticating to the introspection endpoint.
	HTTPClient                 *http.Client                     // HTTP client used for introspection requests.
	ClaimsFunc                 func(claims jwt.MapClaims) *User // Optional function to convert JWT claims to a User struct.
}

// Authenticate validates the access token using OAuth2 token introspection.
func (o *OAuth2Auth) Authenticate(r *http.Request) (*User, error) {
	if o == nil {
		return nil, fmt.Errorf("oauth2 auth is not configured")
	}

	client := o.HTTPClient
	if client == nil {
		client = defaultAuthHTTPClient
	}

	token, err := bearerTokenFromRequest(r)
	if err != nil {
		return nil, err
	}

	tokenInfo, err := o.introspectToken(r.Context(), client, token)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect token: %w", err)
	}

	if !tokenInfo.Active {
		return nil, fmt.Errorf("token is not active")
	}

	// Check expiry
	if tokenInfo.ExpiresAt > 0 && time.Now().Unix() > tokenInfo.ExpiresAt {
		return nil, fmt.Errorf("token has expired")
	}

	claims := jwt.MapClaims{
		"sub":        tokenInfo.Subject,
		"client_id":  tokenInfo.ClientID,
		"username":   tokenInfo.Username,
		"scope":      tokenInfo.Scope,
		"token_type": tokenInfo.TokenType,
		"exp":        tokenInfo.ExpiresAt,
		"iat":        tokenInfo.IssuedAt,
	}

	// If a custom claims function isn't provided, build the pointer object inline
	if o.ClaimsFunc != nil {
		return o.ClaimsFunc(claims), nil
	}

	// Allocation-free setup straight to heap pointer bounds
	user := &User{
		ID:       tokenInfo.Subject,
		Username: tokenInfo.Username,
		Claims:   claims,
	}

	if user.Username == "" {
		user.Username = tokenInfo.Subject
	}

	if tokenInfo.Scope != "" {
		user.Authorities = strings.Split(tokenInfo.Scope, " ")
	}

	return user, nil
}

// introspectToken calls the OAuth2 token introspection endpoint.
func (o *OAuth2Auth) introspectToken(ctx context.Context, client *http.Client, token string) (info *OAuth2TokenInfo, err error) {
	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, "POST", o.TokenIntrospectionEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if o.ClientID != "" && o.ClientSecret != "" {
		req.SetBasicAuth(o.ClientID, o.ClientSecret)
	}

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
		return nil, fmt.Errorf("introspection request failed with status %d", resp.StatusCode)
	}

	var tokenInfo OAuth2TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, err
	}

	return &tokenInfo, nil
}
