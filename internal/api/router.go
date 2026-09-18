package api

import (
	"bytes"
	"io/fs"
	"net/http"
	"strings"

	"backup-manager/internal/api/handler"

	"github.com/gin-gonic/gin"
)

// SetupRouter 注册全部 API 路由。
func SetupRouter(
	repoHandler *handler.RepoHandler,
	entryHandler *handler.EntryHandler,
	linkHandler *handler.LinkHandler,
	deviceHandler *handler.DeviceHandler,
	browseHandler *handler.BrowseHandler,
	contentHandler *handler.ContentHandler,
	backupHandler *handler.BackupHandler,
	authHandler *handler.AuthHandler,
	systemHandler *handler.SystemHandler,
	rollbackHandler *handler.RollbackHandler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(CORSMiddleware(DefaultCORSConfig()))
	r.Use(ErrorRecoveryMiddleware())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", systemHandler.Health)

		// 仓库
		v1.POST("/repos", repoHandler.Create)
		v1.GET("/repos", repoHandler.List)
		v1.GET("/repos/:id", repoHandler.Get)
		v1.DELETE("/repos/:id", repoHandler.Delete)
		v1.PUT("/repos/:id/config", repoHandler.UpdateConfig)
		v1.POST("/repos/:id/git-init", repoHandler.GitInit)

		// 设备
		v1.GET("/devices/current", deviceHandler.Current)
		v1.GET("/repos/:id/devices", deviceHandler.List)
		v1.POST("/repos/:id/devices", deviceHandler.Register)
		v1.PATCH("/repos/:id/devices/:fingerprint", deviceHandler.Rename)
		v1.DELETE("/repos/:id/devices/:fingerprint", deviceHandler.Delete)
		v1.POST("/repos/:id/devices/:fingerprint/apply", deviceHandler.Apply)

		// 条目
		v1.GET("/repos/:id/entries", entryHandler.List)
		v1.POST("/repos/:id/entries/adopt", entryHandler.Adopt)
		v1.GET("/repos/:id/entries/:entryId", entryHandler.Get)
		v1.POST("/repos/:id/entries/:entryId/switch", entryHandler.Switch)
		v1.DELETE("/repos/:id/entries/:entryId", entryHandler.Delete)

		// 链接
		v1.POST("/repos/:id/links/bulk", linkHandler.Bulk)
		v1.POST("/repos/:id/entries/:entryId/links", linkHandler.Create)
		v1.POST("/repos/:id/entries/:entryId/links/:linkId/repair", linkHandler.Repair)
		v1.POST("/repos/:id/entries/:entryId/links/:linkId/remove", linkHandler.Remove)

		// 本机文件浏览
		v1.GET("/browse", browseHandler.Browse)
		v1.GET("/browse/allowed-roots", browseHandler.AllowedRoots)

		// 仓库内容
		v1.GET("/repos/:id/tree", contentHandler.Tree)
		v1.GET("/repos/:id/preview", contentHandler.Preview)
		v1.PUT("/repos/:id/save", contentHandler.Save)
		v1.GET("/repos/:id/changes", contentHandler.Changes)

		// 备份
		v1.POST("/repos/:id/backup", backupHandler.Trigger)
		v1.GET("/repos/:id/backup/history", backupHandler.History)
		v1.POST("/repos/:id/push", backupHandler.Push)

		// Git 认证
		v1.GET("/repos/:id/auth", authHandler.Get)
		v1.PUT("/repos/:id/auth", authHandler.Set)
		v1.DELETE("/repos/:id/auth", authHandler.Clear)

		// 回滚
		v1.GET("/repos/:id/commits/:hash/changed-files", rollbackHandler.ListFiles)
		v1.GET("/repos/:id/commits/:hash/files", rollbackHandler.GetCommitFile)
		v1.POST("/repos/:id/commits/:hash/restore", rollbackHandler.RestoreFile)
		v1.POST("/repos/:id/rollback", rollbackHandler.Rollback)
	}

	return r
}

// MountStatic 挂载前端静态资源，并为 SPA 提供 index.html 回退。
func MountStatic(r *gin.Engine, frontendFS fs.FS) {
	staticFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		staticFS = frontendFS
	}

	fileServer := http.FileServer(http.FS(staticFS))

	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || c.Request.Method != "GET" {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if f, err := staticFS.Open(cleanPath); err == nil {
			f.Close()
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		// 未命中静态文件 → 返回 index.html 交给前端路由
		indexFile, err := staticFS.Open("index.html")
		if err != nil {
			c.Next()
			return
		}
		defer indexFile.Close()

		stat, _ := indexFile.Stat()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(indexFile); err != nil {
			c.Next()
			return
		}
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), bytes.NewReader(buf.Bytes()))
		c.Abort()
	})
}
