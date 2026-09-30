package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

const (
	// revocationContext separates revocation signatures from issuance signatures,
	// so a signed issuance envelope can never be replayed as a revocation.
	revocationContext        = "VBS-PQC-FABRIC-REVOKE-V1"
	revocationIndex          = "ediplomaRevocation~diplomaID"
	anchorDomain             = "PQC-EDIPLOMA-ANCHOR-V1"
	maxRevocationItems       = 500
	revocationRecordIDPrefix = "REVK-"
)

// MerkleProofNode is one sibling on the path from a leaf to the batch root.
type MerkleProofNode struct {
	Hash     string `json:"hash"`
	Position string `json:"position"` // "L" or "R"
}

// RevokedEDiplomaItem proves that the revoked diploma belongs to an anchored batch:
// the chaincode hashes AnchorPayload into the Merkle leaf and walks MerkleProof to
// the root stored for BatchID.
type RevokedEDiplomaItem struct {
	DiplomaID     string            `json:"diploma_id"`
	BatchID       string            `json:"batch_id"`
	AnchorPayload string            `json:"anchor_payload"`
	MerkleProof   []MerkleProofNode `json:"merkle_proof"`
	RevokedAt     string            `json:"revoked_at"`
	ReasonHash    string            `json:"reason_hash"`
}

type EDiplomaRevocationOnChain struct {
	RevocationID   string                     `json:"revocation_id"`
	UniversityID   string                     `json:"university_id"`
	FacultyID      string                     `json:"faculty_id"`
	RoundID        string                     `json:"round_id"`
	ItemsHash      string                     `json:"items_hash"`
	Count          int                        `json:"count"`
	Items          []RevokedEDiplomaItem      `json:"items"`
	PQCTransaction *PQCTransactionAttestation `json:"pqc_transaction,omitempty"`
	TxID           string                     `json:"tx_id"`
	CreatedAt      string                     `json:"created_at"`
	IssuerMSP      string                     `json:"issuer_msp"`
	IssuerID       string                     `json:"issuer_id"`
}

// PQCRevocationEnvelope is the exact byte string the university signs with ML-DSA.
type PQCRevocationEnvelope struct {
	Version         int    `json:"version"`
	TransactionType string `json:"transaction_type"`
	RevocationID    string `json:"revocation_id"`
	UniversityID    string `json:"university_id"`
	FacultyID       string `json:"faculty_id"`
	RoundID         string `json:"round_id"`
	ItemsHash       string `json:"items_hash"`
	Count           int    `json:"count"`
	KeyID           string `json:"key_id"`
	SignedAt        string `json:"signed_at"`
	Nonce           string `json:"nonce"`
}

// EDiplomaRevocationStatus is what a verifier reads for one diploma.
type EDiplomaRevocationStatus struct {
	DiplomaID    string `json:"diploma_id"`
	RevocationID string `json:"revocation_id"`
	BatchID      string `json:"batch_id"`
	RevokedAt    string `json:"revoked_at"`
	ReasonHash   string `json:"reason_hash"`
	TxID         string `json:"tx_id"`
	CreatedAt    string `json:"created_at"`
	IssuerMSP    string `json:"issuer_msp"`
}

type anchorPayloadIdentity struct {
	Domain       string `json:"domain"`
	CredentialID string `json:"credential_id"`
	UniversityID string `json:"university_id"`
}

// RevokeEDiplomaBatch records the revocation of anchored diplomas. The record is
// append-only: a diploma can be revoked on the ledger once and never restored.
func (c *CertificateContract) RevokeEDiplomaBatch(ctx contractapi.TransactionContextInterface, revocationJSON string) (string, error) {
	var revocation EDiplomaRevocationOnChain
	if err := json.Unmarshal([]byte(revocationJSON), &revocation); err != nil {
		return "", fmt.Errorf("failed to unmarshal revocation JSON: %v", err)
	}
	revocation.RevocationID = strings.TrimSpace(revocation.RevocationID)
	revocation.UniversityID = strings.TrimSpace(revocation.UniversityID)
	revocation.ItemsHash = normalizeHash(revocation.ItemsHash)
	if revocation.RevocationID == "" || revocation.UniversityID == "" {
		return "", fmt.Errorf("revocation_id and university_id are required")
	}
	if !strings.HasPrefix(revocation.RevocationID, revocationRecordIDPrefix) {
		return "", fmt.Errorf("revocation_id must start with %s", revocationRecordIDPrefix)
	}
	if len(revocation.Items) == 0 || len(revocation.Items) > maxRevocationItems {
		return "", fmt.Errorf("a revocation must contain between 1 and %d diplomas", maxRevocationItems)
	}
	if revocation.Count != len(revocation.Items) {
		return "", fmt.Errorf("count does not match the number of revoked diplomas")
	}
	itemsHash, err := revocationItemsHash(revocation.Items)
	if err != nil {
		return "", err
	}
	if itemsHash != revocation.ItemsHash {
		return "", fmt.Errorf("items_hash does not match the revoked diplomas")
	}
	if err := verifyPQCRevocationAttestation(&revocation); err != nil {
		return "", err
	}

	existing, err := ctx.GetStub().GetState(revocation.RevocationID)
	if err != nil {
		return "", fmt.Errorf("failed to read from world state: %v", err)
	}
	if existing != nil {
		return "", fmt.Errorf("revocation %s already exists", revocation.RevocationID)
	}

	batches := map[string]*EDiplomaBatchOnChain{}
	seen := map[string]bool{}
	for i := range revocation.Items {
		item := &revocation.Items[i]
		if err := c.checkRevokedItem(ctx, &revocation, item, batches, seen); err != nil {
			return "", fmt.Errorf("diploma %s: %v", item.DiplomaID, err)
		}
	}

	issuerMSP, issuerID, err := getInvoker(ctx)
	if err != nil {
		return "", err
	}
	createdAt, err := getTransactionTime(ctx)
	if err != nil {
		return "", err
	}
	revocation.IssuerMSP = issuerMSP
	revocation.IssuerID = issuerID
	revocation.CreatedAt = createdAt
	revocation.TxID = ctx.GetStub().GetTxID()
	bytes, err := json.Marshal(revocation)
	if err != nil {
		return "", err
	}
	if err := ctx.GetStub().PutState(revocation.RevocationID, bytes); err != nil {
		return "", err
	}
	for _, item := range revocation.Items {
		key, err := ctx.GetStub().CreateCompositeKey(revocationIndex, []string{item.DiplomaID})
		if err != nil {
			return "", fmt.Errorf("failed to create revocation index: %v", err)
		}
		if err := ctx.GetStub().PutState(key, []byte(revocation.RevocationID)); err != nil {
			return "", fmt.Errorf("failed to write revocation index: %v", err)
		}
	}

	eventBytes, _ := json.Marshal(map[string]any{
		"revocation_id": revocation.RevocationID,
		"tx_id":         revocation.TxID,
		"count":         revocation.Count,
	})
	if err := ctx.GetStub().SetEvent("EDiplomaBatchRevoked", eventBytes); err != nil {
		return "", fmt.Errorf("failed to emit EDiplomaBatchRevoked event: %v", err)
	}
	return revocation.TxID, nil
}

func (c *CertificateContract) checkRevokedItem(
	ctx contractapi.TransactionContextInterface,
	revocation *EDiplomaRevocationOnChain,
	item *RevokedEDiplomaItem,
	batches map[string]*EDiplomaBatchOnChain,
	seen map[string]bool,
) error {
	item.DiplomaID = strings.TrimSpace(item.DiplomaID)
	item.BatchID = strings.TrimSpace(item.BatchID)
	item.ReasonHash = normalizeHash(item.ReasonHash)
	if item.DiplomaID == "" || item.BatchID == "" || strings.TrimSpace(item.RevokedAt) == "" {
		return fmt.Errorf("diploma_id, batch_id and revoked_at are required")
	}
	if seen[item.DiplomaID] {
		return fmt.Errorf("listed more than once")
	}
	seen[item.DiplomaID] = true
	if item.MerkleProof == nil {
		item.MerkleProof = []MerkleProofNode{} // a one-leaf batch has an empty proof
	}
	if !isSHA256Hex(item.ReasonHash) {
		return fmt.Errorf("reason_hash must be a SHA-256 hash")
	}

	// The anchored leaf must describe exactly this diploma of this university.
	var identity anchorPayloadIdentity
	if err := json.Unmarshal([]byte(item.AnchorPayload), &identity); err != nil {
		return fmt.Errorf("anchor_payload is not valid JSON")
	}
	if identity.Domain != anchorDomain || identity.CredentialID != item.DiplomaID || identity.UniversityID != revocation.UniversityID {
		return fmt.Errorf("anchor_payload does not belong to this diploma and university")
	}

	batch, ok := batches[item.BatchID]
	if !ok {
		stored, err := c.ReadEDiplomaBatch(ctx, item.BatchID)
		if err != nil {
			return err
		}
		batch = stored
		batches[item.BatchID] = batch
	}
	if batch.UniversityID != revocation.UniversityID {
		return fmt.Errorf("batch %s belongs to another university", item.BatchID)
	}
	leaf := sha256.Sum256([]byte(item.AnchorPayload))
	if !verifyMerkleProof(hex.EncodeToString(leaf[:]), item.MerkleProof, normalizeHash(batch.AggregateInfoHash)) {
		return fmt.Errorf("Merkle proof does not match the root of batch %s", item.BatchID)
	}

	key, err := ctx.GetStub().CreateCompositeKey(revocationIndex, []string{item.DiplomaID})
	if err != nil {
		return fmt.Errorf("failed to create revocation index: %v", err)
	}
	existing, err := ctx.GetStub().GetState(key)
	if err != nil {
		return fmt.Errorf("failed to read revocation index: %v", err)
	}
	if existing != nil {
		return fmt.Errorf("already revoked on the ledger by %s", string(existing))
	}
	return nil
}

// GetEDiplomaRevocation returns the ledger revocation of one diploma, or an error
// containing "is not revoked" when the ledger has none.
func (c *CertificateContract) GetEDiplomaRevocation(ctx contractapi.TransactionContextInterface, diplomaID string) (*EDiplomaRevocationStatus, error) {
	diplomaID = strings.TrimSpace(diplomaID)
	if diplomaID == "" {
		return nil, fmt.Errorf("diploma_id is required")
	}
	key, err := ctx.GetStub().CreateCompositeKey(revocationIndex, []string{diplomaID})
	if err != nil {
		return nil, fmt.Errorf("failed to create revocation index: %v", err)
	}
	revocationID, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read revocation index: %v", err)
	}
	if revocationID == nil {
		return nil, fmt.Errorf("diploma %s is not revoked on the ledger", diplomaID)
	}
	revocation, err := c.ReadEDiplomaRevocation(ctx, string(revocationID))
	if err != nil {
		return nil, err
	}
	for _, item := range revocation.Items {
		if item.DiplomaID == diplomaID {
			return &EDiplomaRevocationStatus{
				DiplomaID:    diplomaID,
				RevocationID: revocation.RevocationID,
				BatchID:      item.BatchID,
				RevokedAt:    item.RevokedAt,
				ReasonHash:   item.ReasonHash,
				TxID:         revocation.TxID,
				CreatedAt:    revocation.CreatedAt,
				IssuerMSP:    revocation.IssuerMSP,
			}, nil
		}
	}
	return nil, fmt.Errorf("revocation index of diploma %s is inconsistent", diplomaID)
}

func (c *CertificateContract) ReadEDiplomaRevocation(ctx contractapi.TransactionContextInterface, revocationID string) (*EDiplomaRevocationOnChain, error) {
	bytes, err := ctx.GetStub().GetState(strings.TrimSpace(revocationID))
	if err != nil {
		return nil, fmt.Errorf("failed to read revocation from world state: %v", err)
	}
	if bytes == nil {
		return nil, fmt.Errorf("revocation %s does not exist", revocationID)
	}
	var revocation EDiplomaRevocationOnChain
	if err := json.Unmarshal(bytes, &revocation); err != nil {
		return nil, fmt.Errorf("failed to unmarshal revocation: %v", err)
	}
	return &revocation, nil
}

func verifyPQCRevocationAttestation(revocation *EDiplomaRevocationOnChain) error {
	attestation := revocation.PQCTransaction
	if attestation == nil {
		return fmt.Errorf("PQC transaction attestation is required")
	}
	if _, supported := verifyMLDSA(attestation.Algorithm, nil, nil, nil, nil); attestation.Version != 1 || !supported ||
		attestation.Context != revocationContext ||
		strings.TrimSpace(attestation.KeyID) == "" ||
		strings.TrimSpace(attestation.PublicKey) == "" ||
		len(attestation.EnvelopeHash) != 64 ||
		strings.TrimSpace(attestation.Nonce) == "" ||
		strings.TrimSpace(attestation.SignedAt) == "" {
		return fmt.Errorf("invalid PQC revocation attestation metadata")
	}
	publicBytes, err := base64.StdEncoding.DecodeString(attestation.PublicKey)
	if err != nil {
		return fmt.Errorf("PQC revocation public key must be base64")
	}
	fingerprint := sha256.Sum256(publicBytes)
	if !strings.EqualFold(hex.EncodeToString(fingerprint[:]), attestation.PublicKeyFingerprint) {
		return fmt.Errorf("PQC revocation public key fingerprint mismatch")
	}
	envelopeBytes, err := json.Marshal(pqcRevocationEnvelope(revocation, attestation.KeyID, attestation.SignedAt, attestation.Nonce))
	if err != nil {
		return fmt.Errorf("marshal PQC revocation envelope: %v", err)
	}
	digest := sha256.Sum256(envelopeBytes)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), attestation.EnvelopeHash) {
		return fmt.Errorf("PQC revocation envelope hash mismatch")
	}
	signature, err := base64.StdEncoding.DecodeString(attestation.Signature)
	if err != nil {
		return fmt.Errorf("PQC revocation signature must be base64")
	}
	if valid, _ := verifyMLDSA(attestation.Algorithm, publicBytes, envelopeBytes, []byte(attestation.Context), signature); !valid {
		return fmt.Errorf("PQC revocation signature verification failed")
	}
	return nil
}

func pqcRevocationEnvelope(revocation *EDiplomaRevocationOnChain, keyID, signedAt, nonce string) PQCRevocationEnvelope {
	return PQCRevocationEnvelope{
		Version:         1,
		TransactionType: "RevokeEDiplomaBatch",
		RevocationID:    revocation.RevocationID,
		UniversityID:    revocation.UniversityID,
		FacultyID:       revocation.FacultyID,
		RoundID:         revocation.RoundID,
		ItemsHash:       revocation.ItemsHash,
		Count:           revocation.Count,
		KeyID:           keyID,
		SignedAt:        signedAt,
		Nonce:           nonce,
	}
}

// revocationItemsHash is SHA-256 over the JSON list of items sorted by diploma ID,
// so the signed envelope commits to every revoked diploma.
func revocationItemsHash(items []RevokedEDiplomaItem) (string, error) {
	sorted := append([]RevokedEDiplomaItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].DiplomaID < sorted[j].DiplomaID })
	bytes, err := json.Marshal(sorted)
	if err != nil {
		return "", fmt.Errorf("marshal revoked diplomas: %v", err)
	}
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:]), nil
}

// verifyMerkleProof mirrors the backend tree: parent = SHA-256(left_hex + right_hex).
func verifyMerkleProof(leaf string, proof []MerkleProofNode, root string) bool {
	hash := leaf
	for _, node := range proof {
		sibling := normalizeHash(node.Hash)
		switch node.Position {
		case "L":
			hash = hashPair(sibling, hash)
		case "R":
			hash = hashPair(hash, sibling)
		default:
			return false
		}
	}
	return root != "" && hash == root
}

func hashPair(left, right string) string {
	digest := sha256.Sum256([]byte(left + right))
	return hex.EncodeToString(digest[:])
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
