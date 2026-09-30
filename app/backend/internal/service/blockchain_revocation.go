package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// revocationsPerTransaction keeps each Fabric transaction well below the
// chaincode limit of 500 diplomas and a reasonable payload size.
const revocationsPerTransaction = 200

var (
	ErrNoRevocationsToRecord = errors.New("Không có văn bằng đã thu hồi nào cần ghi lên Blockchain trong đợt và chuyên ngành này")
	ErrFabricUnavailable     = errors.New("Chưa kết nối được mạng Blockchain")
)

func (s *blockchainService) PushRevocationsToBlockchain(ctx context.Context, universityID string, scope bson.M) (*models.PushRevocationResult, error) {
	if s.fabricClient == nil {
		return nil, ErrFabricUnavailable
	}
	if s.pqcTxSigner == nil {
		return nil, errors.New("PQC transaction signer is not configured")
	}
	filter := bson.M{}
	for k, v := range scope {
		filter[k] = v
	}
	filter["revoked"] = true
	filter["revocation_on_chain"] = bson.M{"$ne": true}
	diplomas, err := s.ediplomaRepo.FindByDynamicFilter(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("load revoked diplomas: %w", err)
	}

	result := &models.PushRevocationResult{Transactions: []string{}}
	items := []models.RevokedEDiplomaItem{}
	for _, d := range diplomas {
		// Only diplomas anchored with a PQC leaf have something to revoke on the ledger.
		if !d.OnBlockchain || d.BatchID == "" || d.PQCProof == nil || d.PQCProof.AnchorVersion < models.PQCAnchorVersion {
			result.NotAnchored++
			continue
		}
		item, err := revokedItem(d)
		if err != nil {
			log.Printf("[BlockchainService] cannot prepare revocation of %s: %v", d.ID.Hex(), err)
			result.NotAnchored++
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return result, ErrNoRevocationsToRecord
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DiplomaID < items[j].DiplomaID })

	facultyID, roundID := "", ""
	if id, ok := scope["faculty_id"].(primitive.ObjectID); ok {
		facultyID = id.Hex()
	}
	if id, ok := scope["round_id"].(primitive.ObjectID); ok {
		roundID = id.Hex()
	}
	for start := 0; start < len(items); start += revocationsPerTransaction {
		end := start + revocationsPerTransaction
		if end > len(items) {
			end = len(items)
		}
		chunk := items[start:end]
		itemsHash, err := revocationItemsHash(chunk)
		if err != nil {
			return result, err
		}
		revocation := models.EDiplomaRevocationOnChain{
			// The ID commits to the listed diplomas, so the same list is never recorded twice.
			RevocationID: fmt.Sprintf("REVK-%s-%s", universityID, itemsHash),
			UniversityID: universityID,
			FacultyID:    facultyID,
			RoundID:      roundID,
			ItemsHash:    itemsHash,
			Count:        len(chunk),
			Items:        chunk,
		}
		if err := s.pqcTxSigner.SignEDiplomaRevocation(ctx, &revocation); err != nil {
			return result, fmt.Errorf("sign revocation with PQC: %w", err)
		}
		txID, err := s.fabricClient.RevokeEDiplomaBatch(revocation)
		if err != nil {
			return result, fmt.Errorf("record revocation on Fabric: %w", err)
		}
		result.Transactions = append(result.Transactions, txID)
		for _, item := range chunk {
			id, _ := primitive.ObjectIDFromHex(item.DiplomaID)
			if err := s.ediplomaRepo.MarkRevocationOnChain(ctx, id, revocation.RevocationID, txID); err != nil {
				log.Printf("[BlockchainService] revocation of %s is on Fabric (%s) but not saved: %v", item.DiplomaID, txID, err)
				continue
			}
			result.Recorded++
		}
	}
	return result, nil
}

func revokedItem(d *models.EDiploma) (models.RevokedEDiplomaItem, error) {
	payload, err := pqcAnchorPayload(d)
	if err != nil {
		return models.RevokedEDiplomaItem{}, err
	}
	proof := d.MerkleProof
	if proof == nil {
		proof = []models.ProofNode{} // a one-leaf batch has an empty proof
	}
	revokedAt := time.Now().UTC()
	if d.RevokedAt != nil {
		revokedAt = d.RevokedAt.UTC()
	}
	reasonHash := sha256.Sum256([]byte(strings.TrimSpace(d.RevocationReason)))
	return models.RevokedEDiplomaItem{
		DiplomaID:     d.ID.Hex(),
		BatchID:       d.BatchID,
		AnchorPayload: string(payload),
		MerkleProof:   proof,
		RevokedAt:     revokedAt.Format(time.RFC3339Nano),
		ReasonHash:    hex.EncodeToString(reasonHash[:]),
	}, nil
}

// revocationItemsHash must match the chaincode: SHA-256 of the JSON list sorted by diploma ID.
func revocationItemsHash(items []models.RevokedEDiplomaItem) (string, error) {
	sorted := append([]models.RevokedEDiplomaItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].DiplomaID < sorted[j].DiplomaID })
	bytes, err := json.Marshal(sorted)
	if err != nil {
		return "", fmt.Errorf("marshal revoked diplomas: %w", err)
	}
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:]), nil
}
