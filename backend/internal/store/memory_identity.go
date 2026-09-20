package store

import (
	"context"
	"errors"
	"github.com/system-design-evaluator/backend/internal/domain"
	"sort"
	"strings"
	"time"
)

func (r *MemoryRepository) CreateOIDCFlow(ctx context.Context, flow domain.OIDCFlow) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flow.StateHash = strings.TrimSpace(flow.StateHash)
	if flow.StateHash == "" || strings.TrimSpace(flow.Nonce) == "" || strings.TrimSpace(flow.CodeVerifier) == "" {
		return errors.New("OIDC flow is incomplete")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.oidcFlows[flow.StateHash]; exists {
		return errors.New("OIDC sign-in request already exists")
	}
	if flow.ExpiresAt.IsZero() {
		flow.ExpiresAt = r.clock().UTC().Add(10 * time.Minute)
	}
	r.oidcFlows[flow.StateHash] = flow
	return nil
}

func (r *MemoryRepository) ConsumeOIDCFlow(ctx context.Context, state string) (domain.OIDCFlow, error) {
	if err := ctx.Err(); err != nil {
		return domain.OIDCFlow{}, err
	}
	stateHash := sessionTokenHash(state)
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, exists := r.oidcFlows[stateHash]
	delete(r.oidcFlows, stateHash)
	if !exists || !flow.ExpiresAt.After(r.clock().UTC()) {
		return domain.OIDCFlow{}, errors.New("OIDC sign-in request is invalid or expired")
	}
	return flow, nil
}

func (r *MemoryRepository) ResolveOIDCUser(ctx context.Context, identity domain.OIDCIdentity, jitProvisioning bool) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	provider := strings.ToLower(strings.TrimSpace(identity.Provider))
	subject := strings.TrimSpace(identity.Subject)
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if provider == "" || subject == "" || email == "" {
		return domain.User{}, errors.New("OIDC identity is incomplete")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	identityKey := provider + "\x00" + subject
	userID := r.userIdentities[identityKey]
	var user domain.User
	var exists bool
	if userID != "" {
		user, exists = r.users[userID]
	}
	if !exists {
		for _, candidate := range r.users {
			if strings.EqualFold(candidate.Email, email) && candidate.Status != "disabled" {
				user = candidate
				exists = true
				break
			}
		}
	}
	if !exists {
		if !jitProvisioning {
			return domain.User{}, errors.New("user is not provisioned for Stratum")
		}
		var err error
		user, err = r.createUserLocked(identity.DisplayName, email, identity.Role, "")
		if err != nil {
			return domain.User{}, err
		}
	}
	if user.Status == "disabled" {
		return domain.User{}, errors.New("user is disabled")
	}
	if displayName := strings.TrimSpace(identity.DisplayName); displayName != "" {
		user.DisplayName = displayName
	}
	if user.Role != "admin" {
		user.Role = normalizedUserRole(identity.Role)
	}
	now := r.clock().UTC()
	user.LastSeenAt = now
	user.UpdatedAt = now
	r.users[user.ID] = user
	r.userIdentities[identityKey] = user.ID
	return user, nil
}

func (r *MemoryRepository) SyncUserAccessGroups(ctx context.Context, userID string, externalGroupNames []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[userID]; !exists {
		return errors.New("user not found")
	}
	claims := make(map[string]struct{}, len(externalGroupNames))
	for _, name := range externalGroupNames {
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			claims[normalized] = struct{}{}
		}
	}
	for key, membership := range r.groupMembers {
		group, exists := r.accessGroups[membership.GroupID]
		if !exists || membership.UserID != userID || strings.TrimSpace(group.OktaGroupName) == "" {
			continue
		}
		if _, keep := claims[strings.ToLower(strings.TrimSpace(group.OktaGroupName))]; !keep {
			delete(r.groupMembers, key)
		}
	}
	now := r.clock().UTC()
	for _, group := range r.accessGroups {
		if _, matched := claims[strings.ToLower(strings.TrimSpace(group.OktaGroupName))]; !matched {
			continue
		}
		key := accessKey(group.ID, userID)
		r.groupMembers[key] = domain.AccessGroupMember{GroupID: group.ID, UserID: userID, AddedAt: now}
	}
	return nil
}

func (r *MemoryRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	return listAllPages(ctx, func(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error) {
		return r.ListUsersPage(ctx, options)
	})
}

func (r *MemoryRepository) ListUsersPage(ctx context.Context, options PageOptions) ([]domain.User, PageInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, PageInfo{}, err
	}
	options = NormalizePageOptions(options)
	query := strings.ToLower(options.Query)
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		if query != "" && !strings.Contains(strings.ToLower(user.DisplayName+" "+user.Email+" "+user.Role), query) {
			continue
		}
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	page, info := PageFromSlice(users, options)
	return page, info, nil
}

func (r *MemoryRepository) GetUser(ctx context.Context, userID string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getUserLocked(userID)
}

func (r *MemoryRepository) AuthenticateUser(ctx context.Context, email string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	for id, user := range r.users {
		if !strings.EqualFold(user.Email, email) || user.Status == "disabled" || !user.PasswordSet {
			continue
		}
		now := r.clock().UTC()
		if until := r.lockedUntil[id]; until.After(now) {
			return domain.User{}, errors.New("invalid email or password")
		}
		if verifyPassword(password, user.PasswordHash) {
			delete(r.failedLogins, id)
			delete(r.lockedUntil, id)
			user.LastSeenAt = now
			r.users[id] = user
			return user, nil
		}
		r.failedLogins[id]++
		if r.failedLogins[id] >= 5 {
			r.failedLogins[id] = 0
			r.lockedUntil[id] = now.Add(15 * time.Minute)
		}
		return domain.User{}, errors.New("invalid email or password")
	}
	return domain.User{}, errors.New("invalid email or password")
}

func (r *MemoryRepository) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.getUserLocked(userID); err != nil {
		return "", err
	}
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(12 * time.Hour)
	}
	r.sessions[tokenHash] = memorySession{
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  expiresAt.UTC(),
	}
	return token, nil
}

func (r *MemoryRepository) GetUserBySessionToken(ctx context.Context, token string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	tokenHash := sessionTokenHash(token)
	session, ok := r.sessions[tokenHash]
	if !ok {
		return domain.User{}, errors.New("session not found")
	}
	now := r.clock().UTC()
	if !session.ExpiresAt.After(now) {
		delete(r.sessions, tokenHash)
		return domain.User{}, errors.New("session expired")
	}
	session.LastSeenAt = now
	r.sessions[tokenHash] = session
	return r.getUserLocked(session.UserID)
}

func (r *MemoryRepository) DeleteSession(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionTokenHash(token))
	return nil
}

func (r *MemoryRepository) PasswordSetupRequired(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.users) == 0 {
		return false, nil
	}
	for _, user := range r.users {
		if user.PasswordSet {
			return false, nil
		}
	}
	return true, nil
}

func (r *MemoryRepository) SetInitialAdminPassword(ctx context.Context, email string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	requiresSetup := len(r.users) > 0
	for _, user := range r.users {
		if user.PasswordSet {
			requiresSetup = false
			break
		}
	}
	if !requiresSetup {
		return domain.User{}, errors.New("password setup is not available")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for id, user := range r.users {
		if strings.EqualFold(user.Email, email) && user.Role == "admin" && user.Status != "disabled" {
			user.PasswordHash = passwordHash
			user.PasswordSet = true
			user.UpdatedAt = r.clock().UTC()
			r.users[id] = user
			return user, nil
		}
	}
	return domain.User{}, errors.New("admin user not found")
}

func (r *MemoryRepository) CreateFirstAdmin(ctx context.Context, displayName string, email string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.users) > 0 {
		return domain.User{}, errors.New("organization already has users")
	}
	user, err := r.createUserLocked(displayName, email, "admin", password)
	if err != nil {
		return domain.User{}, err
	}
	r.claimLegacyGuestDataLocked(user.ID)
	return user, nil
}

func (r *MemoryRepository) CreateUser(ctx context.Context, displayName string, email string, role string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return domain.User{}, errors.New("email is required")
	}
	if r.emailExistsLocked(email, "") {
		return domain.User{}, errors.New("a user with this email already exists")
	}
	if len(r.users) == 0 {
		return domain.User{}, errors.New("create the first admin before inviting users")
	}
	return r.createUserLocked(displayName, email, role, password)
}

func (r *MemoryRepository) UpdateUser(ctx context.Context, userID string, displayName string, email string, role string, status string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[userID]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	if strings.TrimSpace(displayName) != "" {
		user.DisplayName = strings.TrimSpace(displayName)
	}
	if strings.TrimSpace(email) != "" {
		nextEmail := strings.ToLower(strings.TrimSpace(email))
		if r.emailExistsLocked(nextEmail, user.ID) {
			return domain.User{}, errors.New("a user with this email already exists")
		}
		user.Email = nextEmail
	}
	if strings.TrimSpace(role) != "" {
		user.Role = normalizedUserRole(role)
	}
	if strings.TrimSpace(status) != "" {
		user.Status = normalizedUserStatus(status)
	}
	if strings.TrimSpace(password) != "" {
		passwordHash, err := hashPassword(password)
		if err != nil {
			return domain.User{}, err
		}
		user.PasswordHash = passwordHash
		user.PasswordSet = true
	}
	user.UpdatedAt = r.clock().UTC()
	r.users[user.ID] = user
	return user, nil
}

func (r *MemoryRepository) CreatePasswordResetToken(ctx context.Context, userID string, expiresAt time.Time) (string, domain.PasswordResetToken, error) {
	if err := ctx.Err(); err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	user, err := r.getUserLocked(userID)
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	if user.Status == "disabled" {
		return "", domain.PasswordResetToken{}, errors.New("user is disabled")
	}
	token, tokenHash, err := newPasswordResetToken()
	if err != nil {
		return "", domain.PasswordResetToken{}, err
	}
	now := r.clock().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Hour)
	}
	reset := domain.PasswordResetToken{
		Token:     tokenHash,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: expiresAt.UTC(),
	}
	r.passwordReset[tokenHash] = reset
	return token, reset, nil
}

func (r *MemoryRepository) ResetPasswordWithToken(ctx context.Context, token string, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	tokenHash := sessionTokenHash(token)
	reset, ok := r.passwordReset[tokenHash]
	now := r.clock().UTC()
	if !ok || reset.UsedAt != nil || !reset.ExpiresAt.After(now) {
		delete(r.passwordReset, tokenHash)
		return domain.User{}, errors.New("password reset link is invalid or expired")
	}
	user, ok := r.users[reset.UserID]
	if !ok || user.Status == "disabled" {
		return domain.User{}, errors.New("password reset link is invalid or expired")
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	user.PasswordHash = passwordHash
	user.PasswordSet = true
	user.UpdatedAt = now
	r.users[user.ID] = user
	reset.UsedAt = &now
	r.passwordReset[tokenHash] = reset
	return user, nil
}

func (r *MemoryRepository) DeleteUser(ctx context.Context, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	if user.Role == "admin" && r.adminCountLocked() <= 1 {
		return errors.New("at least one admin is required")
	}
	delete(r.users, userID)
	return nil
}

func (r *MemoryRepository) GetSignInConfig(ctx context.Context) (domain.SignInConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.SignInConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	config := r.signInConfig
	config.ClientSecret = ""
	return config, nil
}

func (r *MemoryRepository) GetSignInConfigWithSecret(ctx context.Context) (domain.SignInConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.SignInConfig{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.signInConfig, nil
}

func (r *MemoryRepository) UpdateSignInConfig(ctx context.Context, config domain.SignInConfig, clientSecret string) (domain.SignInConfig, error) {
	if err := ctx.Err(); err != nil {
		return domain.SignInConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current := r.signInConfig
	current.LocalPasswordEnabled = config.LocalPasswordEnabled
	current.SSOEnabled = config.SSOEnabled
	current.Provider = strings.TrimSpace(config.Provider)
	current.OktaDomain = strings.TrimSpace(config.OktaDomain)
	current.Issuer = strings.TrimSpace(config.Issuer)
	current.ClientID = strings.TrimSpace(config.ClientID)
	current.RedirectURI = strings.TrimSpace(config.RedirectURI)
	current.PostLogoutRedirectURI = strings.TrimSpace(config.PostLogoutRedirectURI)
	current.Scopes = normalizedScopes(config.Scopes)
	current.GroupsClaim = strings.TrimSpace(config.GroupsClaim)
	current.AdminGroup = strings.TrimSpace(config.AdminGroup)
	current.ReviewerGroup = strings.TrimSpace(config.ReviewerGroup)
	current.JITProvisioning = config.JITProvisioning
	if strings.TrimSpace(clientSecret) != "" {
		current.ClientSecret = clientSecret
		current.ClientSecretSet = true
	}
	current.UpdatedAt = r.clock().UTC()
	r.signInConfig = current
	return current, nil
}
