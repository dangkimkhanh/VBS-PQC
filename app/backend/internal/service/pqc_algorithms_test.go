package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// memoryKeyRepo is a minimal in-memory PQCKeyRepository for signing tests.
type memoryKeyRepo struct {
	repository.PQCKeyRepository
	keys map[primitive.ObjectID]*models.PQCKey
}

func (r *memoryKeyRepo) Create(_ context.Context, key *models.PQCKey) error {
	r.keys[key.ID] = key
	return nil
}

func (r *memoryKeyRepo) FindByID(_ context.Context, id primitive.ObjectID) (*models.PQCKey, error) {
	if key, ok := r.keys[id]; ok {
		return key, nil
	}
	return nil, mongo.ErrNoDocuments
}

func (r *memoryKeyRepo) FindActive(_ context.Context, universityID primitive.ObjectID) (*models.PQCKey, error) {
	for _, key := range r.keys {
		if key.UniversityID == universityID && key.Status == models.PQCKeyActive {
			return key, nil
		}
	}
	return nil, mongo.ErrNoDocuments
}

func setTestMasterKey(t *testing.T) {
	t.Helper()
	master := make([]byte, 32)
	if _, err := rand.Read(master); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PQC_MASTER_KEY", base64.StdEncoding.EncodeToString(master))
}

func TestCreateKeyDefaultsToMLDSA65(t *testing.T) {
	setTestMasterKey(t)
	svc := &pqcService{keys: &memoryKeyRepo{keys: map[primitive.ObjectID]*models.PQCKey{}}, now: time.Now}
	key, err := svc.CreateKey(context.Background(), primitive.NewObjectID(), "Default key", "")
	if err != nil {
		t.Fatal(err)
	}
	if key.Algorithm != models.PQCAlgorithmMLDSA65 {
		t.Fatalf("default algorithm = %s, want %s", key.Algorithm, models.PQCAlgorithmMLDSA65)
	}
	if _, err := svc.CreateKey(context.Background(), primitive.NewObjectID(), "Bad", "ML-DSA-99"); err == nil {
		t.Fatal("unsupported algorithm was accepted")
	}
}

func TestEveryMLDSAParameterSetSignsAndVerifiesTransactions(t *testing.T) {
	setTestMasterKey(t)
	for _, name := range []string{models.PQCAlgorithmMLDSA44, models.PQCAlgorithmMLDSA65, models.PQCAlgorithmMLDSA87} {
		t.Run(name, func(t *testing.T) {
			repo := &memoryKeyRepo{keys: map[primitive.ObjectID]*models.PQCKey{}}
			universityID := primitive.NewObjectID()
			svc := &pqcService{keys: repo, now: time.Now}
			key, err := svc.CreateKey(context.Background(), universityID, "Key "+name, name)
			if err != nil {
				t.Fatal(err)
			}
			activated := time.Now().UTC().Add(-time.Minute)
			key.Status, key.ActivatedAt = models.PQCKeyActive, &activated

			signer := NewPQCTransactionSigner(repo)
			batch := &models.EDiplomaBatchOnChain{BatchID: "EDIP-test", UniversityID: universityID.Hex(), Count: 1}
			if err := signer.SignEDiplomaBatch(context.Background(), batch); err != nil {
				t.Fatal(err)
			}
			if batch.PQCTransaction.Algorithm != name {
				t.Fatalf("attestation algorithm = %s, want %s", batch.PQCTransaction.Algorithm, name)
			}
			if err := signer.VerifyEDiplomaBatch(context.Background(), batch); err != nil {
				t.Fatalf("valid %s attestation rejected: %v", name, err)
			}

			batch.Count = 2
			if err := signer.VerifyEDiplomaBatch(context.Background(), batch); err == nil {
				t.Fatal("tampered batch was accepted")
			}
			batch.Count = 1

			// Relabelling the signature as another parameter set must fail.
			for _, other := range []string{models.PQCAlgorithmMLDSA44, models.PQCAlgorithmMLDSA65, models.PQCAlgorithmMLDSA87} {
				if other == name {
					continue
				}
				batch.PQCTransaction.Algorithm = other
				if err := signer.VerifyEDiplomaBatch(context.Background(), batch); err == nil {
					t.Fatalf("%s signature accepted when labelled %s", name, other)
				}
			}
		})
	}
}

func TestSigningWithoutActiveKeyFails(t *testing.T) {
	setTestMasterKey(t)
	signer := NewPQCTransactionSigner(&memoryKeyRepo{keys: map[primitive.ObjectID]*models.PQCKey{}})
	err := signer.SignEDiplomaBatch(context.Background(), &models.EDiplomaBatchOnChain{UniversityID: primitive.NewObjectID().Hex()})
	if !errors.Is(err, ErrNoActivePQCKey) {
		t.Fatalf("err = %v, want ErrNoActivePQCKey", err)
	}
}
