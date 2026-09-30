package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-contract-api-go/v2/metadata"
)

const (
	CertificateStatusActive  = "ACTIVE"
	CertificateStatusRevoked = "REVOKED"
	CertificateStatusDeleted = "DELETED"
	certificateHashIndex     = "certHash~certID"
)

type CertificateOnChain struct {
	CertHash            string `json:"cert_hash"`
	HashFile            string `json:"hash_file"`
	UniversitySignature string `json:"university_signature"`
	DateOfIssuing       string `json:"date_of_issuing"`
	SerialNumber        string `json:"serial_number"`
	RegNo               string `json:"registration_number"`
	CertID              string `json:"cert_id"`
	Version             int    `json:"version"`
	UpdatedDate         string `json:"updated_date"`
	Status              string `json:"status"`
	IssuerMSP           string `json:"issuer_msp"`
	IssuerID            string `json:"issuer_id"`
	CreatedAt           string `json:"created_at"`
	LastTxID            string `json:"last_tx_id"`
	RevokedAt           string `json:"revoked_at"`
	RevokedBy           string `json:"revoked_by"`
	RevocationReason    string `json:"revocation_reason"`
	DeletedAt           string `json:"deleted_at"`
	DeletedBy           string `json:"deleted_by"`
	DeletionReason      string `json:"deletion_reason"`
}

type CertificateHistoryEntry struct {
	TxID      string              `json:"tx_id"`
	Timestamp string              `json:"timestamp"`
	IsDelete  bool                `json:"is_delete"`
	Value     *CertificateOnChain `json:"value,omitempty"`
}

type CertificateStatusResponse struct {
	CertID           string `json:"cert_id"`
	Status           string `json:"status"`
	Version          int    `json:"version"`
	LastTxID         string `json:"last_tx_id"`
	UpdatedDate      string `json:"updated_date"`
	RevokedAt        string `json:"revoked_at"`
	RevokedBy        string `json:"revoked_by"`
	RevocationReason string `json:"revocation_reason"`
	DeletedAt        string `json:"deleted_at"`
	DeletedBy        string `json:"deleted_by"`
	DeletionReason   string `json:"deletion_reason"`
}

type CertificateBatchOnChain struct {
	BatchID           string `json:"batch_id"`
	UniversityID      string `json:"university_id"`
	FacultyID         string `json:"faculty_id"`
	CertificateType   string `json:"certificate_type"`
	Course            string `json:"course"`
	AggregateInfoHash string `json:"aggregate_info_hash"`
	AggregateFileHash string `json:"aggregate_file_hash"`
	Count             int    `json:"count"`
	TxID              string `json:"tx_id"`
}
type EDiplomaBatchOnChain struct {
	BatchID           string                     `json:"batch_id"`
	UniversityID      string                     `json:"university_id"`
	FacultyID         string                     `json:"faculty_id"`
	CertificateType   string                     `json:"certificate_type"`
	Course            string                     `json:"course"`
	AggregateInfoHash string                     `json:"aggregate_info_hash"`
	AggregateFileHash string                     `json:"aggregate_file_hash"`
	Count             int                        `json:"count"`
	PQCTransaction    *PQCTransactionAttestation `json:"pqc_transaction,omitempty"`
	TxID              string                     `json:"tx_id"`
	CreatedAt         string                     `json:"created_at"`
	IssuerMSP         string                     `json:"issuer_msp"`
	IssuerID          string                     `json:"issuer_id"`
}

type PQCTransactionAttestation struct {
	Version              int    `json:"version"`
	Algorithm            string `json:"algorithm"`
	Context              string `json:"context"`
	KeyID                string `json:"key_id"`
	PublicKey            string `json:"public_key"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
	EnvelopeHash         string `json:"envelope_hash"`
	Signature            string `json:"signature"`
	SignedAt             string `json:"signed_at"`
	Nonce                string `json:"nonce"`
}

type PQCTransactionEnvelope struct {
	Version           int    `json:"version"`
	TransactionType   string `json:"transaction_type"`
	BatchID           string `json:"batch_id"`
	UniversityID      string `json:"university_id"`
	FacultyID         string `json:"faculty_id"`
	CertificateType   string `json:"certificate_type"`
	Course            string `json:"course"`
	AggregateInfoHash string `json:"aggregate_info_hash"`
	AggregateFileHash string `json:"aggregate_file_hash"`
	Count             int    `json:"count"`
	KeyID             string `json:"key_id"`
	SignedAt          string `json:"signed_at"`
	Nonce             string `json:"nonce"`
}

type CertificateTransactionContext struct {
	contractapi.TransactionContext
}

type CertificateContract struct {
	contractapi.Contract
	info metadata.InfoMetadata
}

func (c *CertificateContract) GetName() string {
	return "CertificateContract"
}

func (c *CertificateContract) GetInfo() metadata.InfoMetadata {
	return c.info
}

func (c *CertificateContract) GetTransactionContextHandler() contractapi.SettableTransactionContextInterface {
	return new(CertificateTransactionContext)
}

func (c *CertificateContract) GetEvaluateTransactions() []string {
	return []string{
		"ReadCertificate",
		"CertificateExists",
		"GetCertificateHistory",
		"GetCertificateByHash",
		"GetCertificateStatus",
		"ReadEDiplomaBatch",
		"ReadCertificateBatch",
		"ReadEDiplomaRevocation",
		"GetEDiplomaRevocation",
	}
}

func (c *CertificateContract) IssueEDiplomaBatch(ctx contractapi.TransactionContextInterface, batchJSON string) (string, error) {
	var batch EDiplomaBatchOnChain
	if err := json.Unmarshal([]byte(batchJSON), &batch); err != nil {
		return "", fmt.Errorf("failed to unmarshal batch JSON: %v", err)
	}
	batch.BatchID = strings.TrimSpace(batch.BatchID)
	batch.UniversityID = strings.TrimSpace(batch.UniversityID)
	batch.AggregateInfoHash = normalizeHash(batch.AggregateInfoHash)
	batch.AggregateFileHash = normalizeHash(batch.AggregateFileHash)
	if batch.BatchID == "" || batch.UniversityID == "" {
		return "", fmt.Errorf("batch_id and university_id are required")
	}
	if len(batch.AggregateInfoHash) != 64 {
		return "", fmt.Errorf("aggregate_info_hash must be a SHA-256 hash")
	}
	if _, err := hex.DecodeString(batch.AggregateInfoHash); err != nil {
		return "", fmt.Errorf("aggregate_info_hash must be hexadecimal")
	}
	if batch.Count <= 0 {
		return "", fmt.Errorf("count must be greater than zero")
	}
	if batch.PQCTransaction == nil {
		return "", fmt.Errorf("PQC transaction attestation is required")
	}
	if err := verifyPQCTransactionAttestation(&batch); err != nil {
		return "", err
	}

	exists, err := c.EDiplomaBatchExists(ctx, batch.BatchID)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("batch %s already exists", batch.BatchID)
	}

	// Lưu xuống world state
	issuerMSP, issuerID, err := getInvoker(ctx)
	if err != nil {
		return "", err
	}
	createdAt, err := getTransactionTime(ctx)
	if err != nil {
		return "", err
	}
	batch.IssuerMSP = issuerMSP
	batch.IssuerID = issuerID
	batch.CreatedAt = createdAt
	batch.TxID = ctx.GetStub().GetTxID()
	bytes, err := json.Marshal(batch)
	if err != nil {
		return "", err
	}
	if err := ctx.GetStub().PutState(batch.BatchID, bytes); err != nil {
		return "", err
	}

	// Emit event
	eventPayload := map[string]string{
		"batch_id": batch.BatchID,
		"tx_id":    batch.TxID,
	}
	eventBytes, _ := json.Marshal(eventPayload)
	ctx.GetStub().SetEvent("EDiplomaBatchIssued", eventBytes)

	return batch.TxID, nil
}

// EDiplomaBatchExists check batchID đã tồn tại chưa
func (c *CertificateContract) EDiplomaBatchExists(ctx contractapi.TransactionContextInterface, batchID string) (bool, error) {
	data, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return data != nil, nil
}

// verifyMLDSA checks a FIPS 204 signature for one of the supported parameter
// sets. ok is false when the algorithm name is not supported.
func verifyMLDSA(algorithm string, publicBytes, message, context, signature []byte) (valid, ok bool) {
	switch algorithm {
	case "ML-DSA-44":
		var pk mldsa44.PublicKey
		return pk.UnmarshalBinary(publicBytes) == nil && mldsa44.Verify(&pk, message, context, signature), true
	case "ML-DSA-65":
		var pk mldsa65.PublicKey
		return pk.UnmarshalBinary(publicBytes) == nil && mldsa65.Verify(&pk, message, context, signature), true
	case "ML-DSA-87":
		var pk mldsa87.PublicKey
		return pk.UnmarshalBinary(publicBytes) == nil && mldsa87.Verify(&pk, message, context, signature), true
	}
	return false, false
}

func verifyPQCTransactionAttestation(batch *EDiplomaBatchOnChain) error {
	attestation := batch.PQCTransaction
	if attestation == nil {
		return fmt.Errorf("PQC transaction attestation is required")
	}
	if _, supported := verifyMLDSA(attestation.Algorithm, nil, nil, nil, nil); attestation.Version != 1 || !supported ||
		attestation.Context != "VBS-PQC-FABRIC-TX-V1" ||
		strings.TrimSpace(attestation.KeyID) == "" ||
		strings.TrimSpace(attestation.PublicKey) == "" ||
		len(attestation.EnvelopeHash) != 64 ||
		strings.TrimSpace(attestation.Nonce) == "" ||
		strings.TrimSpace(attestation.SignedAt) == "" {
		return fmt.Errorf("invalid PQC transaction attestation metadata")
	}

	publicBytes, err := base64.StdEncoding.DecodeString(attestation.PublicKey)
	if err != nil {
		return fmt.Errorf("PQC transaction public key must be base64")
	}
	fingerprint := sha256.Sum256(publicBytes)
	if !strings.EqualFold(hex.EncodeToString(fingerprint[:]), attestation.PublicKeyFingerprint) {
		return fmt.Errorf("PQC transaction public key fingerprint mismatch")
	}

	envelope := pqcTransactionEnvelope(batch, attestation.KeyID, attestation.SignedAt, attestation.Nonce)
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal PQC transaction envelope: %v", err)
	}
	digest := sha256.Sum256(envelopeBytes)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), attestation.EnvelopeHash) {
		return fmt.Errorf("PQC transaction envelope hash mismatch")
	}

	signature, err := base64.StdEncoding.DecodeString(attestation.Signature)
	if err != nil {
		return fmt.Errorf("PQC transaction signature must be base64")
	}
	if valid, _ := verifyMLDSA(attestation.Algorithm, publicBytes, envelopeBytes, []byte(attestation.Context), signature); !valid {
		return fmt.Errorf("PQC transaction signature verification failed")
	}
	return nil
}

func pqcTransactionEnvelope(batch *EDiplomaBatchOnChain, keyID, signedAt, nonce string) PQCTransactionEnvelope {
	return PQCTransactionEnvelope{
		Version:           1,
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
		SignedAt:          signedAt,
		Nonce:             nonce,
	}
}

func (c *CertificateContract) IssueCertificate(ctx *CertificateTransactionContext, certificateJSON string) (string, error) {
	var certificate CertificateOnChain
	if err := json.Unmarshal([]byte(certificateJSON), &certificate); err != nil {
		return "", fmt.Errorf("failed to unmarshal certificate JSON: %v", err)
	}

	certificate.CertID = strings.TrimSpace(certificate.CertID)
	certificate.CertHash = normalizeHash(certificate.CertHash)
	if certificate.CertID == "" {
		return "", fmt.Errorf("cert_id is required")
	}
	if certificate.CertHash == "" {
		return "", fmt.Errorf("cert_hash is required")
	}

	exists, err := c.CertificateExists(ctx, certificate.CertID)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("certificate %s already exists", certificate.CertID)
	}

	duplicate, err := c.lookupCertificateByHash(ctx, certificate.CertHash)
	if err != nil {
		return "", err
	}
	if duplicate != nil {
		return "", fmt.Errorf("certificate hash already belongs to certificate %s", duplicate.CertID)
	}

	mspID, clientID, err := getInvoker(ctx)
	if err != nil {
		return "", err
	}
	timestamp, err := getTransactionTime(ctx)
	if err != nil {
		return "", err
	}

	txID := ctx.GetStub().GetTxID()
	certificate.Status = CertificateStatusActive
	certificate.Version = 1
	certificate.IssuerMSP = mspID
	certificate.IssuerID = clientID
	certificate.CreatedAt = timestamp
	certificate.UpdatedDate = timestamp
	certificate.LastTxID = txID
	certificate.RevokedAt = ""
	certificate.RevokedBy = ""
	certificate.RevocationReason = ""
	certificate.DeletedAt = ""
	certificate.DeletedBy = ""
	certificate.DeletionReason = ""

	if err := c.putCertificate(ctx, &certificate); err != nil {
		return "", err
	}

	eventPayload := map[string]string{
		"certID": certificate.CertID,
		"txID":   txID,
		"status": certificate.Status,
	}
	eventBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return "", err
	}
	if err := ctx.GetStub().SetEvent("CertificateIssued", eventBytes); err != nil {
		return "", fmt.Errorf("failed to emit CertificateIssued event: %v", err)
	}

	return txID, nil
}

func (c *CertificateContract) ReadCertificate(ctx *CertificateTransactionContext, certID string) (*CertificateOnChain, error) {
	bytes, err := ctx.GetStub().GetState(certID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if bytes == nil {
		return nil, fmt.Errorf("certificate %s does not exist", certID)
	}

	var certificate CertificateOnChain
	if err := json.Unmarshal(bytes, &certificate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal certificate: %v", err)
	}
	normalizeCertificate(&certificate)
	return &certificate, nil
}

func (c *CertificateContract) CertificateExists(ctx *CertificateTransactionContext, certID string) (bool, error) {
	bytes, err := ctx.GetStub().GetState(certID)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return bytes != nil, nil
}
func (c *CertificateContract) UpdateCertificate(ctx *CertificateTransactionContext, certificateJSON string) error {
	var updated CertificateOnChain
	if err := json.Unmarshal([]byte(certificateJSON), &updated); err != nil {
		return fmt.Errorf("failed to unmarshal certificate: %v", err)
	}
	updated.CertID = strings.TrimSpace(updated.CertID)
	updated.CertHash = normalizeHash(updated.CertHash)
	if updated.CertID == "" || updated.CertHash == "" {
		return fmt.Errorf("cert_id and cert_hash are required")
	}

	current, err := c.ReadCertificate(ctx, updated.CertID)
	if err != nil {
		return err
	}
	if current.Status != CertificateStatusActive {
		return fmt.Errorf("certificate %s cannot be updated while status is %s", updated.CertID, current.Status)
	}

	mspID, clientID, err := c.authorizeMutation(ctx, current)
	if err != nil {
		return err
	}

	if updated.CertHash != current.CertHash {
		duplicate, err := c.lookupCertificateByHash(ctx, updated.CertHash)
		if err != nil {
			return err
		}
		if duplicate != nil && duplicate.CertID != updated.CertID {
			return fmt.Errorf("certificate hash already belongs to certificate %s", duplicate.CertID)
		}
		if err := c.deleteHashIndex(ctx, current.CertHash, current.CertID); err != nil {
			return err
		}
	}

	timestamp, err := getTransactionTime(ctx)
	if err != nil {
		return err
	}
	updated.Version = current.Version + 1
	updated.Status = CertificateStatusActive
	updated.IssuerMSP = current.IssuerMSP
	updated.IssuerID = current.IssuerID
	if updated.IssuerMSP == "" {
		updated.IssuerMSP = mspID
	}
	if updated.IssuerID == "" {
		updated.IssuerID = clientID
	}
	updated.CreatedAt = current.CreatedAt
	updated.UpdatedDate = timestamp
	updated.LastTxID = ctx.GetStub().GetTxID()
	updated.RevokedAt = ""
	updated.RevokedBy = ""
	updated.RevocationReason = ""
	updated.DeletedAt = ""
	updated.DeletedBy = ""
	updated.DeletionReason = ""

	if err := c.putCertificate(ctx, &updated); err != nil {
		return err
	}

	return emitCertificateEvent(ctx, "CertificateUpdated", &updated, "")
}

// RevokeCertificate permanently marks an active certificate as revoked while
// retaining the certificate and its complete audit trail in world state.
func (c *CertificateContract) RevokeCertificate(ctx *CertificateTransactionContext, certID, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("revocation reason is required")
	}
	if len(reason) > 1024 {
		return "", fmt.Errorf("revocation reason must not exceed 1024 characters")
	}

	certificate, err := c.ReadCertificate(ctx, strings.TrimSpace(certID))
	if err != nil {
		return "", err
	}
	if certificate.Status != CertificateStatusActive {
		return "", fmt.Errorf("certificate %s cannot be revoked while status is %s", certificate.CertID, certificate.Status)
	}

	mspID, clientID, err := c.authorizeMutation(ctx, certificate)
	if err != nil {
		return "", err
	}
	timestamp, err := getTransactionTime(ctx)
	if err != nil {
		return "", err
	}

	certificate.Status = CertificateStatusRevoked
	certificate.Version++
	certificate.RevokedAt = timestamp
	certificate.RevokedBy = clientID
	certificate.RevocationReason = reason
	certificate.UpdatedDate = timestamp
	certificate.LastTxID = ctx.GetStub().GetTxID()
	claimLegacyOwnership(certificate, mspID, clientID)

	if err := c.putCertificate(ctx, certificate); err != nil {
		return "", err
	}
	if err := emitCertificateEvent(ctx, "CertificateRevoked", certificate, reason); err != nil {
		return "", err
	}
	return certificate.LastTxID, nil
}

// DeleteCertificate performs a logical delete. Blockchain records are not
// physically removed so that revocation, deletion and ownership remain auditable.
func (c *CertificateContract) DeleteCertificate(ctx *CertificateTransactionContext, certID, reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("deletion reason is required")
	}
	if len(reason) > 1024 {
		return "", fmt.Errorf("deletion reason must not exceed 1024 characters")
	}

	certificate, err := c.ReadCertificate(ctx, strings.TrimSpace(certID))
	if err != nil {
		return "", err
	}
	if certificate.Status == CertificateStatusDeleted {
		return "", fmt.Errorf("certificate %s is already deleted", certificate.CertID)
	}

	mspID, clientID, err := c.authorizeMutation(ctx, certificate)
	if err != nil {
		return "", err
	}
	timestamp, err := getTransactionTime(ctx)
	if err != nil {
		return "", err
	}

	certificate.Status = CertificateStatusDeleted
	certificate.Version++
	certificate.DeletedAt = timestamp
	certificate.DeletedBy = clientID
	certificate.DeletionReason = reason
	certificate.UpdatedDate = timestamp
	certificate.LastTxID = ctx.GetStub().GetTxID()
	claimLegacyOwnership(certificate, mspID, clientID)

	if err := c.putCertificate(ctx, certificate); err != nil {
		return "", err
	}
	if err := emitCertificateEvent(ctx, "CertificateDeleted", certificate, reason); err != nil {
		return "", err
	}
	return certificate.LastTxID, nil
}

func (c *CertificateContract) GetCertificateStatus(ctx *CertificateTransactionContext, certID string) (*CertificateStatusResponse, error) {
	certificate, err := c.ReadCertificate(ctx, strings.TrimSpace(certID))
	if err != nil {
		return nil, err
	}
	return &CertificateStatusResponse{
		CertID:           certificate.CertID,
		Status:           certificate.Status,
		Version:          certificate.Version,
		LastTxID:         certificate.LastTxID,
		UpdatedDate:      certificate.UpdatedDate,
		RevokedAt:        certificate.RevokedAt,
		RevokedBy:        certificate.RevokedBy,
		RevocationReason: certificate.RevocationReason,
		DeletedAt:        certificate.DeletedAt,
		DeletedBy:        certificate.DeletedBy,
		DeletionReason:   certificate.DeletionReason,
	}, nil
}

func (c *CertificateContract) GetCertificateByHash(ctx *CertificateTransactionContext, certHash string) (*CertificateOnChain, error) {
	certHash = normalizeHash(certHash)
	if certHash == "" {
		return nil, fmt.Errorf("cert_hash is required")
	}

	certificate, err := c.lookupCertificateByHash(ctx, certHash)
	if err != nil {
		return nil, err
	}
	if certificate == nil {
		return nil, fmt.Errorf("certificate with hash %s does not exist", certHash)
	}
	return certificate, nil
}

func (c *CertificateContract) GetCertificateHistory(ctx *CertificateTransactionContext, certID string) ([]CertificateHistoryEntry, error) {
	certID = strings.TrimSpace(certID)
	if certID == "" {
		return nil, fmt.Errorf("cert_id is required")
	}
	exists, err := c.CertificateExists(ctx, certID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("certificate %s does not exist", certID)
	}

	iterator, err := ctx.GetStub().GetHistoryForKey(certID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history for certificate %s: %v", certID, err)
	}
	defer iterator.Close()

	history := make([]CertificateHistoryEntry, 0)
	for iterator.HasNext() {
		modification, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to read history for certificate %s: %v", certID, err)
		}
		entry := CertificateHistoryEntry{
			TxID:     modification.TxId,
			IsDelete: modification.IsDelete,
		}
		if modification.Timestamp != nil {
			entry.Timestamp = modification.Timestamp.AsTime().UTC().Format(time.RFC3339Nano)
		}
		if !modification.IsDelete && len(modification.Value) > 0 {
			var certificate CertificateOnChain
			if err := json.Unmarshal(modification.Value, &certificate); err != nil {
				return nil, fmt.Errorf("failed to unmarshal history value for certificate %s: %v", certID, err)
			}
			normalizeCertificate(&certificate)
			entry.Value = &certificate
		}
		history = append(history, entry)
	}
	return history, nil
}

func (c *CertificateContract) putCertificate(ctx contractapi.TransactionContextInterface, certificate *CertificateOnChain) error {
	certificate.CertHash = normalizeHash(certificate.CertHash)
	bytes, err := json.Marshal(certificate)
	if err != nil {
		return fmt.Errorf("failed to marshal certificate: %v", err)
	}
	if err := ctx.GetStub().PutState(certificate.CertID, bytes); err != nil {
		return fmt.Errorf("failed to write certificate %s: %v", certificate.CertID, err)
	}
	indexKey, err := ctx.GetStub().CreateCompositeKey(certificateHashIndex, []string{certificate.CertHash, certificate.CertID})
	if err != nil {
		return fmt.Errorf("failed to create certificate hash index: %v", err)
	}
	if err := ctx.GetStub().PutState(indexKey, []byte{0}); err != nil {
		return fmt.Errorf("failed to write certificate hash index: %v", err)
	}
	return nil
}

func (c *CertificateContract) deleteHashIndex(ctx contractapi.TransactionContextInterface, certHash, certID string) error {
	indexKey, err := ctx.GetStub().CreateCompositeKey(certificateHashIndex, []string{normalizeHash(certHash), certID})
	if err != nil {
		return fmt.Errorf("failed to create certificate hash index: %v", err)
	}
	if err := ctx.GetStub().DelState(indexKey); err != nil {
		return fmt.Errorf("failed to delete certificate hash index: %v", err)
	}
	return nil
}

func (c *CertificateContract) lookupCertificateByHash(ctx contractapi.TransactionContextInterface, certHash string) (*CertificateOnChain, error) {
	certHash = normalizeHash(certHash)
	iterator, err := ctx.GetStub().GetStateByPartialCompositeKey(certificateHashIndex, []string{certHash})
	if err != nil {
		return nil, fmt.Errorf("failed to query certificate hash index: %v", err)
	}
	defer iterator.Close()

	var found *CertificateOnChain
	for iterator.HasNext() {
		item, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate hash index: %v", err)
		}
		_, parts, err := ctx.GetStub().SplitCompositeKey(item.Key)
		if err != nil || len(parts) != 2 {
			return nil, fmt.Errorf("invalid certificate hash index key")
		}
		certificate, err := c.readCertificateFromContext(ctx, parts[1])
		if err != nil {
			return nil, err
		}
		if found != nil && found.CertID != certificate.CertID {
			return nil, fmt.Errorf("multiple certificates found for hash %s", certHash)
		}
		found = certificate
	}
	if found != nil {
		return found, nil
	}

	// Backward-compatible fallback for certificates created before the composite
	// index was introduced. A later mutation will backfill their index entry.
	rangeIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to scan legacy certificates: %v", err)
	}
	defer rangeIterator.Close()
	for rangeIterator.HasNext() {
		item, err := rangeIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to scan legacy certificates: %v", err)
		}
		var certificate CertificateOnChain
		if err := json.Unmarshal(item.Value, &certificate); err != nil || certificate.CertID == "" {
			continue
		}
		if normalizeHash(certificate.CertHash) != certHash {
			continue
		}
		normalizeCertificate(&certificate)
		if found != nil && found.CertID != certificate.CertID {
			return nil, fmt.Errorf("multiple certificates found for hash %s", certHash)
		}
		copy := certificate
		found = &copy
	}
	return found, nil
}

func (c *CertificateContract) readCertificateFromContext(ctx contractapi.TransactionContextInterface, certID string) (*CertificateOnChain, error) {
	bytes, err := ctx.GetStub().GetState(certID)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if bytes == nil {
		return nil, fmt.Errorf("certificate %s does not exist", certID)
	}
	var certificate CertificateOnChain
	if err := json.Unmarshal(bytes, &certificate); err != nil {
		return nil, fmt.Errorf("failed to unmarshal certificate: %v", err)
	}
	normalizeCertificate(&certificate)
	return &certificate, nil
}

func (c *CertificateContract) authorizeMutation(ctx contractapi.TransactionContextInterface, certificate *CertificateOnChain) (string, string, error) {
	mspID, clientID, err := getInvoker(ctx)
	if err != nil {
		return "", "", err
	}
	if certificate.IssuerMSP != "" && certificate.IssuerMSP != mspID {
		return "", "", fmt.Errorf("client MSP %s is not authorized to mutate certificate issued by MSP %s", mspID, certificate.IssuerMSP)
	}
	if certificate.IssuerID == "" || certificate.IssuerID == clientID {
		return mspID, clientID, nil
	}
	value, found, err := ctx.GetClientIdentity().GetAttributeValue("certificateAdmin")
	if err != nil {
		return "", "", fmt.Errorf("failed to read certificateAdmin attribute: %v", err)
	}
	if found && strings.EqualFold(value, "true") {
		return mspID, clientID, nil
	}
	return "", "", fmt.Errorf("client is not the certificate issuer and does not have certificateAdmin=true")
}

func getInvoker(ctx contractapi.TransactionContextInterface) (string, string, error) {
	mspID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", "", fmt.Errorf("failed to get invoker MSP ID: %v", err)
	}
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return "", "", fmt.Errorf("failed to get invoker client ID: %v", err)
	}
	if strings.TrimSpace(mspID) == "" || strings.TrimSpace(clientID) == "" {
		return "", "", fmt.Errorf("invoker identity is incomplete")
	}
	return mspID, clientID, nil
}

func getTransactionTime(ctx contractapi.TransactionContextInterface) (string, error) {
	timestamp, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("failed to get transaction timestamp: %v", err)
	}
	return timestamp.AsTime().UTC().Format(time.RFC3339Nano), nil
}

func normalizeCertificate(certificate *CertificateOnChain) {
	certificate.CertHash = normalizeHash(certificate.CertHash)
	if certificate.Status == "" {
		certificate.Status = CertificateStatusActive
	}
	if certificate.Version == 0 {
		certificate.Version = 1
	}
}

func normalizeHash(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func claimLegacyOwnership(certificate *CertificateOnChain, mspID, clientID string) {
	if certificate.IssuerMSP == "" {
		certificate.IssuerMSP = mspID
	}
	if certificate.IssuerID == "" {
		certificate.IssuerID = clientID
	}
}

func emitCertificateEvent(ctx contractapi.TransactionContextInterface, eventName string, certificate *CertificateOnChain, reason string) error {
	payload := map[string]string{
		"cert_id": certificate.CertID,
		"tx_id":   certificate.LastTxID,
		"status":  certificate.Status,
		"reason":  reason,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal %s event: %v", eventName, err)
	}
	if err := ctx.GetStub().SetEvent(eventName, bytes); err != nil {
		return fmt.Errorf("failed to emit %s event: %v", eventName, err)
	}
	return nil
}

func main() {
	certificateContract := &CertificateContract{
		info: metadata.InfoMetadata{
			Title:   "Certificate Contract",
			Version: "2.0.0",
		},
	}

	chaincode, err := contractapi.NewChaincode(certificateContract)
	if err != nil {
		log.Fatalf("failed to create certificate chaincode: %v", err)
	}

	chaincodeID := strings.TrimSpace(os.Getenv("CHAINCODE_ID"))
	if chaincodeID == "" {
		log.Fatal("CHAINCODE_ID is required for Chaincode-as-a-Service")
	}

	chaincodeAddress := strings.TrimSpace(os.Getenv("CHAINCODE_SERVER_ADDRESS"))
	if chaincodeAddress == "" {
		chaincodeAddress = "0.0.0.0:9999"
	}

	server := &shim.ChaincodeServer{
		CCID:    chaincodeID,
		Address: chaincodeAddress,
		CC:      chaincode,
		TLSProps: shim.TLSProperties{
			Disabled: true,
		},
	}

	log.Printf("starting KMASC chaincode server on %s", chaincodeAddress)
	if err := server.Start(); err != nil {
		log.Fatalf("failed to start chaincode server: %v", err)
	}
}

func (c *CertificateContract) ReadEDiplomaBatch(ctx contractapi.TransactionContextInterface, batchID string) (*EDiplomaBatchOnChain, error) {
	bytes, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch from world state: %v", err)
	}
	if bytes == nil {
		return nil, fmt.Errorf("batch %s does not exist", batchID)
	}

	var batch EDiplomaBatchOnChain
	if err := json.Unmarshal(bytes, &batch); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch: %v", err)
	}
	return &batch, nil
}

func (c *CertificateContract) ReadCertificateBatch(ctx contractapi.TransactionContextInterface, batchID string) (*CertificateBatchOnChain, error) {
	bytes, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate batch from world state: %v", err)
	}
	if bytes == nil {
		return nil, fmt.Errorf("batch %s does not exist", batchID)
	}

	var batch CertificateBatchOnChain
	if err := json.Unmarshal(bytes, &batch); err != nil {
		return nil, fmt.Errorf("failed to unmarshal certificate batch: %v", err)
	}
	return &batch, nil
}

func (c *CertificateContract) IssueCertificateBatch(ctx contractapi.TransactionContextInterface, batchJSON string) (string, error) {
	var batch CertificateBatchOnChain
	if err := json.Unmarshal([]byte(batchJSON), &batch); err != nil {
		return "", fmt.Errorf("failed to unmarshal batch JSON: %v", err)
	}

	if batch.BatchID == "" {
		batch.BatchID = fmt.Sprintf("%s-%s-%s", batch.FacultyID, batch.CertificateType, batch.Course)
	}

	exists, err := c.CertificateBatchExists(ctx, batch.BatchID)
	if err != nil {
		return "", err
	}
	if exists {
		fmt.Printf("Batch %s đã tồn tại, sẽ ghi đè dữ liệu mới\n", batch.BatchID)
	}

	batch.TxID = ctx.GetStub().GetTxID()
	bytes, err := json.Marshal(batch)
	if err != nil {
		return "", err
	}
	if err := ctx.GetStub().PutState(batch.BatchID, bytes); err != nil {
		return "", err
	}

	// Emit event
	eventPayload := map[string]string{
		"batch_id": batch.BatchID,
		"tx_id":    batch.TxID,
	}
	eventBytes, _ := json.Marshal(eventPayload)
	ctx.GetStub().SetEvent("CertificateBatchIssued", eventBytes)

	return batch.TxID, nil
}

func (c *CertificateContract) CertificateBatchExists(ctx contractapi.TransactionContextInterface, batchID string) (bool, error) {
	data, err := ctx.GetStub().GetState(batchID)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return data != nil, nil
}
