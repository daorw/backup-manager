package api

import (
	"backup-manager/internal/api/handler"
	"bytes"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SetupRouter configures the Gin router with all API routes.
func SetupRouter(
	repoHandler *handler.RepoHandler,
	symlinkHandler *handler.SymlinkHandler,
	browseHandler *handler.BrowseHandler,
	previewHandler *handler.PreviewHandler,
	backupHandler *handler.BackupHandler,
	authHandler *handler.AuthHandler,
	systemHandler *handler.SystemHandler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// Global middleware
	r.Use(CORSMiddleware(DefaultCORSConfig()))
	r.Use(ErrorRecoveryMiddleware())

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Health check
		v1.GET("/health", systemHandler.Health)

		// Repo CRUD
		v1.POST("/repos", repoHandler.Create)
		v1.GET("/repos", repoHandler.List)
		v1.GET("/repos/:id", repoHandler.Get)
		v1.DELETE("/repos/:id", repoHandler.Delete)
		v1.PUT("/repos/:id/config", repoHandler.UpdateConfig)

		// Symlink CRUD
		v1.POST("/repos/:id/symlinks", symlinkHandler.Create)
		v1.GET("/repos/:id/symlinks", symlinkHandler.List)
		v1.GET("/repos/:id/symlinks/:linkId", symlinkHandler.Get)
		v1.DELETE("/repos/:id/symlinks/:linkId", symlinkHandler.Delete)
		v1.PUT("/repos/:id/symlinks/:linkId", symlinkHandler.UpdateTarget)
		v1.POST("/repos/:id/symlinks/batch", symlinkHandler.BatchImport)

		// File browsing
		v1.GET("/browse", browseHandler.Browse)

		// File preview
		v1.GET("/repos/:id/preview", previewHandler.Preview)

		// Backup
		v1.POST("/repos/:id/backup", backupHandler.Trigger)
		v1.GET("/repos/:id/backup/history", backupHandler.History)

		// Git auth
		v1.GET("/repos/:id/auth", authHandler.Get)
		v1.PUT("/repos/:id/auth", authHandler.Set)
		v1.DELETE("/repos/:id/auth", authHandler.Clear)
	}

	return r
}

// MountStatic mounts the frontend static files from an embed.FS onto the router.
// It serves static assets from the embedded filesystem and provides SPA fallback
// routing (all non-API GET requests return index.html).
func MountStatic(r *gin.Engine, frontendFS fs.FS) {
	// Try to use the subdirectory
	staticFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		// Maybe the FS is already the dist directory
		staticFS = frontendFS
	}

	fileServer := http.FileServer(http.FS(staticFS))

	// Handle all GET requests that are not API calls
	r.Use(func(c *gin.Context) {
		// Skip API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}

		// Only handle GET requests for static files
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if the file exists in the embedded filesystem
		cleanPath := strings.TrimPrefix(path, "/")
		f, err := staticFS.Open(cleanPath)
		if err == nil {
			f.Close()
			// File exists, serve it
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		// File doesn't exist, serve index.html for SPA routing
		indexFile, err := staticFS.Open("index.html")
		if err != nil {
			c.Next()
			return
		}
		defer indexFile.Close()

		// Read index.html into memory for ServeContent
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
