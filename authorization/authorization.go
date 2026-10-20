// Package authorization evaluates application-supplied, trusted principal data.
package authorization

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"slices"
)

var ErrForbidden = errors.New("authorization: forbidden")
var ErrUnauthenticated = errors.New("authorization: unauthenticated")

type Principal struct {
	Roles       []string
	Permissions []string
}

// Resolver may retrieve fresh permissions from an application database. It must
// return ErrUnauthenticated when no trusted authenticated principal exists.
type Resolver func(*gin.Context) (Principal, error)
type Policy func(context.Context, Principal) (bool, error)
type Authorizer struct{ resolve Resolver }

func New(resolve Resolver) *Authorizer {
	if resolve == nil {
		panic("authorization: nil resolver")
	}
	return &Authorizer{resolve}
}
func (a *Authorizer) RequireRole(role string) gin.HandlerFunc { return a.RequireAnyRole(role) }
func (a *Authorizer) RequireAnyRole(roles ...string) gin.HandlerFunc {
	roles = slices.Clone(roles)
	return a.Require(func(_ context.Context, p Principal) (bool, error) {
		for _, r := range roles {
			if r != "" && slices.Contains(p.Roles, r) {
				return true, nil
			}
		}
		return false, nil
	})
}
func (a *Authorizer) RequirePermission(permission string) gin.HandlerFunc {
	return a.Require(func(_ context.Context, p Principal) (bool, error) {
		return permission != "" && slices.Contains(p.Permissions, permission), nil
	})
}
func (a *Authorizer) Require(policy Policy) gin.HandlerFunc {
	if policy == nil {
		panic("authorization: nil policy")
	}
	return func(c *gin.Context) {
		p, e := a.resolve(c)
		if e != nil {
			status := 503
			if errors.Is(e, ErrUnauthenticated) {
				status = 401
			} else if errors.Is(e, ErrForbidden) {
				status = 403
			}
			c.AbortWithStatus(status)
			return
		}
		ok, e := policy(c.Request.Context(), p)
		if e != nil {
			c.AbortWithStatus(503)
			return
		}
		if !ok {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	}
}
