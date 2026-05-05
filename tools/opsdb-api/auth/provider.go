//# tools/opsdb-api/auth/oidc_provider.go

package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gopkg.in/yaml.v3"
)

// OIDCProvider implements auth.Provider using OIDC token validation.
// For production human authentication via Okta, Azure AD, Google, etc.
type OIDCProvider struct {
	issuerURL  string
	clientID   string
	audience   string
	httpClient *http.Client

	jwksURL   string
	jwksCache *jwksCache

	// userMapping caches OIDC subject → ops_user_id lookups.
	userMapping     map[string]int
	userMappingLock sync.RWMutex

	// db is used for ops_user lookups when the mapping cache misses.
	db UserLookup
}

// UserLookup is the interface for resolving OIDC subjects to ops_user rows.
type UserLookup interface {
	QueryRow(query string, args ...interface{}) RowScanner
}

// RowScanner is a minimal interface for scanning a single database row.
type RowScanner interface {
	Scan(dest ...interface{}) error
}

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	IssuerURL    string `yaml:"issuer_url"`
	ClientID     string `yaml:"client_id"`
	Audience     string `yaml:"audience"`
	JWKSCacheTTL int    `yaml:"jwks_cache_ttl_seconds"`
}

// jwksCache holds cached JWKS keys with expiration.
type jwksCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
}

func newJWKSCache(ttl time.Duration) *jwksCache {
	return &jwksCache{
		keys: make(map[string]*rsa.PublicKey),
		ttl:  ttl,
	}
}

func (c *jwksCache) isExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.fetchedAt) > c.ttl
}

func (c *jwksCache) getKey(kid string) (*rsa.PublicKey, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok := c.keys[kid]
	return key, ok
}

func (c *jwksCache) setKeys(keys map[string]*rsa.PublicKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keys = keys
	c.fetchedAt = time.Now()
}

// parseOIDCConfigFile reads OIDC configuration from a YAML file.
func parseOIDCConfigFile(path string) (*OIDCConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read OIDC config file %s: %w", path, err)
	}
	cfg := &OIDCConfig{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OIDC config file %s: %w", path, err)
	}

	if cfg.IssuerURL == "" {
		return nil, fmt.Errorf("OIDC config missing required field: issuer_url")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("OIDC config missing required field: client_id")
	}
	if cfg.JWKSCacheTTL <= 0 {
		cfg.JWKSCacheTTL = 3600
	}

	return cfg, nil
}

// newOIDCProviderFromConfig creates an OIDC provider from parsed configuration.
func newOIDCProviderFromConfig(cfg *OIDCConfig) (Provider, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	jwksURL, err := discoverJWKSURL(httpClient, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for %s: %w", cfg.IssuerURL, err)
	}

	cacheTTL := time.Duration(cfg.JWKSCacheTTL) * time.Second

	provider := &OIDCProvider{
		issuerURL:   cfg.IssuerURL,
		clientID:    cfg.ClientID,
		audience:    cfg.Audience,
		httpClient:  httpClient,
		jwksURL:     jwksURL,
		jwksCache:   newJWKSCache(cacheTTL),
		userMapping: make(map[string]int),
	}

	err = provider.refreshJWKS()
	if err != nil {
		return nil, fmt.Errorf("initial JWKS fetch failed: %w", err)
	}

	return provider, nil
}

// Authenticate validates an OIDC token and resolves the caller identity.
func (p *OIDCProvider) Authenticate(creds Credentials) (*Identity, error) {
	tokenStr := creds.OIDCToken
	if tokenStr == "" {
		tokenStr = creds.BearerToken
	}
	if tokenStr == "" {
		return nil, fmt.Errorf("no OIDC or bearer token provided")
	}

	if p.jwksCache.isExpired() {
		err := p.refreshJWKS()
		if err != nil {
			return nil, fmt.Errorf("JWKS refresh failed: %w", err)
		}
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token missing kid header")
		}

		key, found := p.jwksCache.getKey(kid)
		if !found {
			// keys may have rotated; refresh once and retry
			refreshErr := p.refreshJWKS()
			if refreshErr != nil {
				return nil, fmt.Errorf("JWKS refresh failed during key lookup: %w", refreshErr)
			}
			key, found = p.jwksCache.getKey(kid)
			if !found {
				return nil, fmt.Errorf("unknown signing key ID: %s", kid)
			}
		}

		return key, nil
	},
		jwt.WithIssuer(p.issuerURL),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to extract token claims")
	}

	if p.audience != "" {
		if !p.validateAudience(claims) {
			return nil, fmt.Errorf("token audience does not match expected: %s", p.audience)
		}
	}

	subject, err := claims.GetSubject()
	if err != nil || subject == "" {
		return nil, fmt.Errorf("token missing subject claim")
	}

	username := extractStringClaim(claims, "preferred_username")
	if username == "" {
		username = extractStringClaim(claims, "email")
	}
	if username == "" {
		username = subject
	}

	opsUserID, err := p.resolveOpsUser(subject, username)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ops_user for subject %s: %w", subject, err)
	}

	roles := extractStringSliceClaim(claims, "roles")
	groups := extractStringSliceClaim(claims, "groups")

	return &Identity{
		OpsUserID:  &opsUserID,
		Username:   username,
		Roles:      roles,
		Groups:     groups,
		AuthMethod: "oidc",
	}, nil
}

// RefreshToken is not supported server-side for OIDC. Clients should
// refresh tokens directly with the identity provider.
func (p *OIDCProvider) RefreshToken(token string) (*Identity, error) {
	return nil, fmt.Errorf("OIDC token refresh is not supported server-side; " +
		"the client should refresh tokens directly with the identity provider")
}

// Type returns "oidc".
func (p *OIDCProvider) Type() string {
	return "oidc"
}

// SetDB sets the database handle for ops_user lookups.
func (p *OIDCProvider) SetDB(db UserLookup) {
	p.db = db
}

// refreshJWKS fetches the JWKS from the issuer and updates the cache.
func (p *OIDCProvider) refreshJWKS() error {
	resp, err := p.httpClient.Get(p.jwksURL)
	if err != nil {
		return fmt.Errorf("JWKS fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JWKS response: %w", err)
	}

	keys, err := parseJWKS(body)
	if err != nil {
		return fmt.Errorf("failed to parse JWKS: %w", err)
	}

	p.jwksCache.setKeys(keys)
	return nil
}

// validateAudience checks the aud claim against the configured audience.
func (p *OIDCProvider) validateAudience(claims jwt.MapClaims) bool {
	aud, err := claims.GetAudience()
	if err != nil {
		return false
	}
	for _, a := range aud {
		if a == p.audience || a == p.clientID {
			return true
		}
	}
	return false
}

// resolveOpsUser maps an OIDC subject to an ops_user_id. Uses a local
// cache to avoid repeated database lookups.
func (p *OIDCProvider) resolveOpsUser(subject string, username string) (int, error) {
	p.userMappingLock.RLock()
	if userID, ok := p.userMapping[subject]; ok {
		p.userMappingLock.RUnlock()
		return userID, nil
	}
	p.userMappingLock.RUnlock()

	if p.db == nil {
		return 0, fmt.Errorf("no database configured for ops_user lookup")
	}

	var userID int
	err := p.db.QueryRow(
		"SELECT id FROM ops_user WHERE external_id = $1 OR username = $2 LIMIT 1",
		subject, username,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("ops_user not found for subject=%s username=%s: %w", subject, username, err)
	}

	p.userMappingLock.Lock()
	p.userMapping[subject] = userID
	p.userMappingLock.Unlock()

	return userID, nil
}

// discoverJWKSURL fetches the OIDC discovery document and extracts
// the jwks_uri field.
func discoverJWKSURL(client *http.Client, issuerURL string) (string, error) {
	discoveryURL := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"

	resp, err := client.Get(discoveryURL)
	if err != nil {
		return "", fmt.Errorf("discovery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read discovery response: %w", err)
	}

	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	err = json.Unmarshal(body, &doc)
	if err != nil {
		return "", fmt.Errorf("failed to parse discovery document: %w", err)
	}

	if doc.JWKSURI == "" {
		return "", fmt.Errorf("discovery document missing jwks_uri")
	}

	return doc.JWKSURI, nil
}

// parseJWKS parses a JWKS JSON document into a map of key ID → RSA public key.
func parseJWKS(data []byte) (map[string]*rsa.PublicKey, error) {
	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	err := json.Unmarshal(data, &jwks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWKS JSON: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, rawKey := range jwks.Keys {
		var header struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		err := json.Unmarshal(rawKey, &header)
		if err != nil {
			continue
		}

		if header.Kty != "RSA" || header.Kid == "" {
			continue
		}
		if header.Use != "" && header.Use != "sig" {
			continue
		}

		pubKey, err := rsaPublicKeyFromComponents(header.N, header.E)
		if err != nil {
			continue
		}

		keys[header.Kid] = pubKey
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no usable RSA signing keys found in JWKS")
	}

	return keys, nil
}

// rsaPublicKeyFromComponents constructs an RSA public key from
// base64url-encoded modulus and exponent values.
func rsaPublicKeyFromComponents(nB64 string, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)

	var e int
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

// extractStringClaim extracts a string claim from JWT claims map.
func extractStringClaim(claims jwt.MapClaims, key string) string {
	val, ok := claims[key]
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

// extractStringSliceClaim extracts a string slice claim from JWT claims.
// Handles both []string and []interface{} JSON representations.
func extractStringSliceClaim(claims jwt.MapClaims, key string) []string {
	val, ok := claims[key]
	if !ok {
		return nil
	}

	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}
