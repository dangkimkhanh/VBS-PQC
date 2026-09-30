package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PQCHandler struct {
	service service.PQCService
}

func NewPQCHandler(service service.PQCService) *PQCHandler {
	return &PQCHandler{service: service}
}

func (h *PQCHandler) CreateKey(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	var request models.CreatePQCKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key, err := h.service.CreateKey(c.Request.Context(), universityID, request.Name, request.Algorithm)
	if err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": key})
}

func (h *PQCHandler) ListKeys(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	keys, err := h.service.ListKeys(c.Request.Context(), universityID)
	if err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": keys})
}

func (h *PQCHandler) ActivateKey(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	keyID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã khóa không hợp lệ"})
		return
	}
	if err := h.service.ActivateKey(c.Request.Context(), universityID, keyID); err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã kích hoạt khóa ký"})
}

func (h *PQCHandler) RevokeKey(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	keyID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã khóa không hợp lệ"})
		return
	}
	var request models.RevokePQCKeyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.RevokeKey(c.Request.Context(), universityID, keyID, request.Reason, request.CompromiseEffectiveAt); err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã thu hồi khóa ký"})
}

func (h *PQCHandler) SignEDiploma(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	diplomaID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã văn bằng không hợp lệ"})
		return
	}
	proof, err := h.service.SignEDiploma(c.Request.Context(), universityID, diplomaID)
	if err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã ký văn bằng bằng " + proof.Algorithm, "data": proof})
}

func (h *PQCHandler) SignEDiplomaBatch(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	var request models.SignPQCFilterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	count, failures, err := h.service.SignEDiplomaBatch(c.Request.Context(), universityID, request)
	if err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Đã ký văn bằng",
		"signed":   count,
		"failures": failures,
	})
}

func (h *PQCHandler) VerifyEDiploma(c *gin.Context) {
	diplomaID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã văn bằng không hợp lệ"})
		return
	}
	result, err := h.service.VerifyEDiploma(c.Request.Context(), diplomaID)
	if err != nil {
		writePQCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func universityAdminID(c *gin.Context) (primitive.ObjectID, bool) {
	claimsRaw, exists := c.Get("claims")
	claims, ok := claimsRaw.(*utils.CustomClaims)
	if !exists || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return primitive.NilObjectID, false
	}
	if claims.Role != "university_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Chỉ quản trị trường được thực hiện thao tác này"})
		return primitive.NilObjectID, false
	}
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return primitive.NilObjectID, false
	}
	return universityID, true
}

func writePQCError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy dữ liệu"})
	case errors.Is(err, service.ErrRoundNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrPQCNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNoActivePQCKey), errors.Is(err, service.ErrSealAlgorithmMismatch):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
