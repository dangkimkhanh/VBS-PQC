package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/database"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TemplateHandler struct {
	templateService service.TemplateService
	minioClient     *database.MinioClient
	facultyService  service.FacultyService
}

func NewTemplateHandler(
	templateService service.TemplateService,
	minioClient *database.MinioClient,
	facultyService service.FacultyService,
) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
		minioClient:     minioClient,
		facultyService:  facultyService,
	}
}

func (h *TemplateHandler) GetTemplateByID(c *gin.Context) {
	id := c.Param("id")

	template, err := h.templateService.GetTemplateByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không tìm thấy mẫu bằng"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": template,
	})
}

func (h *TemplateHandler) GetTemplates(c *gin.Context) {
	// Lấy faculty_id từ query param (optional)
	facultyIDStr := c.Query("faculty_id")

	// Lấy thông tin claims từ token
	claimsRaw, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	claims, ok := claimsRaw.(*utils.CustomClaims)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	// Chuyển universityID từ token sang ObjectID
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	var templates []*models.DiplomaTemplate

	// Nếu có faculty_id -> lấy template theo khoa
	if facultyIDStr != "" {
		facultyID, err := primitive.ObjectIDFromHex(facultyIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Chuyên ngành không hợp lệ"})
			return
		}

		templates, err = h.templateService.GetTemplatesByFaculty(c.Request.Context(), universityID, facultyID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		templates, err = h.templateService.GetTemplatesByUniversity(c.Request.Context(), universityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": templates,
	})
}

func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	var req struct {
		FacultyID        string `json:"faculty_id" binding:"required"`
		TemplateSampleID string `json:"template_sample_id" binding:"required"`
		Name             string `json:"name" binding:"required"` // tên truyền từ request
		Description      string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ", "details": err.Error()})
		return
	}

	// Lấy UniversityID từ token
	claimsRaw, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	claims, ok := claimsRaw.(*utils.CustomClaims)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	facultyID, err := primitive.ObjectIDFromHex(req.FacultyID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chuyên ngành không hợp lệ"})
		return
	}

	templateSampleID, err := primitive.ObjectIDFromHex(req.TemplateSampleID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Giao diện mẫu không hợp lệ"})
		return
	}

	template, err := h.templateService.CreateTemplate(
		c.Request.Context(),
		universityID,
		facultyID,
		templateSampleID,
		req.Name,        // truyền name từ request
		req.Description, // truyền description từ request
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Đã tạo mẫu bằng",
		"template": template,
	})
}

func (h *TemplateHandler) UpdateDiplomaTemplate(c *gin.Context) {
	templateIDHex := c.Param("template_id")
	templateID, err := primitive.ObjectIDFromHex(templateIDHex)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã mẫu bằng không hợp lệ"})
		return
	}

	var req models.UpdateDiplomaTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.templateService.UpdateDiplomaTemplate(
		c.Request.Context(),
		templateID,
		req,
	)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy mẫu bằng"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Chỉ trả message
	c.JSON(http.StatusOK, gin.H{
		"message": "Cập nhật mẫu văn bằng thành công",
	})
}
