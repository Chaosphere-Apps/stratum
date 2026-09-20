package authn

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/system-design-evaluator/backend/internal/domain"
)

func TestOIDCAuthorizationCodePKCEFlow(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	issuer := "https://issuer.example"
	var expectedNonce string
	var receivedVerifier string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		response := map[string]any{}
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			response = map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}}
		case "/keys":
			response = map[string]any{"keys": []any{jose.JSONWebKey{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}}
		case "/token":
			_ = r.ParseForm()
			receivedVerifier = r.Form.Get("code_verifier")
			signer, signErr := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
			if signErr != nil {
				return nil, signErr
			}
			now := time.Now()
			raw, signErr := jwt.Signed(signer).Claims(jwt.Claims{Issuer: issuer, Subject: "subject-1", Audience: jwt.Audience{"client-1"}, Expiry: jwt.NewNumericDate(now.Add(time.Hour)), IssuedAt: jwt.NewNumericDate(now)}).Claims(map[string]any{"nonce": expectedNonce, "email": "architect@example.com", "email_verified": true, "name": "Architect", "groups": []string{"stratum-admins", "platform"}}).Serialize()
			if signErr != nil {
				return nil, signErr
			}
			response = map[string]any{"access_token": "access", "token_type": "Bearer", "expires_in": 3600, "id_token": raw}
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("not found")), Header: make(http.Header)}, nil
		}
		payload, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			return nil, marshalErr
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(payload))), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})
	ctx := oidc.ClientContext(context.Background(), &http.Client{Transport: transport})

	client := NewOIDCClient()
	config := domain.SignInConfig{SSOEnabled: true, Provider: "okta", Issuer: issuer, ClientID: "client-1", ClientSecret: "secret", RedirectURI: "http://ui.example/callback", GroupsClaim: "groups", AdminGroup: "stratum-admins"}
	authorization, err := client.Begin(ctx, config)
	if err != nil {
		t.Fatalf("Begin returned error: %v", err)
	}
	parsed, _ := url.Parse(authorization.URL)
	if parsed.Query().Get("code_challenge_method") != "S256" || parsed.Query().Get("state") != authorization.State || parsed.Query().Get("nonce") != authorization.Nonce {
		t.Fatalf("authorization URL is missing state, nonce, or PKCE: %s", authorization.URL)
	}
	expectedNonce = authorization.Nonce
	identity, err := client.Complete(ctx, config, "authorization-code", domain.OIDCFlow{Nonce: authorization.Nonce, CodeVerifier: authorization.CodeVerifier})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if identity.Subject != "subject-1" || identity.Email != "architect@example.com" || identity.Role != "admin" {
		t.Fatalf("identity = %#v", identity)
	}
	if receivedVerifier != authorization.CodeVerifier {
		t.Fatalf("token exchange verifier = %q, want %q", receivedVerifier, authorization.CodeVerifier)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }
