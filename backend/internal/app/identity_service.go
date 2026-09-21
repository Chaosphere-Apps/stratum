package app

import (
	"context"
	"errors"
	"github.com/system-design-evaluator/backend/internal/domain"
	"github.com/system-design-evaluator/backend/internal/store"
	"time"
)

type IdentityService struct {
	provider RepositoryProvider
}

func (s IdentityService) repo() store.Repository { return s.provider.Repository() }

func (s IdentityService) ListUsers(ctx context.Context) ([]domain.User, error) {
	return s.repo().ListUsers(ctx)
}

func (s IdentityService) ListUsersPage(ctx context.Context, options store.PageOptions) ([]domain.User, store.PageInfo, error) {
	return s.repo().ListUsersPage(ctx, options)
}

func (s IdentityService) UserBySessionToken(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, errors.New("session is required")
	}
	return s.repo().GetUserBySessionToken(ctx, token)
}

func (s IdentityService) Authenticate(ctx context.Context, email string, password string) (domain.User, error) {
	config, err := s.repo().GetSignInConfig(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if !config.LocalPasswordEnabled {
		return domain.User{}, errors.New("local password sign-in is disabled")
	}
	return s.repo().AuthenticateUser(ctx, email, password)
}

func (s IdentityService) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	return s.repo().CreateSession(ctx, userID, expiresAt)
}

func (s IdentityService) DeleteSession(ctx context.Context, token string) error {
	return s.repo().DeleteSession(ctx, token)
}

func (s IdentityService) PasswordSetupRequired(ctx context.Context) (bool, error) {
	return s.repo().PasswordSetupRequired(ctx)
}

func (s IdentityService) CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error) {
	return s.repo().CreateFirstAdmin(ctx, displayName, email, password)
}

func (s IdentityService) SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error) {
	return s.repo().SetInitialAdminPassword(ctx, email, password)
}

func (s IdentityService) CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error) {
	return s.repo().CreateUser(ctx, displayName, email, role, password)
}

func (s IdentityService) UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error) {
	return s.repo().UpdateUser(ctx, userID, displayName, email, role, status, password)
}

func (s IdentityService) CreatePasswordResetToken(ctx context.Context, userID string, expiresAt time.Time) (string, domain.PasswordResetToken, error) {
	config, err := s.repo().GetSignInConfig(ctx)
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	if !config.LocalPasswordEnabled {
		return "", domain.PasswordResetToken{}, errors.New("local password sign-in is disabled")
	}
	return s.repo().CreatePasswordResetToken(ctx, userID, expiresAt)
}

func (s IdentityService) ResetPasswordWithToken(ctx context.Context, token string, password string) (domain.User, error) {
	config, err := s.repo().GetSignInConfig(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if !config.LocalPasswordEnabled {
		return domain.User{}, errors.New("local password sign-in is disabled")
	}
	return s.repo().ResetPasswordWithToken(ctx, token, password)
}

func (s IdentityService) DeleteUser(ctx context.Context, userID string) error {
	return s.repo().DeleteUser(ctx, userID)
}

func (s IdentityService) GetSignInConfig(ctx context.Context) (domain.SignInConfig, error) {
	return s.repo().GetSignInConfig(ctx)
}

func (s IdentityService) GetSignInConfigWithSecret(ctx context.Context) (domain.SignInConfig, error) {
	return s.repo().GetSignInConfigWithSecret(ctx)
}

func (s IdentityService) UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error) {
	if !config.LocalPasswordEnabled && !config.SSOEnabled {
		return domain.SignInConfig{}, errors.New("at least one sign-in method must remain enabled")
	}
	if config.SSOEnabled && (config.Issuer == "" || config.ClientID == "" || config.RedirectURI == "") {
		return domain.SignInConfig{}, errors.New("SSO issuer, client id, and redirect URI are required")
	}
	return s.repo().UpdateSignInConfig(ctx, config, clientSecret)
}

func (s IdentityService) CreateOIDCFlow(ctx context.Context, flow domain.OIDCFlow) error {
	return s.repo().CreateOIDCFlow(ctx, flow)
}

func (s IdentityService) ConsumeOIDCFlow(ctx context.Context, state string) (domain.OIDCFlow, error) {
	return s.repo().ConsumeOIDCFlow(ctx, state)
}

func (s IdentityService) ResolveOIDCIdentity(ctx context.Context, identity domain.OIDCIdentity, jitProvisioning bool) (domain.User, error) {
	user, err := s.repo().ResolveOIDCUser(ctx, identity, jitProvisioning)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.repo().SyncUserAccessGroups(ctx, user.ID, identity.Groups); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s IdentityService) GetAIProviderConfig(ctx context.Context) (domain.AIProviderConfig, error) {
	return s.repo().GetAIProviderConfig(ctx)
}

func (s IdentityService) GetAIProviderConfigWithSecret(ctx context.Context) (domain.AIProviderConfig, error) {
	return s.repo().GetAIProviderConfigWithSecret(ctx)
}

func (s IdentityService) UpdateAIProviderConfig(ctx context.Context, config domain.AIProviderConfig, apiKey string) (domain.AIProviderConfig, error) {
	return s.repo().UpdateAIProviderConfig(ctx, config, apiKey)
}

func (s IdentityService) GetMCPConfig(ctx context.Context) (domain.MCPConfig, error) {
	return s.repo().GetMCPConfig(ctx)
}

func (s IdentityService) UpdateMCPConfig(ctx context.Context, config domain.MCPConfig) (domain.MCPConfig, error) {
	return s.repo().UpdateMCPConfig(ctx, config)
}

func (s IdentityService) GetTelemetryIntegrationConfig(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	return s.repo().GetTelemetryIntegrationConfig(ctx)
}

func (s IdentityService) GetTelemetryIntegrationConfigWithSecret(ctx context.Context) (domain.TelemetryIntegrationConfig, error) {
	return s.repo().GetTelemetryIntegrationConfigWithSecret(ctx)
}

func (s IdentityService) UpdateTelemetryIntegrationConfig(ctx context.Context, config domain.TelemetryIntegrationConfig, secret string) (domain.TelemetryIntegrationConfig, error) {
	return s.repo().UpdateTelemetryIntegrationConfig(ctx, config, secret)
}

func (s IdentityService) ListNotifications(ctx context.Context, userID string) ([]domain.Notification, error) {
	return s.repo().ListNotifications(ctx, userID)
}

func (s IdentityService) MarkNotificationRead(ctx context.Context, userID string, notificationID string) error {
	return s.repo().MarkNotificationRead(ctx, userID, notificationID)
}
