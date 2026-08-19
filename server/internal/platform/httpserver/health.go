package httpserver

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

func newRequestID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// RegisterHealth 注册 /healthz /readyz /metrics。
func RegisterHealth(r *gin.Engine, db *gorm.DB) {
	metrics.RegisterDefault()

	r.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		if db == nil {
			response.ErrorWith(c, 503, "A05001", "数据库未初始化")
			return
		}
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			response.ErrorWith(c, 503, "A05001", "依赖不可用: postgres")
			return
		}
		response.OK(c, gin.H{"status": "ready", "dependencies": gin.H{"postgres": "up"}})
	})
	r.GET("/metrics", metricsHandler(db))
}

// metricsHandler 输出 Prometheus 文本格式指标（含 HTTP 请求统计 / 业务指标 / DB 池 / 运行时）。
func metricsHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var dbStats func() (open, inUse, idle int64, ok bool)
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				dbStats = func() (int64, int64, int64, bool) {
					st := sqlDB.Stats()
					return int64(st.OpenConnections), int64(st.InUse), int64(st.Idle), true
				}
			}
		}
		body := metrics.Render(dbStats)
		c.Data(200, "text/plain; version=0.0.4; charset=utf-8", body)
	}
}
