package handler

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
)

func historyHandler(store repository.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var before *model.HistoryCursor
		if raw := c.Query("cursor"); raw != "" {
			if len(raw) > 512 {
				c.JSON(400, gin.H{"code": "INVALID_CURSOR", "message": "翻页位置无效，请刷新作品列表"})
				return
			}
			data, err := base64.RawURLEncoding.DecodeString(raw)
			var cursor model.HistoryCursor
			valid := len(raw) <= 512 && err == nil && json.Unmarshal(data, &cursor) == nil
			_, idErr := hex.DecodeString(cursor.ID)
			if !valid || cursor.CreatedAt.IsZero() || len(cursor.ID) != 32 || idErr != nil {
				c.JSON(400, gin.H{"code": "INVALID_CURSOR", "message": "翻页位置无效，请刷新作品列表"})
				return
			}
			before = &cursor
		}
		const pageSize = 20
		petID := c.Query("petId")
		var tasks []model.Task
		var err error
		if petID == "" {
			tasks, err = store.ListHistory(c.Request.Context(), c.GetString("userID"), before, pageSize+1)
		} else {
			if petID == "unassigned" {
				petID = ""
			} else {
				if _, err = store.GetPet(c.Request.Context(), petID, c.GetString("userID")); err != nil {
					respondError(c, err)
					return
				}
			}
			tasks, err = store.ListPetHistory(c.Request.Context(), c.GetString("userID"), petID, before, pageSize+1)
		}
		if err != nil {
			respondError(c, err)
			return
		}
		next := ""
		if len(tasks) > pageSize {
			tasks = tasks[:pageSize]
			last := tasks[len(tasks)-1]
			data, _ := json.Marshal(model.HistoryCursor{CreatedAt: last.CreatedAt, ID: last.ID})
			next = base64.RawURLEncoding.EncodeToString(data)
		}
		items := make([]model.TaskResponse, 0, len(tasks))
		for _, task := range tasks {
			items = append(items, task.Response())
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(200, gin.H{"items": items, "nextCursor": next})
	}
}
