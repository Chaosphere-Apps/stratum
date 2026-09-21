package httpapi

import (
	"encoding/json"
	"github.com/system-design-evaluator/backend/internal/authn"
	"github.com/system-design-evaluator/backend/internal/config"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/realtime"
	"github.com/system-design-evaluator/backend/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProfileRequiresSessionAfterUsersExist(t *testing.T) {
	repo := store.NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/profile", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("profile without session status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAuthConfigPublishesPasswordPolicy(t *testing.T) {
	repo := store.NewMemoryRepository()
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("auth config status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	var response struct {
		PasswordPolicy authn.PasswordPolicy `json:"passwordPolicy"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode auth config: %v", err)
	}
	if response.PasswordPolicy.MinimumLength != authn.MinimumPasswordLength {
		t.Fatalf("minimum password length = %d, want %d", response.PasswordPolicy.MinimumLength, authn.MinimumPasswordLength)
	}
}

func TestProfileReturnsUserForValidSession(t *testing.T) {
	repo := store.NewMemoryRepository()
	user, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), user.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{AllowedOrigins: []string{"http://ui.local"}, PublicURL: "http://ui.local"}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), user.Email) {
		t.Fatalf("profile with session returned status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestPasswordResetLinkIgnoresUntrustedForwardingAndOriginHeaders(t *testing.T) {
	server := NewServer(config.Config{}, nil, config.Logger())
	request := httptest.NewRequest(http.MethodPost, "http://stratum.internal/api/admin/users/u/reset", nil)
	request.RemoteAddr = "203.0.113.9:44100"
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("X-Forwarded-Host", "attacker.example")
	request.Header.Set("X-Forwarded-Proto", "https")
	link := server.passwordResetLink(request, "secret")
	if link != "" {
		t.Fatalf("unconfigured public origin should reject reset link generation: %q", link)
	}
}

func TestPasswordResetLinkUsesConfiguredPublicURLDespiteForwardingHeaders(t *testing.T) {
	server := NewServer(config.Config{PublicURL: "https://stratum.example.com", TrustedProxyCIDRs: []string{"10.0.0.0/8"}}, nil, config.Logger())
	request := httptest.NewRequest(http.MethodPost, "http://backend:8081/api/admin/users/u/reset", nil)
	request.RemoteAddr = "10.2.3.4:44100"
	request.Header.Set("X-Forwarded-Host", "stratum.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	link := server.passwordResetLink(request, "secret")
	if link != "https://stratum.example.com/reset-password?token=secret" {
		t.Fatalf("trusted proxy reset link = %q", link)
	}
}

func TestLoginSetsHttpOnlyCookieAndDoesNotReturnToken(t *testing.T) {
	repo := store.NewMemoryRepository()
	if _, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123"); err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"password123"}`))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"token"`) {
		t.Fatalf("login response leaked token: %q", recorder.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "stratum_session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("session cookie = %#v, want secure HTTP-only browser cookie", sessionCookie)
	}

	profile := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	profile.AddCookie(sessionCookie)
	profileRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(profileRecorder, profile)
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("profile after login status = %d body=%q", profileRecorder.Code, profileRecorder.Body.String())
	}

	badLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"admin@example.com","password":"wrong"}`))
	badRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(badRecorder, badLogin)
	if badRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want %d", badRecorder.Code, http.StatusUnauthorized)
	}
}

func TestOIDCAuthorizationCodeFlowCreatesBrowserSession(t *testing.T) {
	repo := store.NewMemoryRepository()
	signIn, err := repo.GetSignInConfig(t.Context())
	if err != nil {
		t.Fatalf("GetSignInConfig returned error: %v", err)
	}
	signIn.SSOEnabled = true
	signIn.Provider = "okta"
	signIn.Issuer = "https://identity.example.com/oauth2/default"
	signIn.ClientID = "stratum-client"
	signIn.RedirectURI = "http://ui.local/api/auth/oidc/callback"
	signIn.JITProvisioning = true
	if _, err := repo.UpdateSignInConfig(t.Context(), signIn, "client-secret"); err != nil {
		t.Fatalf("UpdateSignInConfig returned error: %v", err)
	}

	protocol := &fakeOIDCProtocol{
		begin: authn.Authorization{
			URL:          "https://identity.example.com/authorize?state=browser-state",
			State:        "browser-state",
			StateHash:    authn.HashState("browser-state"),
			Nonce:        "nonce",
			CodeVerifier: "verifier",
			ExpiresAt:    time.Now().UTC().Add(10 * time.Minute),
		},
		identity: domain.OIDCIdentity{
			Provider:    "okta",
			Subject:     "subject-1",
			Email:       "architect@example.com",
			DisplayName: "Architect",
			Role:        "architect",
			Groups:      []string{"stratum-architects"},
		},
	}
	server := NewServer(
		config.Config{PublicURL: "http://ui.local", AllowedOrigins: []string{"http://ui.local"}},
		realtime.NewHub(repo, config.Logger()),
		config.Logger(),
	)
	server.oidc = protocol

	start := httptest.NewRequest(http.MethodGet, "http://ui.local/api/auth/oidc/start", nil)
	startRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRecorder, start)
	if startRecorder.Code != http.StatusFound || startRecorder.Header().Get("Location") != protocol.begin.URL {
		t.Fatalf("OIDC start status=%d location=%q", startRecorder.Code, startRecorder.Header().Get("Location"))
	}
	var stateCookie *http.Cookie
	for _, cookie := range startRecorder.Result().Cookies() {
		if cookie.Name == oidcStateCookieName {
			stateCookie = cookie
			break
		}
	}
	if stateCookie == nil || stateCookie.Value != "browser-state" || !stateCookie.HttpOnly {
		t.Fatalf("OIDC state cookie = %#v", stateCookie)
	}

	callback := httptest.NewRequest(http.MethodGet, "http://ui.local/api/auth/oidc/callback?state=browser-state&code=authorization-code", nil)
	callback.AddCookie(stateCookie)
	callbackRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(callbackRecorder, callback)
	if callbackRecorder.Code != http.StatusFound || callbackRecorder.Header().Get("Location") != "http://ui.local/" {
		t.Fatalf("OIDC callback status=%d location=%q body=%q", callbackRecorder.Code, callbackRecorder.Header().Get("Location"), callbackRecorder.Body.String())
	}
	if !protocol.completed || protocol.completion.Nonce != "nonce" || protocol.completion.CodeVerifier != "verifier" {
		t.Fatalf("OIDC completion did not receive persisted flow: %#v", protocol.completion)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range callbackRecorder.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" || !sessionCookie.HttpOnly {
		t.Fatalf("session cookie = %#v", sessionCookie)
	}
	user, err := repo.GetUserBySessionToken(t.Context(), sessionCookie.Value)
	if err != nil || user.Email != "architect@example.com" || user.Role != "architect" {
		t.Fatalf("OIDC session user = %#v, err=%v", user, err)
	}

	replay := httptest.NewRequest(http.MethodGet, "http://ui.local/api/auth/oidc/callback?state=browser-state&code=authorization-code", nil)
	replay.AddCookie(stateCookie)
	replayRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(replayRecorder, replay)
	if replayRecorder.Code != http.StatusFound || !strings.Contains(replayRecorder.Header().Get("Location"), "authError=expired_request") {
		t.Fatalf("replayed OIDC callback status=%d location=%q", replayRecorder.Code, replayRecorder.Header().Get("Location"))
	}
}

func TestPasswordResetLinkFlow(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{AllowedOrigins: []string{"http://ui.local"}, PublicURL: "http://ui.local"}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+member.ID+"/password-reset-link", strings.NewReader(`{}`))
	request.Header.Set("Origin", "http://ui.local")
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("reset link status = %d body=%q", recorder.Code, recorder.Body.String())
	}
	var linkResponse struct {
		ResetLink string `json:"resetLink"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &linkResponse); err != nil {
		t.Fatalf("decode reset link response: %v", err)
	}
	if !strings.HasPrefix(linkResponse.ResetLink, "http://ui.local/reset-password?token=") {
		t.Fatalf("reset link used wrong origin: %q", linkResponse.ResetLink)
	}
	parsed, err := url.Parse(linkResponse.ResetLink)
	if err != nil {
		t.Fatalf("parse reset link: %v", err)
	}
	resetToken := parsed.Query().Get("token")
	if resetToken == "" {
		t.Fatal("reset token missing")
	}

	reset := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset", strings.NewReader(`{"token":"`+resetToken+`","password":"new-password123"}`))
	resetRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(resetRecorder, reset)
	if resetRecorder.Code != http.StatusOK {
		t.Fatalf("reset password status = %d body=%q", resetRecorder.Code, resetRecorder.Body.String())
	}
	reuse := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset", strings.NewReader(`{"token":"`+resetToken+`","password":"another-password123"}`))
	reuseRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(reuseRecorder, reuse)
	if reuseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("reused reset token status = %d, want %d", reuseRecorder.Code, http.StatusBadRequest)
	}
	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"member@example.com","password":"new-password123"}`))
	loginRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRecorder, login)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login with reset password status = %d body=%q", loginRecorder.Code, loginRecorder.Body.String())
	}
}

func TestPasswordResetDisabledWhenLocalPasswordsDisabled(t *testing.T) {
	repo := store.NewMemoryRepository()
	admin, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	member, err := repo.CreateUser(t.Context(), "Member", "member@example.com", "member", "")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	signInConfig, err := repo.GetSignInConfig(t.Context())
	if err != nil {
		t.Fatalf("GetSignInConfig returned error: %v", err)
	}
	signInConfig.LocalPasswordEnabled = false
	signInConfig.SSOEnabled = true
	if _, err := repo.UpdateSignInConfig(t.Context(), signInConfig, ""); err != nil {
		t.Fatalf("UpdateSignInConfig returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), admin.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+member.ID+"/password-reset-link", strings.NewReader(`{}`))
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "local password sign-in is disabled") {
		t.Fatalf("disabled reset link status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	repo := store.NewMemoryRepository()
	user, err := repo.CreateFirstAdmin(t.Context(), "Admin", "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateFirstAdmin returned error: %v", err)
	}
	token, err := repo.CreateSession(t.Context(), user.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	server := NewServer(config.Config{}, realtime.NewHub(repo, config.Logger()), config.Logger())

	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: "stratum_session", Value: token})
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if _, err := repo.GetUserBySessionToken(t.Context(), token); err == nil {
		t.Fatal("session token should be deleted after logout")
	}
	var cleared bool
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "stratum_session" && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("logout did not clear session cookie: %#v", recorder.Result().Cookies())
	}
}
