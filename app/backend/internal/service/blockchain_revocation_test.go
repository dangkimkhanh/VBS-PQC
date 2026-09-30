package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// The chaincode re-hashes AnchorPayload into the Merkle leaf and walks the proof
// to the batch root, so both must match what was anchored.
func TestRevokedItemProvesMembershipOfTheAnchoredBatch(t *testing.T) {
	university := primitive.NewObjectID()
	diplomas := []*models.EDiploma{}
	leaves := []string{}
	for i := 0; i < 3; i++ {
		revokedAt := time.Date(2026, 9, 27, 10, i, 0, 0, time.UTC)
		d := &models.EDiploma{ID: primitive.NewObjectID(), UniversityID: university, EDiplomaFileHash: "ABCD", RevokedAt: &revokedAt,
			RevocationReason: "Cấp sai danh sách",
			PQCProof: &models.PQCProof{AnchorVersion: models.PQCAnchorVersion, Algorithm: "ML-DSA-65", KeyID: "key",
				PublicKeyFingerprint: "fp", ManifestHash: "mh", Signature: base64.StdEncoding.EncodeToString([]byte{byte(i)})}}
		leaf, err := hashPQCEDiplomaAnchor(d)
		if err != nil {
			t.Fatal(err)
		}
		diplomas = append(diplomas, d)
		leaves = append(leaves, leaf)
	}
	tree := models.NewMerkleTreeFromStrings(leaves)
	for i, d := range diplomas {
		d.MerkleProof = tree.GetProof(leaves[i])
		item, err := revokedItem(d)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte(item.AnchorPayload))
		if leaf := hex.EncodeToString(digest[:]); leaf != leaves[i] || !models.VerifyProof(leaf, item.MerkleProof, tree.RootHash()) {
			t.Fatalf("revocation item %d does not prove membership of the anchored batch", i)
		}
		if item.RevokedAt != d.RevokedAt.Format(time.RFC3339Nano) || len(item.ReasonHash) != 64 {
			t.Fatalf("unexpected revocation item %+v", item)
		}
	}
	// The hash commits to the list regardless of order.
	items := []models.RevokedEDiplomaItem{}
	for _, d := range diplomas {
		item, _ := revokedItem(d)
		items = append(items, item)
	}
	forward, _ := revocationItemsHash(items)
	reversed, _ := revocationItemsHash([]models.RevokedEDiplomaItem{items[2], items[1], items[0]})
	if forward != reversed {
		t.Fatal("items hash must not depend on the order of the list")
	}
}
