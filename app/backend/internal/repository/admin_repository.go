package repository

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UniversityWithAccount is a university joined with its admin account status.
type UniversityWithAccount struct {
	models.University        `bson:",inline"`
	AdminAccountStatus       string     `bson:"admin_account_status"`
	AdminActivationExpiresAt *time.Time `bson:"admin_activation_expires_at,omitempty"`
}

// AdminRepository serves the platform-admin screens. Every statistic is a
// single grouped aggregation so the cost does not grow per university.
type AdminRepository interface {
	SearchUniversities(ctx context.Context, filter models.UniversityListFilter) ([]UniversityWithAccount, int64, error)
	StatsByUniversity(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]models.UniversityStats, error)
	TopIssuingUniversities(ctx context.Context, limit int) ([]primitive.ObjectID, error)
	PlatformCounts(ctx context.Context, overview *models.PlatformOverview) error
	RecentAdminEvents(ctx context.Context, limit int) ([]bson.M, error)
}

type adminRepository struct {
	db *mongo.Database
}

func NewAdminRepository(db *mongo.Database) AdminRepository {
	return &adminRepository{db: db}
}

func (r *adminRepository) SearchUniversities(ctx context.Context, filter models.UniversityListFilter) ([]UniversityWithAccount, int64, error) {
	match := bson.M{}
	if q := strings.TrimSpace(filter.Query); q != "" {
		pattern := bson.M{"$regex": regexp.QuoteMeta(q), "$options": "i"}
		match["$or"] = bson.A{
			bson.M{"university_name": pattern}, bson.M{"university_code": pattern}, bson.M{"email_domain": pattern},
			bson.M{"admin_email": pattern}, bson.M{"admin_name": pattern},
		}
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$lookup", Value: bson.M{
			"from": "accounts",
			"let":  bson.M{"uid": "$_id"},
			"pipeline": bson.A{
				bson.M{"$match": bson.M{"$expr": bson.M{"$and": bson.A{
					bson.M{"$eq": bson.A{"$university_id", "$$uid"}},
					bson.M{"$eq": bson.A{"$role", "university_admin"}},
				}}}},
				bson.M{"$project": bson.M{"status": 1, "activation_expires_at": 1}},
			},
			"as": "admin_account",
		}}},
		{{Key: "$addFields", Value: bson.M{
			"admin_account_status":        bson.M{"$ifNull": bson.A{bson.M{"$arrayElemAt": bson.A{"$admin_account.status", 0}}, ""}},
			"admin_activation_expires_at": bson.M{"$arrayElemAt": bson.A{"$admin_account.activation_expires_at", 0}},
		}}},
		{{Key: "$project", Value: bson.M{"admin_account": 0}}},
	}
	switch filter.Status {
	case "locked":
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{"status": models.UniversityLocked}}})
	case "pending":
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{"status": bson.M{"$ne": models.UniversityLocked}, "admin_account_status": models.AccountPending}}})
	case "active":
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.M{"status": bson.M{"$ne": models.UniversityLocked}, "admin_account_status": models.AccountActive}}})
	}
	skip := int64((filter.Page - 1) * filter.PageSize)
	pipeline = append(pipeline,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}}},
		bson.D{{Key: "$facet", Value: bson.M{
			"items": bson.A{bson.M{"$skip": skip}, bson.M{"$limit": filter.PageSize}},
			"total": bson.A{bson.M{"$count": "n"}},
		}}},
	)
	cursor, err := r.db.Collection("universities").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var result []struct {
		Items []UniversityWithAccount `bson:"items"`
		Total []struct {
			N int64 `bson:"n"`
		} `bson:"total"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		return nil, 0, err
	}
	if len(result) == 0 {
		return []UniversityWithAccount{}, 0, nil
	}
	var total int64
	if len(result[0].Total) > 0 {
		total = result[0].Total[0].N
	}
	return result[0].Items, total, nil
}

type groupCount struct {
	ID       primitive.ObjectID `bson:"_id"`
	Count    int64              `bson:"count"`
	Issued   int64              `bson:"issued"`
	Signed   int64              `bson:"signed"`
	Anchored int64              `bson:"anchored"`
	Revoked  int64              `bson:"revoked"`
}

func (r *adminRepository) groupBy(ctx context.Context, collection string, match bson.M, sums bson.M) ([]groupCount, error) {
	group := bson.M{"_id": "$university_id", "count": bson.M{"$sum": 1}}
	for k, v := range sums {
		group[k] = v
	}
	cursor, err := r.db.Collection(collection).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: group}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rows []groupCount
	return rows, cursor.All(ctx, &rows)
}

func flagSum(field string) bson.M {
	return bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$" + field, true}}, 1, 0}}}
}

func (r *adminRepository) StatsByUniversity(ctx context.Context, ids []primitive.ObjectID) (map[primitive.ObjectID]models.UniversityStats, error) {
	stats := make(map[primitive.ObjectID]models.UniversityStats, len(ids))
	if len(ids) == 0 {
		return stats, nil
	}
	in := bson.M{"$in": ids}
	users, err := r.groupBy(ctx, "users", bson.M{"university_id": in}, nil)
	if err != nil {
		return nil, err
	}
	for _, row := range users {
		s := stats[row.ID]
		s.Students = row.Count
		stats[row.ID] = s
	}
	accounts, err := r.groupBy(ctx, "accounts", bson.M{"university_id": in, "role": "student", "status": models.AccountActive}, nil)
	if err != nil {
		return nil, err
	}
	for _, row := range accounts {
		s := stats[row.ID]
		s.ActivatedStudents = row.Count
		stats[row.ID] = s
	}
	diplomas, err := r.groupBy(ctx, "ediplomas", bson.M{"university_id": in}, bson.M{
		"issued": flagSum("issued"), "signed": flagSum("signed"), "anchored": flagSum("on_blockchain"), "revoked": flagSum("revoked"),
	})
	if err != nil {
		return nil, err
	}
	for _, row := range diplomas {
		s := stats[row.ID]
		s.Diplomas, s.Issued, s.Signed, s.Anchored, s.Revoked = row.Count, row.Issued, row.Signed, row.Anchored, row.Revoked
		stats[row.ID] = s
	}
	return stats, nil
}

func (r *adminRepository) TopIssuingUniversities(ctx context.Context, limit int) ([]primitive.ObjectID, error) {
	cursor, err := r.db.Collection("ediplomas").Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"issued": true}}},
		{{Key: "$group", Value: bson.M{"_id": "$university_id", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
		{{Key: "$limit", Value: limit}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rows []groupCount
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	ids := make([]primitive.ObjectID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (r *adminRepository) PlatformCounts(ctx context.Context, o *models.PlatformOverview) error {
	count := func(collection string, filter bson.M) (int64, error) {
		return r.db.Collection(collection).CountDocuments(ctx, filter)
	}
	var err error
	if o.Universities.Total, err = count("universities", bson.M{}); err != nil {
		return err
	}
	if o.Universities.Locked, err = count("universities", bson.M{"status": models.UniversityLocked}); err != nil {
		return err
	}
	// A locked university's admin account is "locked", so this counts unlocked ones only.
	if o.Universities.AdminPending, err = count("accounts", bson.M{"role": "university_admin", "status": models.AccountPending}); err != nil {
		return err
	}
	o.Universities.Active = o.Universities.Total - o.Universities.Locked - o.Universities.AdminPending
	if o.Students.Total, err = count("users", bson.M{}); err != nil {
		return err
	}
	if o.Students.Activated, err = count("accounts", bson.M{"role": "student", "status": models.AccountActive}); err != nil {
		return err
	}
	if o.Verifications.Attempts, err = count("audit_events", bson.M{"verification_valid": bson.M{"$exists": true}}); err != nil {
		return err
	}
	if o.Verifications.Passed, err = count("audit_events", bson.M{"verification_valid": true}); err != nil {
		return err
	}

	cursor, err := r.db.Collection("ediplomas").Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id": nil, "count": bson.M{"$sum": 1},
			"issued": flagSum("issued"), "signed": flagSum("signed"), "anchored": flagSum("on_blockchain"), "revoked": flagSum("revoked"),
		}}},
	})
	if err != nil {
		return err
	}
	var totals []struct {
		Count, Issued, Signed, Anchored, Revoked int64
	}
	if err := cursor.All(ctx, &totals); err != nil {
		return err
	}
	if len(totals) > 0 {
		t := totals[0]
		o.Diplomas.Total, o.Diplomas.Issued, o.Diplomas.Signed, o.Diplomas.Anchored, o.Diplomas.Revoked = t.Count, t.Issued, t.Signed, t.Anchored, t.Revoked
	}

	cursor, err = r.db.Collection("pqc_signing_keys").Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": models.PQCKeyActive}}},
		{{Key: "$group", Value: bson.M{"_id": "$algorithm", "count": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return err
	}
	var algorithms []struct {
		ID    string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &algorithms); err != nil {
		return err
	}
	o.Algorithms = map[string]int64{}
	for _, a := range algorithms {
		o.Algorithms[a.ID] = a.Count
	}
	return nil
}

func (r *adminRepository) RecentAdminEvents(ctx context.Context, limit int) ([]bson.M, error) {
	cursor, err := r.db.Collection("audit_events").Find(ctx, bson.M{"role": "admin"},
		options.Find().SetSort(bson.D{{Key: "at", Value: -1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	events := []bson.M{}
	return events, cursor.All(ctx, &events)
}
