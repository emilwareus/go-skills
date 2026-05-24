// role_enum.go shows a struct-backed enum for business roles.
package examples

import "errors"

type Role struct {
	slug string
}

func (r Role) String() string {
	return r.slug
}

func (r Role) IsZero() bool {
	return r == Role{}
}

var (
	Unknown   = Role{""}
	Guest     = Role{"guest"}
	Member    = Role{"member"}
	Moderator = Role{"moderator"}
	Admin     = Role{"admin"}
)

var ErrUnknownRole = errors.New("unknown role")

func RoleFromString(s string) (Role, error) {
	switch s {
	case Guest.slug:
		return Guest, nil
	case Member.slug:
		return Member, nil
	case Moderator.slug:
		return Moderator, nil
	case Admin.slug:
		return Admin, nil
	default:
		return Unknown, ErrUnknownRole
	}
}
