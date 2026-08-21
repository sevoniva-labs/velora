package kratosapi

import "testing"

func TestUserRoleChangePayloadIsDeterministic(t *testing.T) {
	roles, payload, err := userRoleChangePayload([]string{" auditor ", "user", "auditor", ""})
	if err != nil {
		t.Fatalf("userRoleChangePayload() error = %v", err)
	}
	if len(roles) != 2 || roles[0] != "auditor" || roles[1] != "user" {
		t.Fatalf("userRoleChangePayload() roles = %#v", roles)
	}
	if payload != `{"roles":["auditor","user"]}` {
		t.Fatalf("userRoleChangePayload() payload = %s", payload)
	}
}

func TestRoleDataScopePayloadIsDeterministic(t *testing.T) {
	departments, payload, err := roleDataScopePayload("DEPARTMENT_AND_CHILDREN", []string{"dept-b", " dept-a ", "dept-b"})
	if err != nil {
		t.Fatalf("roleDataScopePayload() error = %v", err)
	}
	if len(departments) != 2 || departments[0] != "dept-a" || departments[1] != "dept-b" {
		t.Fatalf("roleDataScopePayload() departments = %#v", departments)
	}
	if payload != `{"data_scope":"DEPARTMENT_AND_CHILDREN","department_ids":["dept-a","dept-b"]}` {
		t.Fatalf("roleDataScopePayload() payload = %s", payload)
	}
}
