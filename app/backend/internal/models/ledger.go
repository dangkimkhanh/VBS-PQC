package models

// Ledger types shared with the deployed Fabric chaincode. The certificate product
// has been removed; these shapes stay because the chaincode and Fabric client use them.

type OnBlockchainVerify struct {
	UniversityID    string `bson:"university_id,omitempty" json:"university_id,omitempty"`
	FacultyID       string `bson:"faculty_id,omitempty" json:"faculty_id,omitempty"`
	CertificateType string `bson:"certificate_type,omitempty" json:"certificate_type,omitempty"`
	Course          string `bson:"course,omitempty" json:"course,omitempty"`
}

type CertificateOnChain struct {
	CertID              string `json:"cert_id" bson:"cert_id"`                           // ID của VBCC
	CertHash            string `json:"cert_hash" bson:"cert_hash"`                       // Mã băm các thông tin chính
	HashFile            string `json:"hash_file" bson:"hash_file"`                       // Mã băm file
	UniversitySignature string `json:"university_signature" bson:"university_signature"` // Chữ ký số của trường
	DateOfIssuing       string `json:"date_of_issuing" bson:"date_of_issuing"`           // Ngày cấp
	SerialNumber        string `bson:"serial_number" json:"serial_number"`               // Số hiệu
	RegNo               string `bson:"registration_number" json:"registration_number"`   // Số vào sổ gốc
	Version             int    `json:"version" bson:"version"`                           // Phiên bản VBCC
	UpdatedDate         string `json:"updated_date" bson:"updated_date"`                 // Ngày sửa đổi
	Status              string `json:"status" bson:"status"`
	IssuerMSP           string `json:"issuer_msp" bson:"issuer_msp"`
	IssuerID            string `json:"issuer_id" bson:"issuer_id"`
	CreatedAt           string `json:"created_at" bson:"created_at"`
	LastTxID            string `json:"last_tx_id" bson:"last_tx_id"`
	RevokedAt           string `json:"revoked_at,omitempty" bson:"revoked_at,omitempty"`
	RevokedBy           string `json:"revoked_by,omitempty" bson:"revoked_by,omitempty"`
	RevocationReason    string `json:"revocation_reason,omitempty" bson:"revocation_reason,omitempty"`
	DeletedAt           string `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
	DeletedBy           string `json:"deleted_by,omitempty" bson:"deleted_by,omitempty"`
	DeletionReason      string `json:"deletion_reason,omitempty" bson:"deletion_reason,omitempty"`
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
	RevokedAt        string `json:"revoked_at,omitempty"`
	RevokedBy        string `json:"revoked_by,omitempty"`
	RevocationReason string `json:"revocation_reason,omitempty"`
	DeletedAt        string `json:"deleted_at,omitempty"`
	DeletedBy        string `json:"deleted_by,omitempty"`
	DeletionReason   string `json:"deletion_reason,omitempty"`
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
}
