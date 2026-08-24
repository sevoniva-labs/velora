package portal

import "time"

type DirectoryCredential struct {
	ApplicationID   string
	OrganizationID  string
	ApplicationCode string
	LifecycleStatus string
	SecretRef       string
}

type DirectoryOrganization struct {
	ID        string
	Key       string
	Name      string
	Status    string
	UpdatedAt time.Time
}

type DirectoryDepartment struct {
	ID        string
	ParentID  string
	Key       string
	Name      string
	Status    string
	SortOrder int64
	UpdatedAt time.Time
}

type DirectoryUser struct {
	Subject      string
	LoginName    string
	DisplayName  string
	Email        string
	DepartmentID string
	Status       string
	Roles        []string
	Version      int64
	UpdatedAt    time.Time
	CursorID     string
}
