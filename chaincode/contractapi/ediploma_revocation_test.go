package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// testAnchorPayload mimics the backend anchor payload of one diploma.
func testAnchorPayload(t *testing.T, diplomaID, universityID string) string {
	t.Helper()
	bytes, err := json.Marshal(map[string]any{
		"domain":         anchorDomain,
		"anchor_version": 1,
		"credential_id":  diplomaID,
		"university_id":  universityID,
		"manifest_hash":  strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

// testMerkle builds the backend tree (odd node carried up) and returns the root and
// the proof of every leaf.
func testMerkle(leaves []string) (string, [][]MerkleProofNode) {
	type node struct {
		hash   string
		leaves []int
	}
	proofs := make([][]MerkleProofNode, len(leaves))
	level := make([]node, len(leaves))
	for i, leaf := range leaves {
		level[i] = node{hash: leaf, leaves: []int{i}}
	}
	for len(level) > 1 {
		next := []node{}
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				next = append(next, level[i])
				continue
			}
			left, right := level[i], level[i+1]
			for _, l := range left.leaves {
				proofs[l] = append(proofs[l], MerkleProofNode{Hash: right.hash, Position: "R"})
			}
			for _, r := range right.leaves {
				proofs[r] = append(proofs[r], MerkleProofNode{Hash: left.hash, Position: "L"})
			}
			next = append(next, node{hash: hashPair(left.hash, right.hash), leaves: append(append([]int{}, left.leaves...), right.leaves...)})
		}
		level = next
	}
	return level[0].hash, proofs
}

func signTestRevocation(t *testing.T, revocation *EDiplomaRevocationOnChain, context string) {
	t.Helper()
	itemsHash, err := revocationItemsHash(revocation.Items)
	if err != nil {
		t.Fatal(err)
	}
	revocation.ItemsHash = itemsHash
	revocation.Count = len(revocation.Items)
	publicKey, privateKey, err := mldsa65.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicBytes := publicKey.Bytes()
	keyID, signedAt, nonce := "revocation-key", "2026-02-01T00:00:00Z", "revocation-nonce"
	envelopeBytes, err := json.Marshal(pqcRevocationEnvelope(revocation, keyID, signedAt, nonce))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(envelopeBytes)
	signature := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(privateKey, envelopeBytes, []byte(context), true, signature); err != nil {
		t.Fatal(err)
	}
	fingerprint := sha256.Sum256(publicBytes)
	revocation.PQCTransaction = &PQCTransactionAttestation{
		Version:              1,
		Algorithm:            "ML-DSA-65",
		Context:              context,
		KeyID:                keyID,
		PublicKey:            base64.StdEncoding.EncodeToString(publicBytes),
		PublicKeyFingerprint: hex.EncodeToString(fingerprint[:]),
		EnvelopeHash:         hex.EncodeToString(digest[:]),
		Signature:            base64.StdEncoding.EncodeToString(signature),
		SignedAt:             signedAt,
		Nonce:                nonce,
	}
}

type revocationFixture struct {
	contract *CertificateContract
	stub     *testStub
	ctx      *CertificateTransactionContext
	batchID  string
	payloads []string
	proofs   [][]MerkleProofNode
}

// newRevocationFixture anchors a batch of three diplomas of university-1.
func newRevocationFixture(t *testing.T) *revocationFixture {
	t.Helper()
	f := &revocationFixture{contract: &CertificateContract{}, stub: newTestStub()}
	f.ctx = testContext(f.stub, &testClientIdentity{mspID: "UniversityMSP", clientID: "issuer-1"})
	leaves := []string{}
	for _, id := range []string{"diploma-1", "diploma-2", "diploma-3"} {
		payload := testAnchorPayload(t, id, "university-1")
		f.payloads = append(f.payloads, payload)
		leaf := sha256.Sum256([]byte(payload))
		leaves = append(leaves, hex.EncodeToString(leaf[:]))
	}
	root, proofs := testMerkle(leaves)
	f.proofs = proofs
	f.batchID = "EDIP-university-1-" + root
	batch := EDiplomaBatchOnChain{BatchID: f.batchID, UniversityID: "university-1", AggregateInfoHash: root,
		AggregateFileHash: strings.Repeat("b", 64), Count: 3}
	testPQCTransaction(t, &batch)
	payload, _ := json.Marshal(batch)
	if _, err := f.contract.IssueEDiplomaBatch(f.ctx, string(payload)); err != nil {
		t.Fatalf("IssueEDiplomaBatch() error = %v", err)
	}
	return f
}

func (f *revocationFixture) item(i int) RevokedEDiplomaItem {
	return RevokedEDiplomaItem{
		DiplomaID:     []string{"diploma-1", "diploma-2", "diploma-3"}[i],
		BatchID:       f.batchID,
		AnchorPayload: f.payloads[i],
		MerkleProof:   f.proofs[i],
		RevokedAt:     "2026-02-01T00:00:00Z",
		ReasonHash:    strings.Repeat("d", 64),
	}
}

func (f *revocationFixture) submit(t *testing.T, id string, items []RevokedEDiplomaItem, context string) error {
	t.Helper()
	revocation := EDiplomaRevocationOnChain{RevocationID: id, UniversityID: "university-1", FacultyID: "faculty-1",
		RoundID: "round-1", Items: items}
	signTestRevocation(t, &revocation, context)
	payload, _ := json.Marshal(revocation)
	_, err := f.contract.RevokeEDiplomaBatch(f.ctx, string(payload))
	return err
}

func TestRevokeEDiplomaBatchRecordsLedgerRevocation(t *testing.T) {
	f := newRevocationFixture(t)
	if err := f.submit(t, "REVK-1", []RevokedEDiplomaItem{f.item(0), f.item(2)}, revocationContext); err != nil {
		t.Fatalf("RevokeEDiplomaBatch() error = %v", err)
	}
	status, err := f.contract.GetEDiplomaRevocation(f.ctx, "diploma-3")
	if err != nil {
		t.Fatalf("GetEDiplomaRevocation() error = %v", err)
	}
	if status.RevocationID != "REVK-1" || status.BatchID != f.batchID || status.TxID != f.stub.txID || status.IssuerMSP != "UniversityMSP" {
		t.Fatalf("unexpected revocation status: %+v", status)
	}
	if _, err := f.contract.GetEDiplomaRevocation(f.ctx, "diploma-2"); err == nil || !strings.Contains(err.Error(), "is not revoked") {
		t.Fatalf("diploma-2 must not be revoked, got %v", err)
	}
	// A diploma is revoked on the ledger once; a second record for it is refused.
	if err := f.submit(t, "REVK-2", []RevokedEDiplomaItem{f.item(0)}, revocationContext); err == nil {
		t.Fatal("RevokeEDiplomaBatch() should refuse revoking the same diploma twice")
	}
}

func TestRevokeEDiplomaBatchRejectsForgedItems(t *testing.T) {
	f := newRevocationFixture(t)

	wrongProof := f.item(1)
	wrongProof.MerkleProof = f.proofs[0]
	if err := f.submit(t, "REVK-A", []RevokedEDiplomaItem{wrongProof}, revocationContext); err == nil {
		t.Fatal("a Merkle proof of another leaf must be rejected")
	}

	otherDiploma := f.item(1)
	otherDiploma.DiplomaID = "diploma-9" // payload still names diploma-2
	if err := f.submit(t, "REVK-B", []RevokedEDiplomaItem{otherDiploma}, revocationContext); err == nil {
		t.Fatal("an anchor payload of another diploma must be rejected")
	}

	missingBatch := f.item(1)
	missingBatch.BatchID = "EDIP-university-1-" + strings.Repeat("0", 64)
	if err := f.submit(t, "REVK-C", []RevokedEDiplomaItem{missingBatch}, revocationContext); err == nil {
		t.Fatal("a batch that is not on the ledger must be rejected")
	}

	// An issuance signature context cannot authorise a revocation.
	if err := f.submit(t, "REVK-D", []RevokedEDiplomaItem{f.item(1)}, "VBS-PQC-FABRIC-TX-V1"); err == nil {
		t.Fatal("a signature under the issuance context must be rejected")
	}

	// Changing an item after signing breaks items_hash.
	revocation := EDiplomaRevocationOnChain{RevocationID: "REVK-E", UniversityID: "university-1", Items: []RevokedEDiplomaItem{f.item(1)}}
	signTestRevocation(t, &revocation, revocationContext)
	revocation.Items[0].RevokedAt = "2020-01-01T00:00:00Z"
	payload, _ := json.Marshal(revocation)
	if _, err := f.contract.RevokeEDiplomaBatch(f.ctx, string(payload)); err == nil {
		t.Fatal("a revocation modified after signing must be rejected")
	}

	if _, err := f.contract.GetEDiplomaRevocation(f.ctx, "diploma-2"); err == nil {
		t.Fatal("no rejected revocation may reach the ledger")
	}
}

func TestMerkleProofMatchesBackendTree(t *testing.T) {
	leaves := []string{strings.Repeat("1", 64), strings.Repeat("2", 64), strings.Repeat("3", 64), strings.Repeat("4", 64), strings.Repeat("5", 64)}
	root, proofs := testMerkle(leaves)
	for i, leaf := range leaves {
		if !verifyMerkleProof(leaf, proofs[i], root) {
			t.Fatalf("proof of leaf %d does not verify", i)
		}
	}
	if verifyMerkleProof(leaves[0], []MerkleProofNode{{Hash: leaves[1], Position: "X"}}, root) {
		t.Fatal("an unknown proof position must be rejected")
	}
}
