package service

import (
	"context"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

func TestIssuePolicyRejectsWrongOwnerAndFinalizedRecords(t *testing.T) {
	university := primitive.NewObjectID()
	for _, tc := range []struct {
		name, role, owner                 string
		issued, signed, revoked, anchored bool
		want                              bool
	}{
		{name: "draft", role: "university_admin", owner: university.Hex(), want: true},
		{name: "student cannot issue", role: "student", owner: university.Hex()},
		{name: "other university", role: "university_admin", owner: primitive.NewObjectID().Hex()},
		{name: "issued is immutable", role: "university_admin", owner: university.Hex(), issued: true},
		{name: "signed is immutable", role: "university_admin", owner: university.Hex(), signed: true},
		{name: "revocation is final", role: "university_admin", owner: university.Hex(), revoked: true},
		{name: "anchored is immutable", role: "university_admin", owner: university.Hex(), anchored: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), utils.ClaimsContextKey, &utils.CustomClaims{UniversityID: tc.owner, Role: tc.role})
			d := &models.EDiploma{UniversityID: university, Issued: tc.issued, Signed: tc.signed, Revoked: tc.revoked, OnBlockchain: tc.anchored}
			if (canIssueDiploma(ctx, d) == nil) != tc.want {
				t.Fatal("unexpected issuance authorization")
			}
		})
	}
	if canIssueDiploma(context.Background(), &models.EDiploma{}) == nil {
		t.Fatal("anonymous issuance allowed")
	}
}
