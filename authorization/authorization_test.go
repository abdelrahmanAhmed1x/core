package authorization

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestPolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name string
		p    Principal
		err  error
		kind string
		want int
	}{
		{"missing", Principal{}, ErrUnauthenticated, "role", 401}, {"empty", Principal{}, nil, "empty", 403}, {"wrong", Principal{Roles: []string{"user"}}, nil, "role", 403}, {"role", Principal{Roles: []string{"admin"}}, nil, "role", 204}, {"permission", Principal{Permissions: []string{"write"}}, nil, "permission", 204}, {"no permission", Principal{}, nil, "permission", 403}, {"resolver failure", Principal{}, errors.New("database"), "role", 503}, {"policy failure", Principal{}, nil, "error", 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := New(func(*gin.Context) (Principal, error) { return tc.p, tc.err })
			var m gin.HandlerFunc
			switch tc.kind {
			case "role":
				m = a.RequireRole("admin")
			case "empty":
				m = a.RequireAnyRole()
			case "permission":
				m = a.RequirePermission("write")
			case "error":
				m = a.Require(func(context.Context, Principal) (bool, error) { return true, errors.New("failure") })
			}
			r := gin.New()
			r.GET("/", m, func(c *gin.Context) { c.Status(204) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			if w.Code != tc.want {
				t.Fatal(w.Code)
			}
		})
	}
}
func BenchmarkRole(b *testing.B) {
	gin.SetMode(gin.TestMode)
	a := New(func(*gin.Context) (Principal, error) { return Principal{Roles: []string{"admin"}}, nil })
	m := a.RequireRole("admin")
	b.ReportAllocs()
	for b.Loop() {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		m(c)
	}
}
