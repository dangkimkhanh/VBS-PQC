package repository

import (
	"context"
	"errors"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"regexp"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EDiplomaRepository interface {
	GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.EDiploma, error)
	UpdateByID(ctx context.Context, id primitive.ObjectID, update bson.M) error
	UpdateFields(ctx context.Context, id primitive.ObjectID, updates bson.M) error
	FindByStudentCode(ctx context.Context, studentCode string) (*models.EDiploma, error)
	FindByDynamicFilter(ctx context.Context, filter bson.M) ([]*models.EDiploma, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.EDiploma, error)
	Save(ctx context.Context, ediploma *models.EDiploma) error
	GetByFacultyID(ctx context.Context, facultyID primitive.ObjectID) ([]*models.EDiploma, error)
	SearchByFilters(ctx context.Context, filter models.EDiplomaSearchFilter) ([]*models.EDiploma, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, ed *models.EDiploma) error
	Revoke(ctx context.Context, id primitive.ObjectID, reason string, revokedAt time.Time) error
	FindByStudentCodeAndFacultyID(ctx context.Context, studentCode string, facultyID primitive.ObjectID) (*models.EDiploma, error)
	// RevokeMany revokes every issued, not yet revoked diploma matching filter within the caller's university.
	RevokeMany(ctx context.Context, filter bson.M, reason string, revokedAt time.Time) (int64, error)
	// AssignRound attaches the diplomas matching filter that have no round yet to roundID.
	AssignRound(ctx context.Context, filter bson.M, roundID primitive.ObjectID) (int64, error)
	DistinctRoundIDs(ctx context.Context, filter bson.M) ([]primitive.ObjectID, error)
	MarkRevocationOnChain(ctx context.Context, id primitive.ObjectID, revocationID, txID string) error
}

type eDiplomaRepository struct {
	db          *mongo.Collection
	facultyRepo FacultyRepository
}

func NewEDiplomaRepository(db *mongo.Database, facultyRepo FacultyRepository) EDiplomaRepository {
	return &eDiplomaRepository{
		db:          db.Collection("ediplomas"),
		facultyRepo: facultyRepo,
	}
}

func (r *eDiplomaRepository) UpdateByID(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["$push"] = bson.M{"history": degreeEvent(ctx, "Cập nhật Blockchain")}
	result, err := r.db.UpdateOne(
		ctx,
		bson.M{"_id": id, "revoked": bson.M{"$ne": true}},
		update,
	)
	if err == nil && result.MatchedCount == 0 {
		return errors.New("Văn bằng đã thu hồi hoặc không tồn tại")
	}
	return err
}

func (r *eDiplomaRepository) SearchByFilters(ctx context.Context, filter models.EDiplomaSearchFilter) ([]*models.EDiploma, int64, error) {
	bsonFilter := bson.M{}
	if filter.UniversityID != "" {
		universityID, err := primitive.ObjectIDFromHex(filter.UniversityID)
		if err == nil {
			// Support mixed historical data: university_id may be stored as ObjectID or string.
			bsonFilter["university_id"] = bson.M{"$in": []interface{}{universityID, filter.UniversityID}}
		} else {
			bsonFilter["university_id"] = filter.UniversityID
		}
	}
	if filter.FacultyID != "" {
		facultyID, err := primitive.ObjectIDFromHex(filter.FacultyID)
		if err == nil {
			// Support mixed historical data: faculty_id may be stored as ObjectID or string.
			bsonFilter["faculty_id"] = bson.M{"$in": []interface{}{facultyID, filter.FacultyID}}
		} else {
			bsonFilter["faculty_id"] = filter.FacultyID
		}
	}

	if filter.CertificateType != "" {
		bsonFilter["certificate_type"] = bson.M{"$regex": regexp.QuoteMeta(filter.CertificateType), "$options": "i"}
	}
	if filter.Course != "" {
		bsonFilter["course"] = bson.M{"$regex": regexp.QuoteMeta(filter.Course), "$options": "i"}
	}
	if filter.Issued != nil {
		bsonFilter["issued"] = *filter.Issued
	}
	if filter.Revoked != nil {
		bsonFilter["revoked"] = *filter.Revoked
	}
	switch round := strings.TrimSpace(filter.RoundID); {
	case round == models.RoundFilterNone:
		bsonFilter["round_id"] = bson.M{"$exists": false}
	case round != "":
		roundID, err := primitive.ObjectIDFromHex(round)
		if err != nil {
			return nil, 0, errors.New("invalid round_id")
		}
		bsonFilter["round_id"] = roundID
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		// Literal match on the student code or name, never a user-supplied pattern.
		pattern := bson.M{"$regex": regexp.QuoteMeta(keyword), "$options": "i"}
		bsonFilter["$or"] = bson.A{bson.M{"student_code": pattern}, bson.M{"full_name": pattern}}
	}

	// Đếm tổng số kết quả
	total, err := r.db.CountDocuments(ctx, bsonFilter)
	if err != nil {
		return nil, 0, err
	}

	// Tính skip/limit
	skip := int64((filter.Page - 1) * filter.PageSize)
	limit := int64(filter.PageSize)

	findOpts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{Key: "student_code", Value: 1}, {Key: "_id", Value: 1}})

	cursor, err := r.db.Find(ctx, bsonFilter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []*models.EDiploma
	for cursor.Next(ctx) {
		var e models.EDiploma
		if err := cursor.Decode(&e); err != nil {
			return nil, 0, err
		}
		results = append(results, &e)
	}

	if err := cursor.Err(); err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *eDiplomaRepository) UpdateFields(ctx context.Context, id primitive.ObjectID, updates bson.M) error {
	updates["updated_at"] = time.Now()
	filter := bson.M{"_id": id, "revoked": bson.M{"$ne": true}}
	if issued, ok := updates["issued"].(bool); ok && issued {
		filter["issued"] = bson.M{"$ne": true}
	}
	action := "Cập nhật văn bằng"
	if proof, ok := updates["pqc_proof"]; ok {
		filter["pqc_proof"] = nil
		filter["issued"] = true
		action = "Ký ML-DSA"
		if p, ok := proof.(*models.PQCProof); ok && p != nil {
			action = "Ký " + p.Algorithm
		}
	}
	if _, ok := updates["ediploma_file_hash"]; ok {
		filter["pqc_proof"] = nil
		filter["on_blockchain"] = bson.M{"$ne": true}
		filter["signed"] = bson.M{"$ne": true}
		action = "Cấp / cập nhật PDF"
	}
	result, err := r.db.UpdateOne(ctx, filter, bson.M{"$set": updates, "$push": bson.M{"history": degreeEvent(ctx, action)}})
	if err == nil && result.MatchedCount == 0 {
		return errors.New("Trạng thái văn bằng đã thay đổi; không thể ghi đè hoặc ký lại")
	}
	return err
}

func degreeEvent(ctx context.Context, action string) bson.M {
	actor, role := "system", "system"
	if claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims); ok {
		actor = claims.AccountID
		role = claims.Role
	}
	return bson.M{"_id": primitive.NewObjectID(), "at": time.Now().UTC(), "actor": actor, "role": role, "action": action, "success": true}
}

func (r *eDiplomaRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.EDiploma, error) {
	var diploma models.EDiploma
	err := r.db.FindOne(ctx, bson.M{"_id": id}).Decode(&diploma)
	if err != nil {
		return nil, err
	}
	return &diploma, nil
}

func (r *eDiplomaRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.EDiploma, error) {
	filter := bson.M{"user_id": userID}
	cursor, err := r.db.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var diplomas []*models.EDiploma
	if err := cursor.All(ctx, &diplomas); err != nil {
		return nil, err
	}
	return diplomas, nil
}

func (r *eDiplomaRepository) FindByDynamicFilter(ctx context.Context, filter bson.M) ([]*models.EDiploma, error) {
	// Nếu không truyền filter thì mặc định lấy tất cả
	if filter == nil {
		filter = bson.M{}
	}

	cursor, err := r.db.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*models.EDiploma
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *eDiplomaRepository) FindByStudentCodeAndFacultyID(ctx context.Context, studentCode string, facultyID primitive.ObjectID) (*models.EDiploma, error) {
	filter := bson.M{
		"student_code": studentCode,
		"faculty_id":   facultyID,
	}

	var ed models.EDiploma
	err := r.db.FindOne(ctx, filter).Decode(&ed)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ed, nil
}

func (r *eDiplomaRepository) Update(ctx context.Context, id primitive.ObjectID, ed *models.EDiploma) error {
	update := bson.M{
		"$set": bson.M{
			"template_id":         ed.TemplateID,
			"signature_of_uni":    ed.SignatureOfUni,
			"signature_of_minedu": ed.SignatureOfMinEdu,
			"issue_date":          ed.IssueDate,
			"issued":              ed.Issued,
			"on_blockchain":       ed.OnBlockchain,
			"updated_at":          time.Now(),
		},
	}
	_, err := r.db.UpdateByID(ctx, id, update)
	return err
}

func (r *eDiplomaRepository) Revoke(ctx context.Context, id primitive.ObjectID, reason string, revokedAt time.Time) error {
	claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims)
	if !ok || claims.Role != "university_admin" {
		return errors.New("access denied")
	}
	university, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		return err
	}
	result, err := r.db.UpdateOne(ctx, bson.M{"_id": id, "university_id": university, "issued": true, "revoked": bson.M{"$ne": true}}, bson.M{"$set": bson.M{
		"revoked_by": claims.AccountID,
		"revoked":    true, "revoked_at": revokedAt, "revocation_reason": reason, "updated_at": time.Now(),
	}, "$push": bson.M{"history": degreeEvent(ctx, "Thu hồi văn bằng")}})
	if err == nil && result.MatchedCount == 0 {
		return errors.New("ediploma is already revoked")
	}
	return err
}

func (r *eDiplomaRepository) FindByStudentCode(ctx context.Context, studentCode string) (*models.EDiploma, error) {
	var ediploma models.EDiploma

	filter := bson.M{
		"student_code": studentCode,
	}

	err := r.db.FindOne(ctx, filter).Decode(&ediploma)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Không tìm thấy
		}
		return nil, err
	}

	return &ediploma, nil
}

func (r *eDiplomaRepository) Save(ctx context.Context, ediploma *models.EDiploma) error {
	ediploma.History = []bson.M{degreeEvent(ctx, "Tạo hồ sơ văn bằng")}
	_, err := r.db.InsertOne(ctx, ediploma)
	return err
}
func (r *eDiplomaRepository) GetByFacultyID(ctx context.Context, facultyID primitive.ObjectID) ([]*models.EDiploma, error) {
	filter := bson.M{"faculty_id": facultyID}

	cursor, err := r.db.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ediplomas []*models.EDiploma
	if err := cursor.All(ctx, &ediplomas); err != nil {
		return nil, err
	}

	return ediplomas, nil
}

// universityOfCaller returns the university of the university admin in ctx.
func universityOfCaller(ctx context.Context) (primitive.ObjectID, *utils.CustomClaims, error) {
	claims, ok := ctx.Value(utils.ClaimsContextKey).(*utils.CustomClaims)
	if !ok || claims.Role != "university_admin" {
		return primitive.NilObjectID, nil, errors.New("access denied")
	}
	university, err := primitive.ObjectIDFromHex(claims.UniversityID)
	if err != nil {
		return primitive.NilObjectID, nil, err
	}
	return university, claims, nil
}

func (r *eDiplomaRepository) RevokeMany(ctx context.Context, filter bson.M, reason string, revokedAt time.Time) (int64, error) {
	university, claims, err := universityOfCaller(ctx)
	if err != nil {
		return 0, err
	}
	scoped := bson.M{}
	for k, v := range filter {
		scoped[k] = v
	}
	// These conditions always win over the caller's filter.
	scoped["university_id"] = university
	scoped["issued"] = true
	scoped["revoked"] = bson.M{"$ne": true}
	result, err := r.db.UpdateMany(ctx, scoped, bson.M{"$set": bson.M{
		"revoked_by": claims.AccountID,
		"revoked":    true, "revoked_at": revokedAt, "revocation_reason": reason, "updated_at": time.Now(),
	}, "$push": bson.M{"history": degreeEvent(ctx, "Thu hồi văn bằng theo đợt")}})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *eDiplomaRepository) AssignRound(ctx context.Context, filter bson.M, roundID primitive.ObjectID) (int64, error) {
	university, _, err := universityOfCaller(ctx)
	if err != nil {
		return 0, err
	}
	scoped := bson.M{}
	for k, v := range filter {
		scoped[k] = v
	}
	scoped["university_id"] = university
	scoped["round_id"] = bson.M{"$exists": false}
	result, err := r.db.UpdateMany(ctx, scoped, bson.M{
		"$set":  bson.M{"round_id": roundID, "updated_at": time.Now()},
		"$push": bson.M{"history": degreeEvent(ctx, "Gán đợt cấp")},
	})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *eDiplomaRepository) DistinctRoundIDs(ctx context.Context, filter bson.M) ([]primitive.ObjectID, error) {
	values, err := r.db.Distinct(ctx, "round_id", filter)
	if err != nil {
		return nil, err
	}
	ids := make([]primitive.ObjectID, 0, len(values))
	for _, v := range values {
		if id, ok := v.(primitive.ObjectID); ok && !id.IsZero() {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *eDiplomaRepository) MarkRevocationOnChain(ctx context.Context, id primitive.ObjectID, revocationID, txID string) error {
	result, err := r.db.UpdateOne(ctx,
		bson.M{"_id": id, "revoked": true, "revocation_on_chain": bson.M{"$ne": true}},
		bson.M{
			"$set":  bson.M{"revocation_on_chain": true, "revocation_id": revocationID, "revocation_tx_id": txID, "updated_at": time.Now()},
			"$push": bson.M{"history": degreeEvent(ctx, "Ghi thu hồi lên Blockchain")},
		})
	if err == nil && result.MatchedCount == 0 {
		return errors.New("diploma is not revoked or its revocation is already on the ledger")
	}
	return err
}
