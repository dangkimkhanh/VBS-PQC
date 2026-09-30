package repository

import (
	"context"
	"regexp"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UniversityRepository interface {
	CheckUniversityConflicts(ctx context.Context, exceptID primitive.ObjectID, universityName, emailDomain, universityCode string) (string, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.University, error)
	FindByCode(ctx context.Context, code string) (*models.University, error)
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	UpdateDetails(ctx context.Context, id primitive.ObjectID, update bson.M) error
	CreateUniversity(ctx context.Context, uni *models.University) error
	DeleteByID(ctx context.Context, id primitive.ObjectID) error
	GetAllUniversities(ctx context.Context) ([]*models.University, error)
	GetUniversityByCode(ctx context.Context, code string) (*models.University, error)
}

type universityRepository struct {
	col *mongo.Collection
}

func NewUniversityRepository(db *mongo.Database) UniversityRepository {
	col := db.Collection("universities")
	return &universityRepository{col: col}
}
func (r *universityRepository) GetUniversityByCode(ctx context.Context, code string) (*models.University, error) {
	var university models.University
	err := r.col.FindOne(ctx, bson.M{"university_code": code}).Decode(&university)
	if err != nil {
		return nil, err
	}
	return &university, nil
}

func (r *universityRepository) CreateUniversity(ctx context.Context, uni *models.University) error {
	_, err := r.col.InsertOne(ctx, uni)
	return err
}

func (r *universityRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.University, error) {
	var university models.University
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&university)
	if err != nil {
		return nil, err
	}
	return &university, nil
}

func (r *universityRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	})
	return err
}

func (r *universityRepository) UpdateDetails(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *universityRepository) GetAllUniversities(ctx context.Context) ([]*models.University, error) {
	cursor, err := r.col.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var universities []*models.University
	if err := cursor.All(ctx, &universities); err != nil {
		return nil, err
	}
	return universities, nil
}
func (r *universityRepository) FindByCode(ctx context.Context, code string) (*models.University, error) {
	filter := bson.M{"university_code": code}
	var university models.University
	err := r.col.FindOne(ctx, filter).Decode(&university)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &university, nil
}

// CheckUniversityConflicts reports the first field already used by another
// university. Names and domains compare case-insensitively; an empty code is skipped.
func (r *universityRepository) CheckUniversityConflicts(ctx context.Context, exceptID primitive.ObjectID, universityName, emailDomain, universityCode string) (string, error) {
	checks := []struct{ field, value string }{
		{"university_name", universityName},
		{"email_domain", emailDomain},
		{"university_code", universityCode},
	}
	for _, check := range checks {
		if check.value == "" {
			continue
		}
		count, err := r.col.CountDocuments(ctx, bson.M{
			"_id":       bson.M{"$ne": exceptID},
			check.field: bson.M{"$regex": "^" + regexp.QuoteMeta(check.value) + "$", "$options": "i"},
		})
		if err != nil {
			return "", err
		}
		if count > 0 {
			return check.field, nil
		}
	}
	return "", nil
}

func (r *universityRepository) DeleteByID(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
