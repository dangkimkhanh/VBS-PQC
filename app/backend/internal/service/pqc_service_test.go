package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSignedManifestSurvivesBSONRoundTrip(t *testing.T) {
	signedAt := time.Date(2026, 9, 16, 12, 0, 0, 123456789, time.UTC).Truncate(time.Millisecond)
	d := models.EDiploma{ID: primitive.NewObjectID(), IssueDate: signedAt, EDiplomaFileHash: "original", PQCProof: &models.PQCProof{SignedAt: signedAt}}
	before, hash, err := marshalCredentialManifest(&d, signedAt)
	if err != nil {
		t.Fatal(err)
	}
	pub, priv, err := mldsa65.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, mldsa65.SignatureSize)
	if err = mldsa65.SignTo(priv, before, []byte(models.PQCSignatureContext), true, sig); err != nil {
		t.Fatal(err)
	}
	encoded, err := bson.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var restored models.EDiploma
	if err = bson.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	after, nextHash, err := marshalCredentialManifest(&restored, restored.PQCProof.SignedAt)
	if err != nil {
		t.Fatal(err)
	}
	if hash != nextHash || !mldsa65.Verify(pub, after, []byte(models.PQCSignatureContext), sig) {
		t.Fatal("stored signature no longer verifies")
	}
	restored.EDiplomaFileHash = "tampered"
	altered, _, _ := marshalCredentialManifest(&restored, restored.PQCProof.SignedAt)
	if mldsa65.Verify(pub, altered, []byte(models.PQCSignatureContext), sig) {
		t.Fatal("tampered PDF accepted")
	}
}

func TestPQCPrivateKeyEncryptionAndSignatureRoundTrip(t *testing.T) {
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := mldsa65.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("test-key-context")
	ciphertext, nonce, err := encryptPrivateKey(masterKey, privateKey.Bytes(), aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, privateKey.Bytes()) {
		t.Fatal("encrypted private key contains plaintext key material")
	}
	decoded, err := decryptPrivateKey(masterKey, ciphertext, nonce, aad)
	if err != nil {
		t.Fatal(err)
	}
	var restored mldsa65.PrivateKey
	if err := restored.UnmarshalBinary(decoded); err != nil {
		t.Fatalf("restore private key: %v", err)
	}
	message := []byte("credential manifest")
	signature := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(&restored, message, []byte(models.PQCSignatureContext), true, signature); err != nil {
		t.Fatal(err)
	}
	if !mldsa65.Verify(publicKey, message, []byte(models.PQCSignatureContext), signature) {
		t.Fatal("ML-DSA-65 signature did not verify")
	}
	if mldsa65.Verify(publicKey, []byte("modified"), []byte(models.PQCSignatureContext), signature) {
		t.Fatal("modified message unexpectedly verified")
	}
}

func TestCredentialManifestIsDeterministicAndBindsFileHash(t *testing.T) {
	diploma := &models.EDiploma{
		ID:                 primitive.NewObjectID(),
		UniversityID:       primitive.NewObjectID(),
		FacultyID:          primitive.NewObjectID(),
		StudentCode:        "AT180001",
		FullName:           "Nguyen Van A",
		CertificateType:    "Bachelor",
		Course:             "AT18",
		EducationType:      "Full-time",
		GPA:                3.75,
		GraduationRank:     "Good",
		IssueDate:          time.Date(2026, 8, 1, 15, 30, 0, 0, time.FixedZone("ICT", 7*60*60)),
		SerialNumber:       "SER-001",
		RegistrationNumber: "REG-001",
		EDiplomaFileHash:   "ABCDEF",
	}
	signedAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	first, firstHash, err := marshalCredentialManifest(diploma, signedAt)
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := marshalCredentialManifest(diploma, signedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || firstHash != secondHash {
		t.Fatal("credential manifest is not deterministic")
	}
	diploma.EDiplomaFileHash = "modified"
	_, modifiedHash, err := marshalCredentialManifest(diploma, signedAt)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == modifiedHash {
		t.Fatal("file hash modification was not bound into the manifest")
	}
	diploma.EDiplomaFileHash = "ABCDEF"
	_, changedTimeHash, err := marshalCredentialManifest(diploma, signedAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == changedTimeHash {
		t.Fatal("signing time modification was not bound into the manifest")
	}
}

func TestHistoricalKeyValidity(t *testing.T) {
	activated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	signed := activated.Add(24 * time.Hour)
	revoked := signed.Add(24 * time.Hour)
	key := &models.PQCKey{ActivatedAt: &activated, RevokedAt: &revoked, Status: models.PQCKeyRevoked}
	if !keyWasValidAt(key, signed) {
		t.Fatal("ordinary later revocation should preserve a historical signature")
	}
	if keyWasValidAt(key, revoked.Add(time.Second)) {
		t.Fatal("signature at or after revocation must be invalid")
	}
	compromised := signed.Add(-time.Hour)
	key.CompromiseEffectiveAt = &compromised
	if keyWasValidAt(key, signed) {
		t.Fatal("signature after compromise-effective time must be invalid")
	}
}

func TestPQCBlockchainAnchorBindsProofKeyAndFile(t *testing.T) {
	diploma := &models.EDiploma{
		ID:               primitive.NewObjectID(),
		UniversityID:     primitive.NewObjectID(),
		EDiplomaFileHash: strings.Repeat("a", 64),
		PQCProof: &models.PQCProof{
			AnchorVersion:        models.PQCAnchorVersion,
			Algorithm:            models.PQCAlgorithmMLDSA65,
			KeyID:                primitive.NewObjectID().Hex(),
			PublicKeyFingerprint: strings.Repeat("b", 64),
			ManifestHash:         strings.Repeat("c", 64),
			Signature:            base64.StdEncoding.EncodeToString([]byte("signature")),
		},
	}
	original, err := hashEDiplomaInfo(diploma)
	if err != nil {
		t.Fatal(err)
	}
	diploma.PQCProof.PublicKeyFingerprint = strings.Repeat("d", 64)
	changedKey, err := hashEDiplomaInfo(diploma)
	if err != nil {
		t.Fatal(err)
	}
	if original == changedKey {
		t.Fatal("public-key fingerprint is not bound to the blockchain leaf")
	}
	diploma.PQCProof.PublicKeyFingerprint = strings.Repeat("b", 64)
	diploma.EDiplomaFileHash = strings.Repeat("e", 64)
	changedFile, err := hashEDiplomaInfo(diploma)
	if err != nil {
		t.Fatal(err)
	}
	if original == changedFile {
		t.Fatal("PDF hash is not bound to the blockchain leaf")
	}
}

func TestImmutableMerkleBatchIDRecognition(t *testing.T) {
	universityID := primitive.NewObjectID()
	root := strings.Repeat("a", 64)
	if !isImmutableMerkleBatchID("EDIP-"+universityID.Hex()+"-"+root, universityID) {
		t.Fatal("new root-addressed batch ID was not recognized")
	}
	if isImmutableMerkleBatchID("EDIP-"+universityID.Hex()+"-faculty-course", universityID) {
		t.Fatal("legacy filter-addressed batch ID was recognized as immutable")
	}
}
