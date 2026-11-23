package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/model"
	"github.com/jason127vip-dot/maohai-shuo/backend/internal/repository"
)

func readPetJSON(c *gin.Context, out any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil || decoder.Decode(new(any)) != io.EOF {
		c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "资料格式不正确，请检查后重试"})
		return false
	}
	return true
}
func cleanText(value string, limit int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func registerPets(api *gin.RouterGroup, store repository.Store) {
	api.GET("/pets", func(c *gin.Context) {
		items, err := store.ListPets(c.Request.Context(), c.GetString("userID"))
		if err != nil {
			respondError(c, err)
			return
		}
		for i, p := range items {
			items[i] = p.At(time.Now())
		}
		c.Header("Cache-Control", "private, no-store")
		c.JSON(200, gin.H{"items": items})
	})
	save := func(create bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			var input struct {
				Name        string `json:"name"`
				Species     string `json:"species"`
				Avatar      string `json:"avatar"`
				BirthDate   string `json:"birthDate"`
				Approximate bool   `json:"approximate"`
				IsDefault   bool   `json:"isDefault"`
			}
			if !readPetJSON(c, &input) {
				return
			}
			input.Name = strings.TrimSpace(input.Name)
			if input.Name == "" || !cleanText(input.Name, 20) || (input.Species != "" && input.Species != "cat" && input.Species != "dog") || (input.Avatar != "paw" && input.Avatar != "cat" && input.Avatar != "dog") {
				c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "名字需为1至20个字，请选择有效的宠物类型和头像"})
				return
			}
			if input.BirthDate != "" {
				birth, err := time.Parse("2006-01-02", input.BirthDate)
				if err != nil || input.BirthDate < "1900-01-01" || birth.After(time.Now().UTC()) {
					c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "生日需为1900年以后且不晚于今天的有效日期"})
					return
				}
			} else if input.Approximate {
				c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "请填写估计年龄或生日"})
				return
			}
			id := c.Param("petId")
			if create {
				var err error
				id, err = model.NewID()
				if err != nil {
					respondError(c, err)
					return
				}
			}
			pet, err := store.SavePet(c.Request.Context(), model.Pet{ID: id, UserID: c.GetString("userID"), Name: input.Name, Species: input.Species, Avatar: input.Avatar, BirthDate: input.BirthDate, Approximate: input.Approximate, IsDefault: input.IsDefault}, create)
			if err != nil {
				respondError(c, err)
				return
			}
			status := 200
			if create {
				status = 201
			}
			c.Header("Cache-Control", "no-store")
			c.JSON(status, pet.At(time.Now()))
		}
	}
	api.POST("/pets", save(true))
	api.PATCH("/pets/:petId", save(false))
	api.PATCH("/generations/:taskId/note", func(c *gin.Context) {
		var input struct {
			Note string `json:"note"`
		}
		if !readPetJSON(c, &input) {
			return
		}
		input.Note = strings.TrimSpace(strings.ReplaceAll(input.Note, "\r\n", "\n"))
		if utf8.RuneCountInString(input.Note) > 200 || !cleanText(strings.ReplaceAll(input.Note, "\n", ""), 200) {
			c.JSON(400, gin.H{"code": "INVALID_INPUT", "message": "记录文字最多200个字，请勿包含控制字符"})
			return
		}
		task, err := store.SaveNote(c.Request.Context(), c.GetString("userID"), c.Param("taskId"), input.Note)
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, task.Response())
	})
	api.PATCH("/generations/:taskId/pet", func(c *gin.Context) {
		var input struct {
			PetID string `json:"petId"`
		}
		if !readPetJSON(c, &input) {
			return
		}
		task, err := store.AssignPet(c.Request.Context(), c.GetString("userID"), c.Param("taskId"), input.PetID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, task.Response())
	})
}
