package model

import (
	"time"

	"github.com/compozy/compozy/engine/core"
)

// Role represents user access level
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User represents a system user
type User struct {
	ID        core.ID   `db:"id,pk"`
	Email     string    `db:"email,unique"`
	Role      Role      `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}

// Valid checks if the role is a valid value
func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleUser
}
