package handler

import (
	"net/http"

	"backup-manager/internal/entry"

	"github.com/gin-gonic/gin"
)

// EntryHandler 处理条目（被备份的文件/目录）相关请求。
type EntryHandler struct {
	svc *entry.Service
}

// NewEntryHandler 创建条目处理器。
func NewEntryHandler(svc *entry.Service) *EntryHandler {
	return &EntryHandler{svc: svc}
}

// List 处理 GET /api/v1/repos/:id/entries
func (h *EntryHandler) List(c *gin.Context) {
	views, err := h.svc.List(c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

// Get 处理 GET /api/v1/repos/:id/entries/:entryId
func (h *EntryHandler) Get(c *gin.Context) {
	view, err := h.svc.Get(c.Param("id"), c.Param("entryId"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// Adopt 处理 POST /api/v1/repos/:id/entries/adopt
// 创建条目：内容移入仓库，原位置替换为软链接（条目的 in 链接）。
func (h *EntryHandler) Adopt(c *gin.Context) {
	var req entry.AdoptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	view, err := h.svc.Adopt(c.Param("id"), &req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": view})
}

// Delete 处理 DELETE /api/v1/repos/:id/entries/:entryId?mode=&link_id=
func (h *EntryHandler) Delete(c *gin.Context) {
	mode := c.DefaultQuery("mode", entry.RemoveModeUnlink)
	if err := h.svc.Remove(c.Param("id"), c.Param("entryId"), mode, c.Query("link_id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "removed"})
}
