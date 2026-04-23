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
	Active    bool   `json:"active"`
	ClientID  string `json:"client_id"`
	Subject   string `json:"sub"`
	Username  string `json:"username"`
	Scope     string `json:"scope"`
	TokenType string `json:"token_type"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

// OAuth2Auth implements OAuth2 token introspection authentication.
type OAuth2Auth struct {
	// TokenIntrospectionEndpoint is the OAuth2 token introspection endpoint
	TokenIntrospectionEndpoint string
	// ClientID for the resource server
	ClientID string
	// ClientSecret for the resource server
	ClientSecret string
	// HTTPClient for making requests (optional)
	HTTPClient *http.Client
	// ClaimsFunc for custom mapping (optional)
	ClaimsFunc func(claims jwt.MapClaims) User
}

// Authenticate validates the access token using OAuth2 token introspection.
func (o *OAuth2Auth) Authenticate(r *http.Request) (User, error) {
	client := o.HTTPClient
	if client == nil {
		client = defaultAuthHTTPClient
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return User{}, fmt.Errorf("missing authorization header")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return User{}, fmt.Errorf("invalid authorization header format")
	}

	tokenInfo, err := o.introspectToken(r.Context(), client, token)
	if err != nil {
		return User{}, fmt.Errorf("failed to introspect token: %w", err)
	}

	if !tokenInfo.Active {
		return User{}, fmt.Errorf("token is not active")
	}

	// Check expiry
	if tokenInfo.ExpiresAt > 0 && time.Now().Unix() > tokenInfo.ExpiresAt {
		return User{}, fmt.Errorf("token has expired")
	}

	claims := jwt.MapClaims{
		"sub":        tokenInfo.Subject,
		"client_id":  tokenInfo.ClientID,
		"username":  tokenInfo.Username,
		"scope":      tokenInfo.Scope,
		"token_type": tokenInfo.TokenType,
		"exp":        tokenInfo.ExpiresAt,
		"iat":        tokenInfo.IssuedAt,
	}

	if o.ClaimsFunc != nil {
		return o.ClaimsFunc(claims), nil
	}

	user := User{
		ID:       tokenInfo.Subject,
		Username: tokenInfo.Username,
		Claims:   claims,
	}

	if user.Username == "" {
		user.Username = tokenInfo.Subject
	}

	if tokenInfo.Scope != "" {
		user.Roles = strings.Split(tokenInfo.Scope, " ")
	}

	return user, nil
}

// introspectToken calls the OAuth2 token introspection endpoint.
func (o *OAuth2Auth) introspectToken(ctx context.Context, client *http.Client, token string) (*OAuth2TokenInfo, error) {
	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, "POST", o.TokenIntrospectionEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Basic auth for client credentials
	if o.ClientID != "" && o.ClientSecret != "" {
		req.SetBasicAuth(o.ClientID, o.ClientSecret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection request failed with status %d", resp.StatusCode)
	}

	var tokenInfo OAuth2TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, err
	}

	return &tokenInfo, nil
}