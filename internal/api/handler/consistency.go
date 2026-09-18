package handler

import (
	"net/http"

	"backup-manager/internal/entry"

	"github.com/gin-gonic/gin"
)

// ConsistencyHandler 处理一致性巡检请求。
type ConsistencyHandler struct {
	svc *entry.Service
}

// NewConsistencyHandler 创建巡检处理器。
func NewConsistencyHandler(svc *entry.Service) *ConsistencyHandler {
	return &ConsistencyHandler{svc: svc}
}

// Audit 处理 GET /api/v1/repos/:id/consistency
// 返回清单不变量与文件系统实际状态的全部结论。
func (h *ConsistencyHandler) Audit(c *gin.Context) {
	res, err := h.svc.Audit(c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// Repair 处理 POST /api/v1/repos/:id/consistency/repair
// 收敛所有可自动修复的问题，其余作为未修复项返回。
func (h *ConsistencyHandler) Repair(c *gin.Context) {
	res, err := h.svc.RepairAll(c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}
