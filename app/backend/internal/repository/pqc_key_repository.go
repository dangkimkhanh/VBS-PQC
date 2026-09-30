package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PQCKeyRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, key *models.PQCKey) error
	ListByUniversity(ctx context.Context, universityID primitive.ObjectID) ([]models.PQCKey, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.PQCKey, error)
	FindByIDAndUniversity(ctx context.Context, id, universityID primitive.ObjectID) (*models.PQCKey, error)
	FindActive(ctx context.Context, universityID primitive.ObjectID) (*models.PQCKey, error)
	Activate(ctx context.Context, id, universityID primitive.ObjectID, now time.Time) error
	Revoke(ctx context.Context, id, universityID primitive.ObjectID, reason string, compromisedAt *time.Time, now time.Time) error
}

type pqcKeyRepository struct {
	collection *mongo.Collection
}

func NewPQCKeyRepository(db *mongo.Database) PQCKeyRepository {
	return &pqcKeyRepository{collection: db.Collection("pqc_signing_keys")}
}

func (r *pqcKeyRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "public_key_fingerprint", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_pqc_key_fingerprint"),
		},
		{
			Keys: bson.D{{Key: "university_id", Value: 1}, {Key: "algorithm", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("uniq_active_pqc_key_per_university_algorithm").
				SetPartialFilterExpression(bson.M{"status": models.PQCKeyActive}),
		},
		{
			// A university signs with exactly one active key, whatever its ML-DSA parameter set.
			Keys: bson.D{{Key: "university_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("uniq_active_pqc_key_per_university").
				SetPartialFilterExpression(bson.M{"status": models.PQCKeyActive}),
		},
		{
			Keys:    bson.D{{Key: "university_id", Value: 1}, {Key: "algorithm", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetName("pqc_key_lookup"),
		},
	})
	return err
}

func (r *pqcKeyRepository) Create(ctx context.Context, key *models.PQCKey) error {
	_, err := r.collection.InsertOne(ctx, key)
	return err
}

func (r *pqcKeyRepository) ListByUniversity(ctx context.Context, universityID primitive.ObjectID) ([]models.PQCKey, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"university_id": universityID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	// Return an empty slice when the university has no keys so the JSON API
	// responds with [] instead of null.
	keys := make([]models.PQCKey, 0)
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *pqcKeyRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.PQCKey, error) {
	var key models.PQCKey
	if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&key); err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *pqcKeyRepository) FindByIDAndUniversity(ctx context.Context, id, universityID primitive.ObjectID) (*models.PQCKey, error) {
	var key models.PQCKey
	if err := r.collection.FindOne(ctx, bson.M{"_id": id, "university_id": universityID}).Decode(&key); err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *pqcKeyRepository) FindActive(ctx context.Context, universityID primitive.ObjectID) (*models.PQCKey, error) {
	var key models.PQCKey
	err := r.collection.FindOne(ctx, bson.M{
		"university_id": universityID,
		"status":        models.PQCKeyActive,
	}).Decode(&key)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *pqcKeyRepository) Activate(ctx context.Context, id, universityID primitive.ObjectID, now time.Time) error {
	target, err := r.FindByIDAndUniversity(ctx, id, universityID)
	if err != nil {
		return err
	}
	if target.Status == models.PQCKeyRevoked {
		return fmt.Errorf("revoked keys cannot be activated")
	}
	if target.Status == models.PQCKeyActive {
		return nil
	}
	if target.Status != models.PQCKeyPending {
		return fmt.Errorf("only pending keys can be activated")
	}

	// Retire the current key even if it uses another parameter set; its past
	// signatures stay verifiable because retired_at is after their signing time.
	_, err = r.collection.UpdateMany(ctx, bson.M{
		"university_id": universityID,
		"status":        models.PQCKeyActive,
		"_id":           bson.M{"$ne": id},
	}, bson.M{"$set": bson.M{"status": models.PQCKeyRetired, "retired_at": now}})
	if err != nil {
		return err
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":           id,
		"university_id": universityID,
		"status":        models.PQCKeyPending,
	}, bson.M{
		"$set":   bson.M{"status": models.PQCKeyActive, "activated_at": now},
		"$unset": bson.M{"retired_at": ""},
	})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *pqcKeyRepository) Revoke(ctx context.Context, id, universityID primitive.ObjectID, reason string, compromisedAt *time.Time, now time.Time) error {
	set := bson.M{
		"status":            models.PQCKeyRevoked,
		"revoked_at":        now,
		"revocation_reason": reason,
	}
	if compromisedAt != nil {
		set["compromise_effective_at"] = *compromisedAt
	}
	result, err := r.collection.UpdateOne(ctx, bson.M{
		"_id":           id,
		"university_id": universityID,
		"status":        bson.M{"$ne": models.PQCKeyRevoked},
	}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return mongo.ErrNoDocuments
	}
	return nil
}
