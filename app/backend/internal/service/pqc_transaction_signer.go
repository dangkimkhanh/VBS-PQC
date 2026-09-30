package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// PQCTransactionSigner creates and verifies the post-quantum attestation
// attached to a Fabric transaction payload. It deliberately does not replace
// Fabric's X.509 identity or endorsement flow.
type PQCTransactionSigner struct {
	keys repository.PQCKeyRepository
	now  func() time.Time
}

func NewPQCTransactionSigner(keys repository.PQCKeyRepository) *PQCTransactionSigner {
	return &PQCTransactionSigner{keys: keys, now: time.Now}
}

func (s *PQCTransactionSigner) SignEDiplomaBatch(ctx context.Context, batch *models.EDiplomaBatchOnChain) error {
	if batch == nil {
		return errors.New("eDiploma batch is required")
	}
	universityID, err := primitive.ObjectIDFromHex(batch.UniversityID)
	if err != nil {
		return fmt.Errorf("invalid university ID for PQC transaction signing: %w", err)
	}
	key, err := s.keys.FindActive(ctx, universityID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNoActivePQCKey
	}
	if err != nil {
		return err
	}
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("generate transaction nonce: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	signedAt := s.now().UTC()
	envelope := transactionEnvelope(batch, key.ID.Hex(), signedAt, nonce)
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal transaction envelope: %w", err)
	}
	envelopeHash := sha256.Sum256(envelopeBytes)
	signature, err := signWithPQCKey(key, envelopeBytes, []byte(models.PQCTransactionContext))
	if err != nil {
		return fmt.Errorf("sign Fabric transaction envelope with %s: %w", key.Algorithm, err)
	}
	batch.PQCTransaction = &models.PQCTransactionAttestation{
		Version:              models.PQCTransactionVersion,
		Algorithm:            key.Algorithm,
		Context:              models.PQCTransactionContext,
		KeyID:                key.ID.Hex(),
		PublicKey:            key.PublicKey,
		PublicKeyFingerprint: key.PublicKeyFingerprint,
		EnvelopeHash:         hex.EncodeToString(envelopeHash[:]),
		Signature:            base64.StdEncoding.EncodeToString(signature),
		SignedAt:             signedAt,
		Nonce:                nonce,
	}
	return nil
}

func (s *PQCTransactionSigner) VerifyEDiplomaBatch(ctx context.Context, batch *models.EDiplomaBatchOnChain) error {
	if batch == nil || batch.PQCTransaction == nil {
		return errors.New("batch has no PQC transaction attestation")
	}
	attestation := batch.PQCTransaction
	if attestation.Version != models.PQCTransactionVersion ||
		attestation.Context != models.PQCTransactionContext ||
		strings.TrimSpace(attestation.KeyID) == "" ||
		strings.TrimSpace(attestation.Nonce) == "" {
		return errors.New("invalid PQC transaction attestation metadata")
	}
	keyID, err := primitive.ObjectIDFromHex(attestation.KeyID)
	if err != nil {
		return errors.New("invalid PQC transaction key ID")
	}
	key, err := s.keys.FindByID(ctx, keyID)
	if err != nil {
		return errors.New("PQC transaction verification key is unavailable")
	}
	algorithm, err := lookupPQCAlgorithm(attestation.Algorithm)
	if err != nil || key.Algorithm != attestation.Algorithm {
		return errors.New("PQC transaction algorithm does not match the signing key")
	}
	if key.UniversityID.Hex() != batch.UniversityID || key.PublicKeyFingerprint != attestation.PublicKeyFingerprint {
		return errors.New("PQC transaction key identity does not match batch")
	}
	envelope := transactionEnvelope(batch, attestation.KeyID, attestation.SignedAt.UTC(), attestation.Nonce)
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal transaction envelope for verification: %w", err)
	}
	digest := sha256.Sum256(envelopeBytes)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), attestation.EnvelopeHash) {
		return errors.New("PQC transaction envelope hash mismatch")
	}
	publicBytes, err := base64.StdEncoding.DecodeString(key.PublicKey)
	if err != nil {
		return errors.New("stored PQC transaction public key is corrupted")
	}
	signature, err := base64.StdEncoding.DecodeString(attestation.Signature)
	if err != nil || !algorithm.verify(publicBytes, envelopeBytes, []byte(attestation.Context), signature) {
		return errors.New("PQC transaction signature verification failed")
	}
	if !keyWasValidAt(key, attestation.SignedAt) {
		return errors.New("PQC transaction key was not valid at signing time")
	}
	return nil
}

// SignEDiplomaRevocation attaches an ML-DSA attestation to a ledger revocation,
// signed under PQCRevocationContext so it cannot be confused with an issuance.
func (s *PQCTransactionSigner) SignEDiplomaRevocation(ctx context.Context, revocation *models.EDiplomaRevocationOnChain) error {
	if revocation == nil {
		return errors.New("revocation is required")
	}
	universityID, err := primitive.ObjectIDFromHex(revocation.UniversityID)
	if err != nil {
		return fmt.Errorf("invalid university ID for PQC revocation signing: %w", err)
	}
	key, err := s.keys.FindActive(ctx, universityID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNoActivePQCKey
	}
	if err != nil {
		return err
	}
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("generate revocation nonce: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	signedAt := s.now().UTC()
	envelopeBytes, err := json.Marshal(revocationEnvelope(revocation, key.ID.Hex(), signedAt, nonce))
	if err != nil {
		return fmt.Errorf("marshal revocation envelope: %w", err)
	}
	envelopeHash := sha256.Sum256(envelopeBytes)
	signature, err := signWithPQCKey(key, envelopeBytes, []byte(models.PQCRevocationContext))
	if err != nil {
		return fmt.Errorf("sign revocation envelope with %s: %w", key.Algorithm, err)
	}
	revocation.PQCTransaction = &models.PQCTransactionAttestation{
		Version:              models.PQCTransactionVersion,
		Algorithm:            key.Algorithm,
		Context:              models.PQCRevocationContext,
		KeyID:                key.ID.Hex(),
		PublicKey:            key.PublicKey,
		PublicKeyFingerprint: key.PublicKeyFingerprint,
		EnvelopeHash:         hex.EncodeToString(envelopeHash[:]),
		Signature:            base64.StdEncoding.EncodeToString(signature),
		SignedAt:             signedAt,
		Nonce:                nonce,
	}
	return nil
}

// revocationEnvelope must stay byte-identical to pqcRevocationEnvelope in the chaincode.
func revocationEnvelope(revocation *models.EDiplomaRevocationOnChain, keyID string, signedAt time.Time, nonce string) models.PQCRevocationEnvelope {
	return models.PQCRevocationEnvelope{
		Version:         models.PQCTransactionVersion,
		TransactionType: "RevokeEDiplomaBatch",
		RevocationID:    revocation.RevocationID,
		UniversityID:    revocation.UniversityID,
		FacultyID:       revocation.FacultyID,
		RoundID:         revocation.RoundID,
		ItemsHash:       revocation.ItemsHash,
		Count:           revocation.Count,
		KeyID:           keyID,
		SignedAt:        signedAt.UTC().Format(time.RFC3339Nano),
		Nonce:           nonce,
	}
}

func transactionEnvelope(batch *models.EDiplomaBatchOnChain, keyID string, signedAt time.Time, nonce string) models.PQCTransactionEnvelope {
	return models.PQCTransactionEnvelope{
		Version:           models.PQCTransactionVersion,
		TransactionType:   "IssueEDiplomaBatch",
		BatchID:           batch.BatchID,
		UniversityID:      batch.UniversityID,
		FacultyID:         batch.FacultyID,
		CertificateType:   batch.CertificateType,
		Course:            batch.Course,
		AggregateInfoHash: batch.AggregateInfoHash,
		AggregateFileHash: batch.AggregateFileHash,
		Count:             batch.Count,
		KeyID:             keyID,
		SignedAt:          signedAt.UTC().Format(time.RFC3339Nano),
		Nonce:             nonce,
	}
}
