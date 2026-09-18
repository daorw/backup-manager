package handler

import (
	"net/http"

	"backup-manager/internal/entry"

	"github.com/gin-gonic/gin"
)

// LinkHandler 处理链接相关请求。
type LinkHandler struct {
	svc *entry.Service
}

// NewLinkHandler 创建链接处理器。
func NewLinkHandler(svc *entry.Service) *LinkHandler {
	return &LinkHandler{svc: svc}
}

// Create 处理 POST /api/v1/repos/:id/entries/:entryId/links
// 为条目添加一条 out 链接（不复制内容）。
func (h *LinkHandler) Create(c *gin.Context) {
	var req entry.AddLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	view, err := h.svc.AddLink(c.Param("id"), c.Param("entryId"), &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": view})
}

// Bulk 处理 POST /api/v1/repos/:id/links/bulk
// 把一个本地根目录下的多个条目批量链接到本机。
func (h *LinkHandler) Bulk(c *gin.Context) {
	var req entry.BulkLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	views, err := h.svc.BulkLink(c.Param("id"), &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": views})
}

// Repair 处理 POST /api/v1/repos/:id/entries/:entryId/links/:linkId/repair
func (h *LinkHandler) Repair(c *gin.Context) {
	view, err := h.svc.RepairLink(c.Param("id"), c.Param("entryId"), c.Param("linkId"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// Readopt 处理 POST /api/v1/repos/:id/entries/:entryId/links/:linkId/readopt
// replaced 状态：把本机的真实文件/目录移入仓库并恢复软链接。
func (h *LinkHandler) Readopt(c *gin.Context) {
	view, err := h.svc.Readopt(c.Param("id"), c.Param("entryId"), c.Param("linkId"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// Remove 处理 POST /api/v1/repos/:id/entries/:entryId/links/:linkId/remove
func (h *LinkHandler) Remove(c *gin.Context) {
	view, err := h.svc.RemoveLink(c.Param("id"), c.Param("entryId"), c.Param("linkId"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}
