package service

import (
	"context"
	"errors"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
)

func canIssueDiploma(ctx context.Context, d *models.EDiploma) error {
	claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims)
	if !ok || claims.Role != "university_admin" || claims.UniversityID != d.UniversityID.Hex() {
		return errors.New("access denied")
	}
	if d.Revoked || d.Signed || d.PQCProof != nil || d.OnBlockchain || d.Issued {
		return errors.New("Văn bằng đã cấp không được ghi đè. Hãy thu hồi và tạo bản thay thế.")
	}
	return nil
}
