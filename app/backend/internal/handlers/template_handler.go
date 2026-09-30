package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TemplateSampleHandler struct {
	service *service.TemplateSampleService
}

// Constructor
func NewTemplateSampleHandler(service *service.TemplateSampleService) *TemplateSampleHandler {
	return &TemplateSampleHandler{service: service}
}
func (h *TemplateSampleHandler) GetTemplateSampleByID(c *gin.Context) {
	idParam := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Giao diện mẫu không hợp lệ"})
		return
	}

	sample, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy giao diện mẫu"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": sample,
	})
}

type CreateTemplateSampleRequest struct {
	Name        string `json:"name" binding:"required"`
	HTMLContent string `json:"html_content" binding:"required"`
}

func (h *TemplateSampleHandler) CreateTemplateSample(c *gin.Context) {
	var req CreateTemplateSampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Dữ liệu không hợp lệ",
			"details": err.Error(),
		})
		return
	}

	val, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}
	claims, ok := val.(*utils.CustomClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	sample := &models.TemplateSample{
		Name:         req.Name,
		HTMLContent:  req.HTMLContent,
		UniversityID: universityID, // gán từ token
	}

	id, err := h.service.Create(c.Request.Context(), sample)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Gán ID để trả về
	sample.ID = id

	c.JSON(http.StatusOK, gin.H{
		"message": "Đã tạo giao diện mẫu",
		"data":    sample,
	})
}

type UpdateTemplateSampleRequest struct {
	Name        string `json:"name" binding:"required"`
	HTMLContent string `json:"html_content" binding:"required"`
}

// Handler
func (h *TemplateSampleHandler) UpdateTemplateSample(c *gin.Context) {
	idParam := c.Param("id")
	id, err := primitive.ObjectIDFromHex(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Giao diện mẫu không hợp lệ"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		HTMLContent string `json:"html_content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ", "details": err.Error()})
		return
	}

	sample := &models.TemplateSample{
		ID:          id,
		Name:        req.Name,
		HTMLContent: req.HTMLContent,
	}

	if err := h.service.Update(c.Request.Context(), sample); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Đã cập nhật giao diện mẫu",
	})
}

func (h *TemplateSampleHandler) GetAllTemplateSamples(c *gin.Context) {
	val, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}
	claims, ok := val.(*utils.CustomClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	samples, err := h.service.GetAllVisible(c.Request.Context(), universityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, s := range samples {
		result = append(result, gin.H{
			"id":   s.ID.Hex(),
			"name": s.Name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Thành công",
		"data":    result,
	})
}

func (h *TemplateSampleHandler) GetTemplateSampleView(c *gin.Context) {
	ctx := c.Request.Context()
	templateSampleIDStr := c.Param("id")

	templateSampleID, err := primitive.ObjectIDFromHex(templateSampleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Giao diện mẫu không hợp lệ"})
		return
	}
	templateSample, err := h.service.GetByID(ctx, templateSampleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy giao diện mẫu"})
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(templateSample.HTMLContent))
}
