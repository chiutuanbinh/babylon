package casbin_test

import (
	"context"
	"testing"

	"babylon/internal/adapters/casbin"
	"babylon/internal/authz"
	"babylon/internal/identity"
)

var (
	alice = authz.Subject{ID: "1001", Role: "user"}
	bob   = authz.Subject{ID: "1002", Role: "user"}
	root  = authz.Subject{ID: "1", Role: "admin"}
	anon  = authz.Subject{}

	doc = authz.Resource{Type: "doc", OwnerID: "1001"} // owned by alice
)

func TestCan(t *testing.T) {
	a, err := casbin.New()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		sub  authz.Subject
		act  authz.Action
		res  authz.Resource
		want bool
	}{
		{"owner reads", alice, authz.Read, doc, true},
		{"owner updates", alice, authz.Update, doc, true},
		{"owner deletes", alice, authz.Delete, doc, true},
		{"other user reads", bob, authz.Read, doc, false},
		{"other user updates", bob, authz.Update, doc, false},
		{"other user deletes", bob, authz.Delete, doc, false},
		{"admin reads", root, authz.Read, doc, true},
		{"admin updates", root, authz.Update, doc, true},
		{"admin deletes", root, authz.Delete, doc, true},
		{"anonymous reads", anon, authz.Read, doc, false},
		{"anonymous reads ownerless resource", anon, authz.Read, authz.Resource{Type: "doc"}, false},
		{"empty ID with admin role denied", authz.Subject{Role: "admin"}, authz.Read, doc, false},
		{"non-admin on ownerless resource denied", bob, authz.Read, authz.Resource{Type: "doc"}, false},
		{"unknown action denied for owner", alice, authz.Action("create"), doc, false},
		{"anonymous updates", anon, authz.Update, doc, false},
		{"anonymous deletes", anon, authz.Delete, doc, false},
		{"unknown action denied even for admin", root, authz.Action("create"), doc, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := a.Can(context.Background(), tc.sub, tc.act, tc.res)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Can(%+v, %q, %+v) = %v, want %v", tc.sub, tc.act, tc.res, got, tc.want)
			}
		})
	}
}

// The identity context and the authz kernel each spell the admin role; they
// must agree or admins silently lose access.
func TestRoleNamesAgree(t *testing.T) {
	if string(identity.RoleAdmin) != authz.RoleAdmin {
		t.Fatalf("identity.RoleAdmin = %q, authz.RoleAdmin = %q", identity.RoleAdmin, authz.RoleAdmin)
	}
}
