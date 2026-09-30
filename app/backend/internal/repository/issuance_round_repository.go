package repository

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrIssuanceRoundExists = errors.New("issuance round name already exists")

type IssuanceRoundRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, round *models.IssuanceRound) error
	FindByID(ctx context.Context, universityID, id primitive.ObjectID) (*models.IssuanceRound, error)
	FindByIDs(ctx context.Context, universityID primitive.ObjectID, ids []primitive.ObjectID) (map[primitive.ObjectID]*models.IssuanceRound, error)
	// Search returns the newest rounds of a university whose name contains query.
	// When onlyIDs is not nil, the result is restricted to those rounds.
	Search(ctx context.Context, universityID primitive.ObjectID, query string, onlyIDs []primitive.ObjectID, limit int64) ([]models.IssuanceRound, error)
}

type issuanceRoundRepository struct {
	collection *mongo.Collection
}

func NewIssuanceRoundRepository(db *mongo.Database) IssuanceRoundRepository {
	return &issuanceRoundRepository{collection: db.Collection("issuance_rounds")}
}

func (r *issuanceRoundRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "university_id", Value: 1}, {Key: "name_key", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_round_name_per_university"),
		},
		{
			Keys:    bson.D{{Key: "university_id", Value: 1}, {Key: "created_at", Value: -1}},
			Options: options.Index().SetName("round_recent"),
		},
	})
	return err
}

func (r *issuanceRoundRepository) Create(ctx context.Context, round *models.IssuanceRound) error {
	_, err := r.collection.InsertOne(ctx, round)
	if mongo.IsDuplicateKeyError(err) {
		return ErrIssuanceRoundExists
	}
	return err
}

func (r *issuanceRoundRepository) FindByID(ctx context.Context, universityID, id primitive.ObjectID) (*models.IssuanceRound, error) {
	var round models.IssuanceRound
	if err := r.collection.FindOne(ctx, bson.M{"_id": id, "university_id": universityID}).Decode(&round); err != nil {
		return nil, err
	}
	return &round, nil
}

func (r *issuanceRoundRepository) FindByIDs(ctx context.Context, universityID primitive.ObjectID, ids []primitive.ObjectID) (map[primitive.ObjectID]*models.IssuanceRound, error) {
	result := map[primitive.ObjectID]*models.IssuanceRound{}
	if len(ids) == 0 {
		return result, nil
	}
	cursor, err := r.collection.Find(ctx, bson.M{"_id": bson.M{"$in": ids}, "university_id": universityID})
	if err != nil {
		return nil, err
	}
	var rounds []models.IssuanceRound
	if err := cursor.All(ctx, &rounds); err != nil {
		return nil, err
	}
	for i := range rounds {
		result[rounds[i].ID] = &rounds[i]
	}
	return result, nil
}

func (r *issuanceRoundRepository) Search(ctx context.Context, universityID primitive.ObjectID, query string, onlyIDs []primitive.ObjectID, limit int64) ([]models.IssuanceRound, error) {
	filter := bson.M{"university_id": universityID}
	if query = strings.TrimSpace(query); query != "" {
		// Match the typed text literally, whatever characters it contains.
		filter["name"] = bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"}
	}
	if onlyIDs != nil {
		filter["_id"] = bson.M{"$in": onlyIDs}
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(limit)
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	rounds := []models.IssuanceRound{}
	if err := cursor.All(ctx, &rounds); err != nil {
		return nil, err
	}
	return rounds, nil
}
