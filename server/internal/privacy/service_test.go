package privacy

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestExportService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, ddl := range []string{
		`CREATE TABLE application_favorites (user_id TEXT, application_id INTEGER, created_at DATETIME, PRIMARY KEY(user_id, application_id))`,
		`CREATE TABLE application_visits (user_id TEXT, application_id INTEGER, visit_count INTEGER, last_visited_at DATETIME, PRIMARY KEY(user_id, application_id))`,
		`CREATE TABLE todos (id INTEGER PRIMARY KEY, user_id TEXT, title TEXT, kind TEXT, source_system TEXT, source_id TEXT, priority TEXT, status TEXT, due_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE mail_messages (id INTEGER PRIMARY KEY, user_id TEXT, folder TEXT, subject TEXT, from_address TEXT, from_name TEXT, to_addresses TEXT, received_at DATETIME, is_read INTEGER, is_starred INTEGER, has_attachment INTEGER)`,
		`CREATE TABLE audit_logs (id INTEGER PRIMARY KEY, operator TEXT, action TEXT, resource TEXT, resource_id TEXT, ip TEXT, detail TEXT, created_at DATETIME)`,
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
	return NewService(db), db
}

func TestExportUserAggregates(t *testing.T) {
	svc, db := newTestExportService(t)
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, db.Exec(`INSERT INTO application_favorites VALUES (?,?,?)`, "u-1", 1, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO application_favorites VALUES (?,?,?)`, "u-1", 2, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO application_visits VALUES (?,?,?,?)`, "u-1", 1, 5, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO todos (id,user_id,title,kind,source_system,source_id,priority,status,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		10, "u-1", "审批单", "approval", "itil", "T-1", "high", "OPEN", now).Error)
	require.NoError(t, db.Exec(`INSERT INTO mail_messages (id,user_id,folder,subject,from_address,is_read,is_starred,has_attachment,received_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		100, "u-1", "INBOX", "周报", "boss@corp.com", 0, 1, 0, now).Error)
	require.NoError(t, db.Exec(`INSERT INTO audit_logs (id,operator,action,resource,resource_id,ip,detail,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		5, "u-1", "LOGIN", "session", "", "10.0.0.1", "登录", now).Error)
	// 其他用户数据不应混入
	require.NoError(t, db.Exec(`INSERT INTO application_favorites VALUES (?,?,?)`, "u-2", 9, now).Error)

	out, err := svc.ExportUser(ctx, "u-1")
	require.NoError(t, err)
	assert.Equal(t, "u-1", out.UserID)
	assert.Len(t, out.Favorites, 2)
	assert.Len(t, out.Visits, 1)
	assert.Equal(t, int64(5), out.Visits[0].Count)
	assert.Len(t, out.Todos, 1)
	assert.Equal(t, "审批单", out.Todos[0].Title)
	assert.Len(t, out.MailMeta, 1)
	assert.Equal(t, "周报", out.MailMeta[0].Subject)
	assert.Len(t, out.AuditLogs, 1)
	assert.Equal(t, "LOGIN", out.AuditLogs[0].Action)
}

func TestExportUserEmpty(t *testing.T) {
	svc, _ := newTestExportService(t)
	out, err := svc.ExportUser(context.Background(), "no-such-user")
	require.NoError(t, err)
	assert.Empty(t, out.Favorites)
	assert.Empty(t, out.Todos)
	assert.Empty(t, out.AuditLogs)
}

func TestExportUserEmptyID(t *testing.T) {
	svc, _ := newTestExportService(t)
	_, err := svc.ExportUser(context.Background(), "")
	assert.Error(t, err)
}
