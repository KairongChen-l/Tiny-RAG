// Package auth provides RBAC (Role-Based Access Control) with tenant isolation
// and document-level authorization for knowledge base access.
package auth

import (
	"context"
	"fmt"
)

// Role represents a user role in the system.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// Permission represents an action that can be performed on a resource.
type Permission string

const (
	PermRead   Permission = "read"
	PermWrite  Permission = "write"
	PermDelete Permission = "delete"
	PermAdmin  Permission = "admin"
)

// UserContext holds the authenticated user's identity and roles.
type UserContext struct {
	UserID   string
	TenantID string
	Roles    []Role
}

// HasRole checks whether the user has the specified role.
func (u *UserContext) HasRole(role Role) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// ResourceACL defines access control metadata for a resource.
type ResourceACL struct {
	ResourceID   string
	TenantID     string
	OwnerID      string
	IsPublic     bool
	AllowedRoles []Role
	AllowedUsers []string
}

// RBACChecker defines the interface for access control checks.
type RBACChecker interface {
	// CheckAccess determines whether the user has the given permission on the resource.
	CheckAccess(ctx context.Context, user UserContext, resource ResourceACL, permission Permission) (bool, error)
}

// DefaultRBACChecker implements RBACChecker with tenant isolation and
// document-level authorization.
type DefaultRBACChecker struct{}

// NewDefaultRBACChecker creates a new DefaultRBACChecker.
func NewDefaultRBACChecker() *DefaultRBACChecker {
	return &DefaultRBACChecker{}
}

// rolePermissions maps each role to its allowed permissions.
var rolePermissions = map[Role]map[Permission]bool{
	RoleAdmin: {
		PermRead:   true,
		PermWrite:  true,
		PermDelete: true,
		PermAdmin:  true,
	},
	RoleEditor: {
		PermRead:  true,
		PermWrite: true,
	},
	RoleViewer: {
		PermRead: true,
	},
}

// CheckAccess determines whether the user has the given permission on the resource.
// It enforces tenant isolation, role-based permissions, and document-level ACLs.
func (c *DefaultRBACChecker) CheckAccess(_ context.Context, user UserContext, resource ResourceACL, permission Permission) (bool, error) {
	if user.UserID == "" {
		return false, fmt.Errorf("user ID is required")
	}
	if user.TenantID == "" {
		return false, fmt.Errorf("tenant ID is required")
	}

	// Enforce tenant isolation: user and resource must belong to the same tenant.
	if resource.TenantID != "" && user.TenantID != resource.TenantID {
		return false, nil
	}

	// Admin role bypasses all further checks.
	if user.HasRole(RoleAdmin) {
		return true, nil
	}

	// Check if the user's roles grant the requested permission.
	if !hasRolePermission(user.Roles, permission) {
		return false, nil
	}

	// Public resources are readable by anyone in the same tenant.
	if resource.IsPublic && permission == PermRead {
		return true, nil
	}

	// Resource owner has full access.
	if resource.OwnerID != "" && resource.OwnerID == user.UserID {
		return true, nil
	}

	// Check document-level ACL: allowed users.
	for _, uid := range resource.AllowedUsers {
		if uid == user.UserID {
			return true, nil
		}
	}

	// Check document-level ACL: allowed roles.
	for _, allowedRole := range resource.AllowedRoles {
		if user.HasRole(allowedRole) {
			return true, nil
		}
	}

	return false, nil
}

// hasRolePermission checks whether any of the given roles grant the permission.
func hasRolePermission(roles []Role, perm Permission) bool {
	for _, role := range roles {
		if perms, ok := rolePermissions[role]; ok && perms[perm] {
			return true
		}
	}
	return false
}
