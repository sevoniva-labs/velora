// Package migrations 嵌入数据库迁移 SQL 文件（embed 不允许跨目录 pattern，
// 故由本包统一持有，供 db 包执行）。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
