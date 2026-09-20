package authn

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/system-design-evaluator/backend/internal/domain"
	"golang.org/x/oauth2"
)

type providerEntry struct {
	provider   *oidc.Provider
	discovered time.Time
}

// OIDCClient owns provider discovery and cryptographic protocol validation.
// Provisioning and sessions remain application-service responsibilities.
type OIDCClient struct {
	mu        sync.Mutex
	providers map[string]providerEntry
	now       func() time.Time
}

type Authorization struct {
	URL          string
	State        string
	StateHash    string
	Nonce        string
	CodeVerifier string
	ExpiresAt    time.Time
}

func NewOIDCClient() *OIDCClient {
	return &OIDCClient{providers: map[string]providerEntry{}, now: time.Now}
}

func (c *OIDCClient) Begin(ctx context.Context, config domain.SignInConfig) (Authorization, error) {
	provider, oauthConfig, err := c.configuration(ctx, config)
	if err != nil {
		return Authorization{}, err
	}
	_ = provider
	state, err := randomURLToken(32)
	if err != nil {
		return Authorization{}, err
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		return Authorization{}, err
	}
	verifier := oauth2.GenerateVerifier()
	return Authorization{
		URL: oauthConfig.AuthCodeURL(
			state,
			oidc.Nonce(nonce),
			oauth2.S256ChallengeOption(verifier),
		),
		State:        state,
		StateHash:    HashState(state),
		Nonce:        nonce,
		CodeVerifier: verifier,
		ExpiresAt:    c.now().UTC().Add(10 * time.Minute),
	}, nil
}

func (c *OIDCClient) Complete(ctx context.Context, config domain.SignInConfig, code string, flow domain.OIDCFlow) (domain.OIDCIdentity, error) {
	provider, oauthConfig, err := c.configuration(ctx, config)
	if err != nil {
		return domain.OIDCIdentity{}, err
	}
	if strings.TrimSpace(code) == "" {
		return domain.OIDCIdentity{}, errors.New("authorization code is missing")
	}
	token, err := oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(flow.CodeVerifier))
	if err != nil {
		return domain.OIDCIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return domain.OIDCIdentity{}, errors.New("identity provider did not return an ID token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: config.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return domain.OIDCIdentity{}, fmt.Errorf("verify ID token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return domain.OIDCIdentity{}, errors.New("ID token nonce does not match the sign-in request")
	}
	var standard struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		Name          string `json:"name"`
		PreferredName string `json:"preferred_username"`
	}
	if err := idToken.Claims(&standard); err != nil {
		return domain.OIDCIdentity{}, fmt.Errorf("decode ID token claims: %w", err)
	}
	if standard.EmailVerified != nil && !*standard.EmailVerified {
		return domain.OIDCIdentity{}, errors.New("identity provider email is not verified")
	}
	if strings.TrimSpace(standard.Subject) == "" || strings.TrimSpace(standard.Email) == "" {
		return domain.OIDCIdentity{}, errors.New("ID token must contain subject and email claims")
	}
	var rawClaims map[string]json.RawMessage
	if err := idToken.Claims(&rawClaims); err != nil {
		return domain.OIDCIdentity{}, fmt.Errorf("decode ID token groups: %w", err)
	}
	groups := []string{}
	groupsClaim := strings.TrimSpace(config.GroupsClaim)
	if groupsClaim == "" {
		groupsClaim = "groups"
	}
	if raw := rawClaims[groupsClaim]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &groups); err != nil {
			return domain.OIDCIdentity{}, fmt.Errorf("groups claim %q must be an array of strings", groupsClaim)
		}
	}
	displayName := strings.TrimSpace(standard.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(standard.PreferredName)
	}
	if displayName == "" {
		displayName = strings.Split(standard.Email, "@")[0]
	}
	return domain.OIDCIdentity{
		Provider:    normalizedProvider(config.Provider),
		Subject:     standard.Subject,
		Email:       standard.Email,
		DisplayName: displayName,
		Role:        roleFromGroups(groups, config),
		Groups:      groups,
	}, nil
}

func (c *OIDCClient) VerifyConfiguration(ctx context.Context, config domain.SignInConfig) error {
	_, _, err := c.configuration(ctx, config)
	return err
}

func (c *OIDCClient) configuration(ctx context.Context, config domain.SignInConfig) (*oidc.Provider, oauth2.Config, error) {
	if !config.SSOEnabled {
		return nil, oauth2.Config{}, errors.New("SSO is disabled")
	}
	issuer := normalizedIssuer(config)
	if issuer == "" || strings.TrimSpace(config.ClientID) == "" || strings.TrimSpace(config.RedirectURI) == "" {
		return nil, oauth2.Config{}, errors.New("issuer, client ID, and redirect URI are required")
	}
	provider, err := c.provider(ctx, issuer)
	if err != nil {
		return nil, oauth2.Config{}, fmt.Errorf("discover OIDC provider: %w", err)
	}
	scopes := strings.Fields(config.Scopes)
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}
	}
	if !containsFold(scopes, oidc.ScopeOpenID) {
		scopes = append([]string{oidc.ScopeOpenID}, scopes...)
	}
	return provider, oauth2.Config{
		ClientID:     strings.TrimSpace(config.ClientID),
		ClientSecret: config.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  strings.TrimSpace(config.RedirectURI),
		Scopes:       scopes,
	}, nil
}

func (c *OIDCClient) provider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	c.mu.Lock()
	entry, ok := c.providers[issuer]
	if ok && c.now().Sub(entry.discovered) < time.Hour {
		c.mu.Unlock()
		return entry.provider, nil
	}
	c.mu.Unlock()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.providers[issuer] = providerEntry{provider: provider, discovered: c.now()}
	c.mu.Unlock()
	return provider, nil
}

func HashState(state string) string {
	sum := sha256.Sum256([]byte(state))
	// Repositories use the same non-reversible token hash representation for
	// sessions and one-time OIDC state. Keep this encoding in lockstep with
	// store.sessionTokenHash so a persisted authorization flow can be consumed.
	return base64.RawStdEncoding.EncodeToString(sum[:])
}

func randomURLToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func normalizedIssuer(config domain.SignInConfig) string {
	issuer := strings.TrimRight(strings.TrimSpace(config.Issuer), "/")
	if issuer != "" {
		return issuer
	}
	domainName := strings.TrimRight(strings.TrimSpace(config.OktaDomain), "/")
	if domainName != "" && !strings.Contains(domainName, "://") {
		domainName = "https://" + domainName
	}
	return domainName
}

func normalizedProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return "oidc"
	}
	return provider
}

func roleFromGroups(groups []string, config domain.SignInConfig) string {
	for _, group := range groups {
		if config.AdminGroup != "" && strings.EqualFold(group, config.AdminGroup) {
			return "admin"
		}
	}
	for _, group := range groups {
		if config.ReviewerGroup != "" && strings.EqualFold(group, config.ReviewerGroup) {
			return "reviewer"
		}
	}
	return "member"
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}
