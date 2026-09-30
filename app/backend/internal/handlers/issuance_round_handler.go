package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// writeRoundError maps round and faculty validation errors to 400/404/409.
func writeRoundError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRoundRequired), errors.Is(err, service.ErrFacultyRequired),
		errors.Is(err, service.ErrRoundNameInvalid), errors.Is(err, service.ErrRevokeReason):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrRoundNotFound), errors.Is(err, service.ErrNothingToRevoke):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrRoundExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// requireRound resolves the round named in a request; it writes the error itself.
func (h *EDiplomaHandler) requireRound(c *gin.Context, claims *utils.CustomClaims, roundID string) (*models.IssuanceRound, bool) {
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return nil, false
	}
	round, err := h.ediplomaService.RequireRound(c.Request.Context(), universityID, roundID)
	if err != nil {
		writeRoundError(c, err)
		return nil, false
	}
	return round, true
}

// ListRounds suggests rounds for the round picker: the newest ones, or those whose
// name contains q. With faculty_id, only rounds holding that faculty's diplomas.
func (h *EDiplomaHandler) ListRounds(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	rounds, err := h.ediplomaService.SearchRounds(c.Request.Context(), universityID, c.Query("q"), c.Query("faculty_id"), limit)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rounds})
}

func (h *EDiplomaHandler) CreateRound(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	var req models.CreateIssuanceRoundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	round, err := h.ediplomaService.CreateRound(c.Request.Context(), universityID, req)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": round})
}

// AssignRound attaches a faculty's diplomas that have no round yet to a round.
func (h *EDiplomaHandler) AssignRound(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	var req struct {
		RoundID         string `json:"round_id"`
		FacultyID       string `json:"faculty_id"`
		Course          string `json:"course"`
		CertificateType string `json:"certificate_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	count, err := h.ediplomaService.AssignRound(c.Request.Context(), universityID, req.RoundID, req.FacultyID, req.Course, req.CertificateType)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã gán đợt cấp", "updated": count})
}

// RevokeRound revokes every issued diploma of one faculty in one round.
func (h *EDiplomaHandler) RevokeRound(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	var req struct {
		FacultyID string `json:"faculty_id"`
		RoundID   string `json:"round_id"`
		Reason    string `json:"reason"`
		// Confirm must repeat the round name, so a whole round is never revoked by a stray click.
		Confirm string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	round, err := h.ediplomaService.RequireRound(c.Request.Context(), universityID, req.RoundID)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	if !strings.EqualFold(strings.Join(strings.Fields(req.Confirm), " "), round.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nhập đúng tên đợt cấp để xác nhận thu hồi"})
		return
	}
	count, err := h.ediplomaService.RevokeRound(c.Request.Context(), universityID, req.FacultyID, req.RoundID, req.Reason)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã thu hồi văn bằng của đợt cấp", "revoked": count})
}
