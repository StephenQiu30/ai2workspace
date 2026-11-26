// Package domain defines workspace membership for the creation module.
package domain

import "errors"

// ErrMembershipNotFound indicates the user has no active role in the workspace.
var ErrMembershipNotFound = errors.New("active workspace membership not found")

// Role is a user's role in a production workspace.
type Role string
