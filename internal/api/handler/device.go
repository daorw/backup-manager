package handler

import (
	"net/http"

	"backup-manager/internal/entry"

	"github.com/gin-gonic/gin"
)

// DeviceHandler 处理设备相关请求。
type DeviceHandler struct {
	svc *entry.Service
}

// NewDeviceHandler 创建设备处理器。
func NewDeviceHandler(svc *entry.Service) *DeviceHandler {
	return &DeviceHandler{svc: svc}
}

// Current 处理 GET /api/v1/devices/current
// 返回当前机器指纹（由本机派生，无需配置）。
func (h *DeviceHandler) Current(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.svc.CurrentDeviceInfo()})
}

// List 处理 GET /api/v1/repos/:id/devices
func (h *DeviceHandler) List(c *gin.Context) {
	views, err := h.svc.ListDevices(c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

// Register 处理 POST /api/v1/repos/:id/devices
func (h *DeviceHandler) Register(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	view, err := h.svc.RegisterDevice(c.Param("id"), req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// Rename 处理 PATCH /api/v1/repos/:id/devices/:fingerprint
func (h *DeviceHandler) Rename(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if err := h.svc.RenameDevice(c.Param("id"), c.Param("fingerprint"), req.Name); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "renamed"})
}

// Delete 处理 DELETE /api/v1/repos/:id/devices/:fingerprint
func (h *DeviceHandler) Delete(c *gin.Context) {
	if err := h.svc.DeleteDevice(c.Param("id"), c.Param("fingerprint")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "deleted"})
}

// Apply 处理 POST /api/v1/repos/:id/devices/:fingerprint/apply
// 让本机与清单收敛：按需创建/修复本机软链接，从不覆盖已占用路径。
func (h *DeviceHandler) Apply(c *gin.Context) {
	var req struct {
		DryRun bool `json:"dry_run"`
	}
	_ = c.ShouldBindJSON(&req)
	result, err := h.svc.Apply(c.Param("id"), c.Param("fingerprint"), req.DryRun)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
