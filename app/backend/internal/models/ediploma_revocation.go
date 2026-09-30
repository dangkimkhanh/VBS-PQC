package models

// PQCRevocationContext separates revocation signatures from issuance signatures,
// so a signed issuance envelope can never be replayed as a revocation.
const PQCRevocationContext = "VBS-PQC-FABRIC-REVOKE-V1"

// RevokedEDiplomaItem lets the chaincode check that the revoked diploma belongs to
// an anchored batch: SHA-256(AnchorPayload) is the Merkle leaf and MerkleProof
// leads to the root stored for BatchID.
type RevokedEDiplomaItem struct {
	DiplomaID     string      `json:"diploma_id"`
	BatchID       string      `json:"batch_id"`
	AnchorPayload string      `json:"anchor_payload"`
	MerkleProof   []ProofNode `json:"merkle_proof"`
	RevokedAt     string      `json:"revoked_at"`
	ReasonHash    string      `json:"reason_hash"`
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
	TxID           string                     `json:"tx_id,omitempty"`
	CreatedAt      string                     `json:"created_at,omitempty"`
}

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

// EDiplomaRevocationStatus is the ledger view of one revoked diploma.
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

// PushRevocationResult summarises one "write revocations to the ledger" run.
type PushRevocationResult struct {
	Recorded     int      `json:"recorded"`
	NotAnchored  int      `json:"not_anchored"`
	Transactions []string `json:"transactions"`
}
