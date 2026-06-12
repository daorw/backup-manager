package handler

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// SystemHandler handles system-level HTTP requests.
type SystemHandler struct{}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

// Health handles GET /api/v1/health
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status":    "ok",
			"uptime":    time.Since(startTime).String(),
			"version":   "1.0.0",
			"go_version": runtime.Version(),
			"platform":  runtime.GOOS + "/" + runtime.GOARCH,
		},
	})
}
