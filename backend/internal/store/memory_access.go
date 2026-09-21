package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/system-design-evaluator/backend/internal/domain"
	"sort"
	"strings"
)

func (r *MemoryRepository) ListAccessGroups(ctx context.Context) ([]domain.AccessGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	groups := make([]domain.AccessGroup, 0, len(r.accessGroups))
	for _, group := range r.accessGroups {
		group.MemberCount = r.accessGroupMemberCountLocked(group.ID)
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name) })
	return groups, nil
}

func (r *MemoryRepository) CreateAccessGroup(ctx context.Context, group domain.AccessGroup) (domain.AccessGroup, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccessGroup{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return domain.AccessGroup{}, errors.New("group name is required")
	}
	for _, existing := range r.accessGroups {
		if strings.EqualFold(existing.Name, name) {
			return domain.AccessGroup{}, errors.New("group name already exists")
		}
	}
	now := r.clock().UTC()
	group.ID = fmt.Sprintf("grp_%d", now.UnixNano())
	group.Name = name
	group.Description = strings.TrimSpace(group.Description)
	group.OktaGroupName = strings.TrimSpace(group.OktaGroupName)
	group.CreatedAt = now
	group.UpdatedAt = now
	r.accessGroups[group.ID] = group
	return group, nil
}

func (r *MemoryRepository) UpdateAccessGroup(ctx context.Context, groupID string, group domain.AccessGroup) (domain.AccessGroup, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccessGroup{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	groupID = strings.TrimSpace(groupID)
	existing, ok := r.accessGroups[groupID]
	if !ok {
		return domain.AccessGroup{}, errors.New("group not found")
	}
	name := strings.TrimSpace(group.Name)
	if name == "" {
		return domain.AccessGroup{}, errors.New("group name is required")
	}
	for _, other := range r.accessGroups {
		if other.ID != groupID && strings.EqualFold(other.Name, name) {
			return domain.AccessGroup{}, errors.New("group name already exists")
		}
	}
	existing.Name = name
	existing.Description = strings.TrimSpace(group.Description)
	existing.OktaGroupName = strings.TrimSpace(group.OktaGroupName)
	existing.UpdatedAt = r.clock().UTC()
	existing.MemberCount = r.accessGroupMemberCountLocked(groupID)
	r.accessGroups[groupID] = existing
	return existing, nil
}

func (r *MemoryRepository) DeleteAccessGroup(ctx context.Context, groupID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	groupID = strings.TrimSpace(groupID)
	if _, ok := r.accessGroups[groupID]; !ok {
		return errors.New("group not found")
	}
	delete(r.accessGroups, groupID)
	for key, member := range r.groupMembers {
		if member.GroupID == groupID {
			delete(r.groupMembers, key)
		}
	}
	for key, access := range r.workspaceGACL {
		if access.GroupID == groupID {
			delete(r.workspaceGACL, key)
		}
	}
	for key, access := range r.designGACL {
		if access.GroupID == groupID {
			delete(r.designGACL, key)
		}
	}
	return nil
}

func (r *MemoryRepository) ListAccessGroupMembers(ctx context.Context, groupID string) ([]domain.AccessGroupMember, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.accessGroups[strings.TrimSpace(groupID)]; !ok {
		return nil, errors.New("group not found")
	}
	members := []domain.AccessGroupMember{}
	for _, member := range r.groupMembers {
		if member.GroupID == groupID {
			members = append(members, member)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (r *MemoryRepository) ReplaceAccessGroupMembers(ctx context.Context, groupID string, userIDs []string) ([]domain.AccessGroupMember, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	groupID = strings.TrimSpace(groupID)
	if _, ok := r.accessGroups[groupID]; !ok {
		return nil, errors.New("group not found")
	}
	now := r.clock().UTC()
	next := map[string]domain.AccessGroupMember{}
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, err := r.getUserLocked(userID); err != nil {
			return nil, err
		}
		key := accessKey(groupID, userID)
		member := domain.AccessGroupMember{GroupID: groupID, UserID: userID, AddedAt: now}
		if existing, ok := r.groupMembers[key]; ok {
			member.AddedAt = existing.AddedAt
		}
		next[key] = member
	}
	for key, member := range r.groupMembers {
		if member.GroupID == groupID {
			delete(r.groupMembers, key)
		}
	}
	for key, member := range next {
		r.groupMembers[key] = member
	}
	members := make([]domain.AccessGroupMember, 0, len(next))
	for _, member := range next {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (r *MemoryRepository) ListUserAccessGroupIDs(ctx context.Context, userID string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	groupIDs := []string{}
	for _, member := range r.groupMembers {
		if member.UserID == strings.TrimSpace(userID) {
			groupIDs = append(groupIDs, member.GroupID)
		}
	}
	sort.Strings(groupIDs)
	return groupIDs, nil
}

func (r *MemoryRepository) ListWorkspaceAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.WorkspaceAccess{}
	for _, entry := range r.workspaceACL {
		if entry.WorkspaceID == workspaceID {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].UserID < access[j].UserID })
	return access, nil
}

func (r *MemoryRepository) GrantWorkspaceAccess(ctx context.Context, access domain.WorkspaceAccess) (domain.WorkspaceAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.WorkspaceAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.UserID = strings.TrimSpace(access.UserID)
	if access.WorkspaceID == "" || access.UserID == "" {
		return domain.WorkspaceAccess{}, errors.New("workspace id and user id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[access.WorkspaceID]; !ok {
		return domain.WorkspaceAccess{}, errors.New("workspace not found")
	}
	if _, err := r.getUserLocked(access.UserID); err != nil {
		return domain.WorkspaceAccess{}, err
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.UserID)
	if existing, ok := r.workspaceACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.workspaceACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeWorkspaceAccess(ctx context.Context, workspaceID string, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workspaceACL, accessKey(workspaceID, userID))
	return nil
}

func (r *MemoryRepository) ListWorkspaceGroupAccess(ctx context.Context, workspaceID string) ([]domain.WorkspaceGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.WorkspaceGroupAccess{}
	for _, entry := range r.workspaceGACL {
		if entry.WorkspaceID == strings.TrimSpace(workspaceID) {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].GroupID < access[j].GroupID })
	return access, nil
}

func (r *MemoryRepository) GrantWorkspaceGroupAccess(ctx context.Context, access domain.WorkspaceGroupAccess) (domain.WorkspaceGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.WorkspaceGroupAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.GroupID = strings.TrimSpace(access.GroupID)
	if access.WorkspaceID == "" || access.GroupID == "" {
		return domain.WorkspaceGroupAccess{}, errors.New("workspace id and group id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[access.WorkspaceID]; !ok {
		return domain.WorkspaceGroupAccess{}, errors.New("workspace not found")
	}
	if _, ok := r.accessGroups[access.GroupID]; !ok {
		return domain.WorkspaceGroupAccess{}, errors.New("group not found")
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.GroupID)
	if existing, ok := r.workspaceGACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.workspaceGACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeWorkspaceGroupAccess(ctx context.Context, workspaceID string, groupID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workspaceGACL, accessKey(workspaceID, groupID))
	return nil
}

func (r *MemoryRepository) ListDesignAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.DesignAccess{}
	for _, entry := range r.designACL {
		if entry.WorkspaceID == workspaceID && entry.DesignID == designID {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].UserID < access[j].UserID })
	return access, nil
}

func (r *MemoryRepository) GrantDesignAccess(ctx context.Context, access domain.DesignAccess) (domain.DesignAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.DesignID = strings.TrimSpace(access.DesignID)
	access.UserID = strings.TrimSpace(access.UserID)
	if access.WorkspaceID == "" || access.DesignID == "" || access.UserID == "" {
		return domain.DesignAccess{}, errors.New("workspace id, design id, and user id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	design, ok := r.designs[access.DesignID]
	if !ok || design.WorkspaceID != access.WorkspaceID {
		return domain.DesignAccess{}, errors.New("design not found")
	}
	if _, err := r.getUserLocked(access.UserID); err != nil {
		return domain.DesignAccess{}, err
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.DesignID, access.UserID)
	if existing, ok := r.designACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.designACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeDesignAccess(ctx context.Context, workspaceID string, designID string, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.designACL, accessKey(workspaceID, designID, userID))
	return nil
}

func (r *MemoryRepository) ListDesignGroupAccess(ctx context.Context, workspaceID string, designID string) ([]domain.DesignGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	access := []domain.DesignGroupAccess{}
	for _, entry := range r.designGACL {
		if entry.WorkspaceID == workspaceID && entry.DesignID == designID {
			access = append(access, entry)
		}
	}
	sort.Slice(access, func(i, j int) bool { return access[i].GroupID < access[j].GroupID })
	return access, nil
}

func (r *MemoryRepository) GrantDesignGroupAccess(ctx context.Context, access domain.DesignGroupAccess) (domain.DesignGroupAccess, error) {
	if err := ctx.Err(); err != nil {
		return domain.DesignGroupAccess{}, err
	}
	access.WorkspaceID = strings.TrimSpace(access.WorkspaceID)
	access.DesignID = strings.TrimSpace(access.DesignID)
	access.GroupID = strings.TrimSpace(access.GroupID)
	if access.WorkspaceID == "" || access.DesignID == "" || access.GroupID == "" {
		return domain.DesignGroupAccess{}, errors.New("workspace id, design id, and group id are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	design, ok := r.designs[access.DesignID]
	if !ok || design.WorkspaceID != access.WorkspaceID {
		return domain.DesignGroupAccess{}, errors.New("design not found")
	}
	if _, ok := r.accessGroups[access.GroupID]; !ok {
		return domain.DesignGroupAccess{}, errors.New("group not found")
	}
	now := r.clock().UTC()
	key := accessKey(access.WorkspaceID, access.DesignID, access.GroupID)
	if existing, ok := r.designGACL[key]; ok {
		access.CreatedAt = existing.CreatedAt
	} else {
		access.CreatedAt = now
	}
	access.UpdatedAt = now
	r.designGACL[key] = access
	return access, nil
}

func (r *MemoryRepository) RevokeDesignGroupAccess(ctx context.Context, workspaceID string, designID string, groupID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.designGACL, accessKey(workspaceID, designID, groupID))
	return nil
}
