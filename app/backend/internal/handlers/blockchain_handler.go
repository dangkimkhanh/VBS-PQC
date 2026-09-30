package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/service"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BlockchainHandler struct {
	BlockchainSvc service.BlockchainService
	EDiplomaSvc   service.EDiplomaService
}

func NewBlockchainHandler(blockchainSvc service.BlockchainService, ediplomaSvc service.EDiplomaService) *BlockchainHandler {
	return &BlockchainHandler{
		BlockchainSvc: blockchainSvc,
		EDiplomaSvc:   ediplomaSvc,
	}
}

func (h *BlockchainHandler) PushEDiplomasToBlockchain(c *gin.Context) {
	var req struct {
		FacultyID       string `form:"faculty_id" json:"faculty_id"`
		RoundID         string `form:"round_id" json:"round_id"`
		CertificateType string `form:"certificate_type" json:"certificate_type"`
		Course          string `form:"course" json:"course"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ", "details": err.Error()})
		return
	}

	// Lấy claims
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

	// Parse university_id
	universityID, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Phiên đăng nhập không hợp lệ, vui lòng đăng nhập lại"})
		return
	}

	// Anchoring covers one faculty of one issuance round.
	scope, err := h.EDiplomaSvc.RoundScope(c.Request.Context(), universityID, req.FacultyID, req.RoundID)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	count, err := h.BlockchainSvc.PushToBlockchain(
		c.Request.Context(),
		universityID.Hex(),
		scope,
		req.CertificateType,
		req.Course,
	)

	if err != nil {
		switch {
		case errors.Is(err, common.ErrInvalidFaculty):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, common.ErrNoDiplomas):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, common.ErrNoValidDiplomas):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Đã đẩy lên chuỗi khối",
		"updated_records": count,
	})
}

// PushRevocationsToBlockchain records on Fabric the revocations of one faculty of
// one issuance round, so verifiers see them even if the database is altered.
func (h *BlockchainHandler) PushRevocationsToBlockchain(c *gin.Context) {
	universityID, ok := universityAdminID(c)
	if !ok {
		return
	}
	var req struct {
		FacultyID string `json:"faculty_id"`
		RoundID   string `json:"round_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	scope, err := h.EDiplomaSvc.RoundScope(c.Request.Context(), universityID, req.FacultyID, req.RoundID)
	if err != nil {
		writeRoundError(c, err)
		return
	}
	result, err := h.BlockchainSvc.PushRevocationsToBlockchain(c.Request.Context(), universityID.Hex(), scope)
	switch {
	case errors.Is(err, service.ErrNoRevocationsToRecord):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "data": result})
	case errors.Is(err, service.ErrFabricUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrNoActivePQCKey):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		// Transactions already committed stay on the ledger; report them with the error.
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "data": result})
	default:
		c.JSON(http.StatusOK, gin.H{"message": "Đã ghi thu hồi lên Blockchain", "data": result})
	}
}
