package openapi

// SecuritySchemeType is the type of an OpenAPI security scheme.
type SecuritySchemeType string

// Security scheme type constants for OpenAPI security definitions.
const (
	SecurityAPIKey  SecuritySchemeType = "apiKey"
	SecurityHTTP    SecuritySchemeType = "http"
	SecurityOAuth2  SecuritySchemeType = "oauth2"
	SecurityOpenID  SecuritySchemeType = "openIdConnect"
)

// SecurityScheme defines an OpenAPI security scheme for components/securitySchemes.
type SecurityScheme struct {
	Type             SecuritySchemeType
	Description      string
	Name             string      // apiKey: header/query/cookie param name
	In               string      // apiKey: "header", "query", or "cookie"
	Scheme           string      // http: "bearer", "basic", etc.
	BearerFormat     string      // http bearer: "JWT", etc.
	OpenIDConnectURL string      // openIdConnect: OIDC discovery URL
	Flows            *OAuthFlows // oauth2
}

// OAuthFlows holds OAuth2 flow configurations.
type OAuthFlows struct {
	Implicit          *OAuthFlow
	Password          *OAuthFlow
	ClientCredentials *OAuthFlow
	AuthorizationCode *OAuthFlow
}

// OAuthFlow defines a single OAuth2 flow.
type OAuthFlow struct {
	AuthorizationURL string
	TokenURL         string
	RefreshURL       string
	Scopes           map[string]string
}

// APIKeySecurity creates an apiKey security scheme that reads from a header.
func APIKeySecurity(headerName string) SecurityScheme {
	return SecurityScheme{Type: SecurityAPIKey, Name: headerName, In: "header"}
}

// APIKeyQuerySecurity creates an apiKey security scheme that reads from a query param.
func APIKeyQuerySecurity(paramName string) SecurityScheme {
	return SecurityScheme{Type: SecurityAPIKey, Name: paramName, In: "query"}
}

// BearerSecurity creates an http bearer security scheme.
func BearerSecurity(bearerFormat string) SecurityScheme {
	return SecurityScheme{Type: SecurityHTTP, Scheme: "bearer", BearerFormat: bearerFormat}
}

// BasicSecurity creates an http basic security scheme.
func BasicSecurity() SecurityScheme {
	return SecurityScheme{Type: SecurityHTTP, Scheme: "basic"}
}

// buildSecuritySchemes converts configured schemes to OpenAPI map format.
func (o *OpenAPI) buildSecuritySchemes() map[string]any {
	if len(o.cfg.SecuritySchemes) == 0 {
		return nil
	}
	schemes := make(map[string]any, len(o.cfg.SecuritySchemes))
	for name, s := range o.cfg.SecuritySchemes {
		m := map[string]any{
			"type":        string(s.Type),
			"description": s.Description,
		}
		switch s.Type {
		case SecurityAPIKey:
			m["name"] = s.Name
			m["in"] = s.In
		case SecurityHTTP:
			m["scheme"] = s.Scheme
			if s.BearerFormat != "" {
				m["bearerFormat"] = s.BearerFormat
			}
		case SecurityOAuth2:
			if s.Flows != nil {
				m["flows"] = buildOAuthFlows(s.Flows)
			}
		case SecurityOpenID:
			m["openIdConnectUrl"] = s.OpenIDConnectURL
		}
		schemes[name] = m
	}
	return schemes
}

func buildOAuthFlows(f *OAuthFlows) map[string]any {
	m := make(map[string]any)
	if f.Implicit != nil {
		m["implicit"] = buildOAuthFlow(f.Implicit)
	}
	if f.Password != nil {
		m["password"] = buildOAuthFlow(f.Password)
	}
	if f.ClientCredentials != nil {
		m["clientCredentials"] = buildOAuthFlow(f.ClientCredentials)
	}
	if f.AuthorizationCode != nil {
		m["authorizationCode"] = buildOAuthFlow(f.AuthorizationCode)
	}
	return m
}

func buildOAuthFlow(f *OAuthFlow) map[string]any {
	m := map[string]any{
		"authorizationUrl": f.AuthorizationURL,
		"tokenUrl":         f.TokenURL,
	}
	if f.RefreshURL != "" {
		m["refreshUrl"] = f.RefreshURL
	}
	if len(f.Scopes) > 0 {
		m["scopes"] = f.Scopes
	}
	return m
}
