package handler

import (
	"net/http"
	"runtime"
	"time"

	"backup-manager/internal/appconfig"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// SystemHandler handles system-level HTTP requests.
type SystemHandler struct {
	config *appconfig.Manager
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(config *appconfig.Manager) *SystemHandler {
	return &SystemHandler{config: config}
}

// Health handles GET /api/v1/health.
func (h *SystemHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status":     "ok",
			"uptime":     time.Since(startTime).String(),
			"version":    "1.0.0",
			"go_version": runtime.Version(),
			"platform":   runtime.GOOS + "/" + runtime.GOARCH,
		},
	})
}

// Settings handles GET /api/v1/settings.
func (h *SystemHandler) Settings(c *gin.Context) {
	config := h.config.Get()
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"language": config.Language,
		},
	})
}

type updateSettingsRequest struct {
	Language string `json:"language" binding:"required"`
}

// UpdateSettings handles PUT /api/v1/settings.
func (h *SystemHandler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	config, err := h.config.SetLanguage(req.Language)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"language": config.Language,
		},
	})
}
