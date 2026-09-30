package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UniversityHandler struct {
	universityService service.UniversityService
}

func NewUniversityHandler(s service.UniversityService) *UniversityHandler {
	return &UniversityHandler{universityService: s}
}

// writeUniversityError maps university service errors to HTTP responses.
func writeUniversityError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrUniversityNameExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Tên trường đã tồn tại"})
	case errors.Is(err, common.ErrUniversityEmailDomainExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Tên miền email đã được trường khác sử dụng"})
	case errors.Is(err, common.ErrUniversityCodeExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Mã trường đã tồn tại"})
	case errors.Is(err, common.ErrAccountUniversityAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": "Email quản trị đã được dùng cho tài khoản khác"})
	case errors.Is(err, common.ErrUniversityNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy trường"})
	case errors.Is(err, common.ErrAccountUniversityNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Trường chưa có tài khoản quản trị"})
	case errors.Is(err, service.ErrActivationRecentlySent):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
	case errors.Is(err, common.ErrUniversityApprovalEmailFailed):
		c.JSON(http.StatusBadGateway, gin.H{"error": "Không gửi được email kích hoạt. Kiểm tra cấu hình email rồi thử gửi lại"})
	case errors.Is(err, service.ErrInvalidEmailDomain), errors.Is(err, service.ErrInvalidPhone),
		errors.Is(err, service.ErrAccountAlreadyActive), errors.Is(err, service.ErrUniversityLocked):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("[UniversityHandler] unexpected error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi hệ thống, vui lòng thử lại"})
	}
}

func bindUniversityRequest(c *gin.Context, req interface{}) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		if errs, ok := common.ParseValidationError(err); ok {
			c.JSON(http.StatusBadRequest, gin.H{"errors": errs})
			return false
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return false
	}
	return true
}

func universityIDParam(c *gin.Context) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã trường không hợp lệ"})
		return primitive.NilObjectID, false
	}
	// Admin actions are logged against the university they affect.
	c.Set("audit_university", id.Hex())
	return id, true
}

func (h *UniversityHandler) CreateUniversity(c *gin.Context) {
	var req models.CreateUniversityRequest
	if !bindUniversityRequest(c, &req) {
		return
	}
	university, emailSent, err := h.universityService.CreateUniversity(c.Request.Context(), &req)
	if err != nil {
		writeUniversityError(c, err)
		return
	}
	c.Set("audit_university", university.ID.Hex())
	message := fmt.Sprintf("Đã tạo trường và gửi link kích hoạt tới %s", university.AdminEmail)
	if !emailSent {
		message = "Đã tạo trường nhưng chưa gửi được email kích hoạt. Hãy kiểm tra cấu hình email rồi bấm \"Gửi lại link kích hoạt\""
	}
	c.JSON(http.StatusCreated, gin.H{"message": message, "email_sent": emailSent, "id": university.ID.Hex()})
}

// GetMySchool returns the signed-in school admin's own university, limited to
// what the school screens need (e.g. the student email domain hint).
func (h *UniversityHandler) GetMySchool(c *gin.Context) {
	claims, ok := c.Get("claims")
	userClaims, isClaims := claims.(*utils.CustomClaims)
	if !ok || !isClaims {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không tìm thấy thông tin đăng nhập"})
		return
	}
	id, err := primitive.ObjectIDFromHex(userClaims.UniversityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}
	university, err := h.universityService.GetUniversityByID(c.Request.Context(), id)
	if err != nil || university == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy trường"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"university_name": university.UniversityName,
		"university_code": university.UniversityCode,
		"email_domain":    university.EmailDomain,
	}})
}

func (h *UniversityHandler) GetUniversity(c *gin.Context) {
	id, ok := universityIDParam(c)
	if !ok {
		return
	}
	resp, err := h.universityService.GetUniversityResponse(c.Request.Context(), id)
	if err != nil {
		writeUniversityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (h *UniversityHandler) UpdateUniversity(c *gin.Context) {
	id, ok := universityIDParam(c)
	if !ok {
		return
	}
	var req models.UpdateUniversityRequest
	if !bindUniversityRequest(c, &req) {
		return
	}
	activationSent, err := h.universityService.UpdateUniversity(c.Request.Context(), id, &req)
	if err != nil {
		writeUniversityError(c, err)
		return
	}
	message := "Đã cập nhật thông tin trường"
	if activationSent {
		message = "Đã cập nhật thông tin trường và gửi link kích hoạt tới email quản trị mới"
	}
	c.JSON(http.StatusOK, gin.H{"message": message, "activation_sent": activationSent})
}

func (h *UniversityHandler) LockUniversity(c *gin.Context) {
	id, ok := universityIDParam(c)
	if !ok {
		return
	}
	if err := h.universityService.LockUniversity(c.Request.Context(), id); err != nil {
		writeUniversityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã khóa trường và tài khoản quản trị"})
}

func (h *UniversityHandler) UnlockUniversity(c *gin.Context) {
	id, ok := universityIDParam(c)
	if !ok {
		return
	}
	if err := h.universityService.UnlockUniversity(c.Request.Context(), id); err != nil {
		writeUniversityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã mở khóa trường"})
}

func (h *UniversityHandler) ResendActivation(c *gin.Context) {
	id, ok := universityIDParam(c)
	if !ok {
		return
	}
	if err := h.universityService.ResendActivation(c.Request.Context(), id); err != nil {
		writeUniversityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã gửi lại link kích hoạt"})
}

func (h *UniversityHandler) GetAllUniversities(c *gin.Context) {
	var filter models.UniversityListFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bộ lọc không hợp lệ"})
		return
	}
	items, total, err := h.universityService.ListUniversities(c.Request.Context(), filter)
	if err != nil {
		log.Printf("[UniversityHandler] ListUniversities error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể lấy danh sách trường"})
		return
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	c.JSON(http.StatusOK, gin.H{
		"data": items, "total": total, "page": page, "page_size": pageSize,
		"total_page": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

func (h *UniversityHandler) Overview(c *gin.Context) {
	overview, err := h.universityService.Overview(c.Request.Context())
	if err != nil {
		log.Printf("[UniversityHandler] Overview error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tải thống kê hệ thống"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": overview})
}

func (h *UniversityHandler) RecentAdminEvents(c *gin.Context) {
	events, err := h.universityService.RecentAdminEvents(c.Request.Context(), 30)
	if err != nil {
		log.Printf("[UniversityHandler] RecentAdminEvents error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tải nhật ký quản trị"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}
