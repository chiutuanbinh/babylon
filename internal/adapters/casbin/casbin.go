// Package casbin implements authz.Authorizer with a Casbin ABAC model:
// a subject may act on a resource if they own it or are an admin.
package casbin

import (
	"context"
	"fmt"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"babylon/internal/authz"
)

var modelText = fmt.Sprintf(`
[request_definition]
r = sub, act, obj

[policy_definition]
p = act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.act == p.act && r.sub.ID != "" && (r.sub.Role == %q || r.sub.ID == r.obj.OwnerID)
`, authz.RoleAdmin)

var _ authz.Authorizer = (*Authorizer)(nil)

// Authorizer enforces the owner-or-admin policy with an in-memory Casbin enforcer.
type Authorizer struct {
	e *casbinlib.Enforcer
}

// New builds an Authorizer with the model and the read/update/delete policy rows.
func New() (*Authorizer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("casbin model: %w", err)
	}
	e, err := casbinlib.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("casbin enforcer: %w", err)
	}
	for _, act := range []authz.Action{authz.Read, authz.Update, authz.Delete} {
		if _, err := e.AddPolicy(string(act)); err != nil {
			return nil, fmt.Errorf("casbin policy %q: %w", act, err)
		}
	}
	return &Authorizer{e: e}, nil
}

// Can reports whether sub may perform act on res. Anonymous subjects (empty ID)
// and unknown actions are always denied.
func (a *Authorizer) Can(_ context.Context, sub authz.Subject, act authz.Action, res authz.Resource) (bool, error) {
	// act must be passed as a plain string: Casbin's expression engine compares
	// with reflect.DeepEqual, so authz.Action("read") never equals the policy's
	// "read" and every check would silently come back false.
	return a.e.Enforce(sub, string(act), res)
}
