package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrPQCNotConfigured = errors.New("PQC_MASTER_KEY is not configured as a base64-encoded 32-byte key")
	ErrNoActivePQCKey   = errors.New("Trường chưa có khóa ký ML-DSA đang hoạt động. Hãy tạo và kích hoạt khóa ký trước")
	// ErrSealAlgorithmMismatch: the key was rotated to another parameter set after the PDF was rendered.
	ErrSealAlgorithmMismatch = errors.New("Bộ tham số của khóa ký không khớp với dấu trên văn bằng")
)

type PQCService interface {
	CreateKey(ctx context.Context, universityID primitive.ObjectID, name, algorithm string) (*models.PQCKey, error)
	ListKeys(ctx context.Context, universityID primitive.ObjectID) ([]models.PQCKey, error)
	ActivateKey(ctx context.Context, universityID, keyID primitive.ObjectID) error
	RevokeKey(ctx context.Context, universityID, keyID primitive.ObjectID, reason string, compromisedAt *time.Time) error
	SignEDiploma(ctx context.Context, universityID, diplomaID primitive.ObjectID) (*models.PQCProof, error)
	SignEDiplomaBatch(ctx context.Context, universityID primitive.ObjectID, request models.SignPQCFilterRequest) (int, []string, error)
	VerifyEDiploma(ctx context.Context, diplomaID primitive.ObjectID) (*models.PQCVerificationResult, error)
}

type pqcService struct {
	keys          repository.PQCKeyRepository
	ediplomas     repository.EDiplomaRepository
	ediplomaFiles EDiplomaService
	blockchain    BlockchainService
	now           func() time.Time
}

func NewPQCService(
	keys repository.PQCKeyRepository,
	ediplomas repository.EDiplomaRepository,
	ediplomaFiles EDiplomaService,
	blockchainService BlockchainService,
) PQCService {
	return &pqcService{
		keys: keys, ediplomas: ediplomas, ediplomaFiles: ediplomaFiles,
		blockchain: blockchainService, now: time.Now,
	}
}

func (s *pqcService) CreateKey(ctx context.Context, universityID primitive.ObjectID, name, algorithmName string) (*models.PQCKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("key name is required")
	}
	if algorithmName == "" {
		algorithmName = models.PQCDefaultAlgorithm
	}
	algorithm, err := lookupPQCAlgorithm(algorithmName)
	if err != nil {
		return nil, err
	}
	masterKey, err := loadPQCMasterKey()
	if err != nil {
		return nil, err
	}
	defer clear(masterKey)

	publicBytes, privateBytes, err := algorithm.generate()
	if err != nil {
		return nil, fmt.Errorf("generate %s key: %w", algorithmName, err)
	}
	defer clear(privateBytes)
	id := primitive.NewObjectID()
	ciphertext, nonce, err := encryptPrivateKey(masterKey, privateBytes, keyAAD(id, universityID))
	if err != nil {
		return nil, err
	}
	fingerprint := sha256.Sum256(publicBytes)
	key := &models.PQCKey{
		ID:                   id,
		UniversityID:         universityID,
		Name:                 name,
		Algorithm:            algorithmName,
		Status:               models.PQCKeyPending,
		PublicKey:            base64.StdEncoding.EncodeToString(publicBytes),
		PublicKeyFingerprint: hex.EncodeToString(fingerprint[:]),
		EncryptedPrivateKey:  base64.StdEncoding.EncodeToString(ciphertext),
		PrivateKeyNonce:      base64.StdEncoding.EncodeToString(nonce),
		CreatedAt:            s.now().UTC(),
	}
	if err := s.keys.Create(ctx, key); err != nil {
		return nil, fmt.Errorf("store PQC key: %w", err)
	}
	return key, nil
}

func (s *pqcService) ListKeys(ctx context.Context, universityID primitive.ObjectID) ([]models.PQCKey, error) {
	return s.keys.ListByUniversity(ctx, universityID)
}

func (s *pqcService) ActivateKey(ctx context.Context, universityID, keyID primitive.ObjectID) error {
	return s.keys.Activate(ctx, keyID, universityID, s.now().UTC())
}

func (s *pqcService) RevokeKey(ctx context.Context, universityID, keyID primitive.ObjectID, reason string, compromisedAt *time.Time) error {
	reason = strings.TrimSpace(reason)
	if len(reason) < 5 {
		return errors.New("revocation reason must contain at least 5 characters")
	}
	if compromisedAt != nil && compromisedAt.After(s.now()) {
		return errors.New("compromise effective time cannot be in the future")
	}
	return s.keys.Revoke(ctx, keyID, universityID, reason, compromisedAt, s.now().UTC())
}

func (s *pqcService) SignEDiploma(ctx context.Context, universityID, diplomaID primitive.ObjectID) (*models.PQCProof, error) {
	diploma, err := s.ediplomas.FindByID(ctx, diplomaID)
	if err != nil {
		return nil, err
	}
	if diploma.UniversityID != universityID {
		return nil, mongo.ErrNoDocuments
	}
	if diploma.Revoked {
		return nil, errors.New("revoked diploma cannot be signed")
	}
	if !diploma.Issued || strings.TrimSpace(diploma.EDiplomaFileHash) == "" {
		return nil, errors.New("diploma must be issued and have a file hash before signing")
	}
	if diploma.OnBlockchain {
		return nil, errors.New("a diploma anchored on blockchain cannot be re-signed")
	}
	if diploma.PQCProof != nil {
		return nil, errors.New("diploma already has a PQC proof")
	}

	key, err := s.keys.FindActive(ctx, universityID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNoActivePQCKey
	}
	if err != nil {
		return nil, err
	}
	if diploma.SealAlgorithm != "" && diploma.SealAlgorithm != key.Algorithm {
		return nil, fmt.Errorf("%w: văn bằng in dấu %s nhưng khóa đang hoạt động là %s", ErrSealAlgorithmMismatch, diploma.SealAlgorithm, key.Algorithm)
	}
	// BSON dates retain milliseconds; sign the exact precision that can be persisted.
	now := s.now().UTC().Truncate(time.Millisecond)
	manifestBytes, manifestHash, err := marshalCredentialManifest(diploma, now)
	if err != nil {
		return nil, err
	}
	signature, err := signWithPQCKey(key, manifestBytes, []byte(models.PQCSignatureContext))
	if err != nil {
		return nil, fmt.Errorf("sign diploma with %s: %w", key.Algorithm, err)
	}
	proof := &models.PQCProof{
		Version:              models.PQCProofVersion,
		AnchorVersion:        models.PQCAnchorVersion,
		Algorithm:            key.Algorithm,
		Context:              models.PQCSignatureContext,
		KeyID:                key.ID.Hex(),
		PublicKeyFingerprint: key.PublicKeyFingerprint,
		ManifestHash:         manifestHash,
		Signature:            base64.StdEncoding.EncodeToString(signature),
		SignedAt:             now,
	}
	if err := s.ediplomas.UpdateFields(ctx, diploma.ID, bson.M{
		"pqc_proof":  proof,
		"signature":  proof.Signature,
		"signed":     true,
		"signed_at":  now,
		"updated_at": now,
	}); err != nil {
		return nil, fmt.Errorf("store PQC proof: %w", err)
	}
	return proof, nil
}

func (s *pqcService) SignEDiplomaBatch(ctx context.Context, universityID primitive.ObjectID, request models.SignPQCFilterRequest) (int, []string, error) {
	// Batch signing covers one faculty of one issuance round.
	if s.ediplomaFiles == nil {
		return 0, nil, errors.New("diploma service is not configured")
	}
	filter, err := s.ediplomaFiles.RoundScope(ctx, universityID, request.FacultyID, request.RoundID)
	if err != nil {
		return 0, nil, err
	}
	filter["issued"] = true
	filter["pqc_proof"] = bson.M{"$exists": false}
	filter["on_blockchain"] = bson.M{"$ne": true}
	if request.TemplateID != "" {
		id, err := primitive.ObjectIDFromHex(request.TemplateID)
		if err != nil {
			return 0, nil, errors.New("invalid template_id")
		}
		filter["template_id"] = id
	}
	if value := strings.TrimSpace(request.CertificateType); value != "" {
		filter["certificate_type"] = bson.M{"$regex": regexp.QuoteMeta(value), "$options": "i"}
	}
	if value := strings.TrimSpace(request.Course); value != "" {
		filter["course"] = bson.M{"$regex": regexp.QuoteMeta(value), "$options": "i"}
	}
	diplomas, err := s.ediplomas.FindByDynamicFilter(ctx, filter)
	if err != nil {
		return 0, nil, err
	}
	if len(diplomas) == 0 {
		return 0, nil, errors.New("no unsigned, issued diplomas match the selected filter")
	}

	count := 0
	failures := make([]string, 0)
	for _, diploma := range diplomas {
		if _, err := s.SignEDiploma(ctx, universityID, diploma.ID); err != nil {
			failures = append(failures, diploma.ID.Hex()+": "+err.Error())
			continue
		}
		count++
	}
	return count, failures, nil
}

func (s *pqcService) VerifyEDiploma(ctx context.Context, diplomaID primitive.ObjectID) (*models.PQCVerificationResult, error) {
	diploma, err := s.ediplomas.FindByID(ctx, diplomaID)
	if err != nil {
		return nil, err
	}
	proof := diploma.PQCProof
	if proof == nil {
		return &models.PQCVerificationResult{Valid: false, Error: "diploma has no PQC proof"}, nil
	}
	result := &models.PQCVerificationResult{
		Algorithm:        proof.Algorithm,
		KeyID:            proof.KeyID,
		ManifestHash:     proof.ManifestHash,
		SignedAt:         proof.SignedAt,
		CurrentKeyStatus: "unknown",
		AssuranceLevel:   "invalid",
		Blockchain:       models.BlockchainAnchorVerification{Status: "not_checked"},
	}
	if proof.Version != models.PQCProofVersion ||
		(proof.AnchorVersion != 0 && proof.AnchorVersion != models.PQCAnchorVersion) ||
		proof.Context != models.PQCSignatureContext {
		result.Error = "unsupported or inconsistent PQC proof metadata"
		return result, nil
	}
	keyID, err := primitive.ObjectIDFromHex(proof.KeyID)
	if err != nil {
		result.Error = "invalid PQC key ID"
		return result, nil
	}
	key, err := s.keys.FindByID(ctx, keyID)
	if err != nil {
		result.Error = "PQC verification key is unavailable"
		return result, nil
	}
	result.CurrentKeyStatus = key.Status
	// The proof must name the same parameter set as the key record, so a
	// signature can never be checked under a different algorithm than it claims.
	algorithm, algErr := lookupPQCAlgorithm(proof.Algorithm)
	if algErr != nil || key.Algorithm != proof.Algorithm {
		result.Error = "unsupported or inconsistent PQC proof algorithm"
		return result, nil
	}
	if key.UniversityID != diploma.UniversityID || key.PublicKeyFingerprint != proof.PublicKeyFingerprint {
		result.Error = "PQC key identity does not match the diploma proof"
		return result, nil
	}
	manifestBytes, manifestHash, err := marshalCredentialManifest(diploma, proof.SignedAt)
	if err != nil {
		return nil, err
	}
	if manifestHash != proof.ManifestHash {
		result.Error = "credential manifest was modified after signing"
		return result, nil
	}
	publicBytes, err := base64.StdEncoding.DecodeString(key.PublicKey)
	if err != nil {
		result.Error = "stored public key is corrupted"
		return result, nil
	}
	signature, err := base64.StdEncoding.DecodeString(proof.Signature)
	if err != nil {
		result.Error = "PQC signature encoding is invalid"
		return result, nil
	}
	result.CryptographicallyValid = algorithm.verify(publicBytes, manifestBytes, []byte(proof.Context), signature)
	result.KeyValidAtSigning = keyWasValidAt(key, proof.SignedAt)
	result.SignatureValid = result.CryptographicallyValid && result.KeyValidAtSigning
	if key.Status == models.PQCKeyRevoked && result.SignatureValid {
		result.Warning = "the key was revoked after this signature; historical verification remains valid"
	}
	if !result.CryptographicallyValid {
		result.Error = proof.Algorithm + " signature verification failed"
	} else if !result.KeyValidAtSigning {
		result.Error = "the key was not valid at the recorded signing time"
	}

	result.StoredFileHash = strings.ToLower(strings.TrimSpace(diploma.EDiplomaFileHash))
	if s.ediplomaFiles != nil {
		stream, _, fileErr := s.ediplomaFiles.GetEDiplomaFile(ctx, diploma.ID, diploma.UniversityID)
		if fileErr != nil {
			result.Error = appendVerificationMessage(result.Error, "cannot read the issued PDF: "+fileErr.Error())
		} else {
			hasher := sha256.New()
			_, hashErr := io.Copy(hasher, stream)
			closeErr := stream.Close()
			if hashErr != nil {
				result.Error = appendVerificationMessage(result.Error, "cannot hash the issued PDF: "+hashErr.Error())
			} else {
				result.FileIntegrityChecked = true
				result.ComputedFileHash = hex.EncodeToString(hasher.Sum(nil))
				result.FileIntegrityValid = strings.EqualFold(result.ComputedFileHash, result.StoredFileHash)
				if !result.FileIntegrityValid {
					result.Error = appendVerificationMessage(result.Error, "the issued PDF hash does not match the signed manifest")
				}
			}
			if closeErr != nil {
				result.Warning = appendVerificationMessage(result.Warning, "PDF stream close warning: "+closeErr.Error())
			}
		}
	}

	if s.blockchain != nil {
		anchor, anchorErr := s.blockchain.VerifyEDiplomaAnchor(ctx, diploma.ID)
		if anchorErr != nil {
			result.Blockchain = models.BlockchainAnchorVerification{Status: "unavailable", Error: anchorErr.Error()}
		} else {
			result.Blockchain = *anchor
		}
	}

	// A revocation recorded on Fabric counts even if the database record says otherwise.
	revoked := diploma.Revoked || result.Blockchain.RevokedOnChain
	result.Valid = !revoked && result.SignatureValid && result.FileIntegrityValid && result.Blockchain.Valid
	switch {
	case revoked:
		result.AssuranceLevel = "revoked"
		result.Error = appendVerificationMessage(result.Error, "Văn bằng đã thu hồi")
		if result.Blockchain.RevokedOnChain {
			result.Warning = appendVerificationMessage(result.Warning, "the revocation is recorded on Fabric")
		}
	case result.Valid:
		result.AssuranceLevel = "complete"
	case result.SignatureValid && result.FileIntegrityValid && result.Blockchain.Status == "not_anchored":
		result.AssuranceLevel = "signature_only"
		result.Warning = appendVerificationMessage(result.Warning, "the signature and PDF are valid, but the proof has not been anchored on Fabric")
	default:
		result.AssuranceLevel = "invalid"
	}
	return result, nil
}

func appendVerificationMessage(current, message string) string {
	if current == "" {
		return message
	}
	return current + "; " + message
}

// signWithPQCKey decrypts the private key only for the duration of one signature.
func signWithPQCKey(key *models.PQCKey, message, context []byte) ([]byte, error) {
	algorithm, err := lookupPQCAlgorithm(key.Algorithm)
	if err != nil {
		return nil, err
	}
	privateBytes, err := decryptPQCPrivateKey(key)
	if err != nil {
		return nil, err
	}
	defer clear(privateBytes)
	return algorithm.sign(privateBytes, message, context)
}

func decryptPQCPrivateKey(key *models.PQCKey) ([]byte, error) {
	masterKey, err := loadPQCMasterKey()
	if err != nil {
		return nil, err
	}
	defer clear(masterKey)
	ciphertext, err := base64.StdEncoding.DecodeString(key.EncryptedPrivateKey)
	if err != nil {
		return nil, errors.New("encrypted PQC private key is corrupted")
	}
	nonce, err := base64.StdEncoding.DecodeString(key.PrivateKeyNonce)
	if err != nil {
		return nil, errors.New("PQC private key nonce is corrupted")
	}
	privateBytes, err := decryptPrivateKey(masterKey, ciphertext, nonce, keyAAD(key.ID, key.UniversityID))
	if err != nil {
		return nil, fmt.Errorf("decrypt PQC private key: %w", err)
	}
	return privateBytes, nil
}

func marshalCredentialManifest(diploma *models.EDiploma, signedAt time.Time) ([]byte, string, error) {
	manifest := models.PQCCredentialManifest{
		Schema:             "urn:pqc-ediploma:schema:proof:v1",
		CredentialID:       diploma.ID.Hex(),
		UniversityID:       diploma.UniversityID.Hex(),
		FacultyID:          diploma.FacultyID.Hex(),
		StudentCode:        diploma.StudentCode,
		FullName:           diploma.FullName,
		CertificateType:    diploma.CertificateType,
		Course:             diploma.Course,
		EducationType:      diploma.EducationType,
		GPA:                fmt.Sprintf("%.2f", diploma.GPA),
		GraduationRank:     diploma.GraduationRank,
		IssueDate:          diploma.IssueDate.UTC().Format("2006-01-02"),
		SerialNumber:       diploma.SerialNumber,
		RegistrationNumber: diploma.RegistrationNumber,
		FileSHA256:         strings.ToLower(diploma.EDiplomaFileHash),
		SignedAt:           signedAt.UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(data)
	return data, hex.EncodeToString(digest[:]), nil
}

func keyWasValidAt(key *models.PQCKey, signedAt time.Time) bool {
	if key.ActivatedAt == nil || signedAt.Before(*key.ActivatedAt) {
		return false
	}
	if key.RetiredAt != nil && !signedAt.Before(*key.RetiredAt) {
		return false
	}
	if key.RevokedAt != nil && !signedAt.Before(*key.RevokedAt) {
		return false
	}
	if key.CompromiseEffectiveAt != nil && !signedAt.Before(*key.CompromiseEffectiveAt) {
		return false
	}
	return true
}

func loadPQCMasterKey() ([]byte, error) {
	encoded := strings.TrimSpace(os.Getenv("PQC_MASTER_KEY"))
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != 32 {
		return nil, ErrPQCNotConfigured
	}
	return key, nil
}

func encryptPrivateKey(key, plaintext, aad []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, aad), nonce, nil
}

func decryptPrivateKey(key, ciphertext, nonce, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	return gcm.Open(nil, nonce, ciphertext, aad)
}

func keyAAD(keyID, universityID primitive.ObjectID) []byte {
	return []byte("PQC-EDIPLOMA-KEY-V1:" + universityID.Hex() + ":" + keyID.Hex())
}
