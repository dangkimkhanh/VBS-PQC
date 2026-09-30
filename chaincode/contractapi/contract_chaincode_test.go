package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/hyperledger/fabric-chaincode-go/v2/pkg/cid"
	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type testClientIdentity struct {
	cid.ClientIdentity
	mspID      string
	clientID   string
	attributes map[string]string
}

func (i *testClientIdentity) GetMSPID() (string, error) { return i.mspID, nil }
func (i *testClientIdentity) GetID() (string, error)    { return i.clientID, nil }
func (i *testClientIdentity) GetAttributeValue(name string) (string, bool, error) {
	value, found := i.attributes[name]
	return value, found, nil
}

type testStub struct {
	shim.ChaincodeStubInterface
	state     map[string][]byte
	history   map[string][]*queryresult.KeyModification
	txID      string
	timestamp *timestamppb.Timestamp
	events    map[string][]byte
}

func newTestStub() *testStub {
	return &testStub{
		state:     make(map[string][]byte),
		history:   make(map[string][]*queryresult.KeyModification),
		txID:      "tx-1",
		timestamp: timestamppb.New(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)),
		events:    make(map[string][]byte),
	}
}

func (s *testStub) GetState(key string) ([]byte, error) {
	value := s.state[key]
	return append([]byte(nil), value...), nil
}

func (s *testStub) PutState(key string, value []byte) error {
	copyValue := append([]byte(nil), value...)
	s.state[key] = copyValue
	s.history[key] = append(s.history[key], &queryresult.KeyModification{
		TxId:      s.txID,
		Value:     copyValue,
		Timestamp: s.timestamp,
	})
	return nil
}

func (s *testStub) DelState(key string) error {
	delete(s.state, key)
	s.history[key] = append(s.history[key], &queryresult.KeyModification{
		TxId:      s.txID,
		Timestamp: s.timestamp,
		IsDelete:  true,
	})
	return nil
}

func (s *testStub) GetTxID() string                                 { return s.txID }
func (s *testStub) GetTxTimestamp() (*timestamppb.Timestamp, error) { return s.timestamp, nil }
func (s *testStub) SetEvent(name string, payload []byte) error {
	s.events[name] = append([]byte(nil), payload...)
	return nil
}

func (s *testStub) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	return shim.CreateCompositeKey(objectType, attributes)
}

func (s *testStub) SplitCompositeKey(key string) (string, []string, error) {
	parts := strings.Split(key, string(rune(0)))
	return parts[1], parts[2 : len(parts)-1], nil
}

func (s *testStub) GetStateByPartialCompositeKey(objectType string, attributes []string) (shim.StateQueryIteratorInterface, error) {
	prefix, err := shim.CreateCompositeKey(objectType, attributes)
	if err != nil {
		return nil, err
	}
	return s.iteratorForPrefix(prefix), nil
}

func (s *testStub) GetStateByRange(startKey, endKey string) (shim.StateQueryIteratorInterface, error) {
	keys := make([]string, 0, len(s.state))
	for key := range s.state {
		if (startKey == "" || key >= startKey) && (endKey == "" || key < endKey) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	items := make([]*queryresult.KV, 0, len(keys))
	for _, key := range keys {
		items = append(items, &queryresult.KV{Key: key, Value: append([]byte(nil), s.state[key]...)})
	}
	return &testStateIterator{items: items}, nil
}

func (s *testStub) iteratorForPrefix(prefix string) shim.StateQueryIteratorInterface {
	keys := make([]string, 0)
	for key := range s.state {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	items := make([]*queryresult.KV, 0, len(keys))
	for _, key := range keys {
		items = append(items, &queryresult.KV{Key: key, Value: append([]byte(nil), s.state[key]...)})
	}
	return &testStateIterator{items: items}
}

func (s *testStub) GetHistoryForKey(key string) (shim.HistoryQueryIteratorInterface, error) {
	return &testHistoryIterator{items: s.history[key]}, nil
}

type testStateIterator struct {
	items []*queryresult.KV
	index int
}

func (i *testStateIterator) HasNext() bool { return i.index < len(i.items) }
func (i *testStateIterator) Close() error  { return nil }
func (i *testStateIterator) Next() (*queryresult.KV, error) {
	item := i.items[i.index]
	i.index++
	return item, nil
}

type testHistoryIterator struct {
	items []*queryresult.KeyModification
	index int
}

func (i *testHistoryIterator) HasNext() bool { return i.index < len(i.items) }
func (i *testHistoryIterator) Close() error  { return nil }
func (i *testHistoryIterator) Next() (*queryresult.KeyModification, error) {
	item := i.items[i.index]
	i.index++
	return item, nil
}

func testContext(stub *testStub, identity *testClientIdentity) *CertificateTransactionContext {
	ctx := &CertificateTransactionContext{TransactionContext: contractapi.TransactionContext{}}
	ctx.SetStub(stub)
	ctx.SetClientIdentity(identity)
	return ctx
}

func certificateJSON(t *testing.T, certID, certHash string) string {
	t.Helper()
	bytes, err := json.Marshal(CertificateOnChain{
		CertID:       certID,
		CertHash:     certHash,
		HashFile:     "file-hash",
		SerialNumber: "SER-1",
		RegNo:        "REG-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes)
}

func testPQCTransaction(t *testing.T, batch *EDiplomaBatchOnChain) {
	t.Helper()
	publicKey, privateKey, err := mldsa65.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicBytes := publicKey.Bytes()
	keyID := "test-transaction-key"
	signedAt := "2026-01-01T00:00:00Z"
	nonce := "test-nonce"
	envelope := pqcTransactionEnvelope(batch, keyID, signedAt, nonce)
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(envelopeBytes)
	signature := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(privateKey, envelopeBytes, []byte("VBS-PQC-FABRIC-TX-V1"), true, signature); err != nil {
		t.Fatal(err)
	}
	fingerprint := sha256.Sum256(publicBytes)
	batch.PQCTransaction = &PQCTransactionAttestation{
		Version:              1,
		Algorithm:            "ML-DSA-65",
		Context:              "VBS-PQC-FABRIC-TX-V1",
		KeyID:                keyID,
		PublicKey:            base64.StdEncoding.EncodeToString(publicBytes),
		PublicKeyFingerprint: hex.EncodeToString(fingerprint[:]),
		EnvelopeHash:         hex.EncodeToString(digest[:]),
		Signature:            base64.StdEncoding.EncodeToString(signature),
		SignedAt:             signedAt,
		Nonce:                nonce,
	}
}

func TestEDiplomaBatchIsImmutableAndAudited(t *testing.T) {
	contract := &CertificateContract{}
	stub := newTestStub()
	identity := &testClientIdentity{mspID: "UniversityMSP", clientID: "issuer-1"}
	ctx := testContext(stub, identity)
	batch := EDiplomaBatchOnChain{
		BatchID:           "EDIP-university-root",
		UniversityID:      "university-1",
		AggregateInfoHash: strings.Repeat("a", 64),
		AggregateFileHash: strings.Repeat("b", 64),
		Count:             2,
	}
	testPQCTransaction(t, &batch)
	payload, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contract.IssueEDiplomaBatch(ctx, string(payload)); err != nil {
		t.Fatalf("IssueEDiplomaBatch() error = %v", err)
	}
	stored, err := contract.ReadEDiplomaBatch(ctx, batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TxID != stub.txID || stored.CreatedAt == "" || stored.IssuerMSP != identity.mspID || stored.IssuerID != identity.clientID {
		t.Fatalf("batch audit metadata missing: %+v", stored)
	}
	if _, err := contract.IssueEDiplomaBatch(ctx, string(payload)); err == nil {
		t.Fatal("IssueEDiplomaBatch() should reject overwriting an existing root")
	}
}

func TestCertificateLifecycleAndQueries(t *testing.T) {
	contract := &CertificateContract{}
	stub := newTestStub()
	identity := &testClientIdentity{mspID: "UniversityMSP", clientID: "issuer-1"}
	ctx := testContext(stub, identity)
	hash := strings.Repeat("a", 64)

	if _, err := contract.IssueCertificate(ctx, certificateJSON(t, "cert-1", hash)); err != nil {
		t.Fatalf("IssueCertificate() error = %v", err)
	}
	status, err := contract.GetCertificateStatus(ctx, "cert-1")
	if err != nil || status.Status != CertificateStatusActive || status.Version != 1 {
		t.Fatalf("unexpected active status: status=%+v err=%v", status, err)
	}
	byHash, err := contract.GetCertificateByHash(ctx, strings.ToUpper(hash))
	if err != nil || byHash.CertID != "cert-1" {
		t.Fatalf("GetCertificateByHash() = %+v, %v", byHash, err)
	}

	stub.txID = "tx-2"
	stub.timestamp = timestamppb.New(time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC))
	if _, err := contract.RevokeCertificate(ctx, "cert-1", "credential issued in error"); err != nil {
		t.Fatalf("RevokeCertificate() error = %v", err)
	}
	status, err = contract.GetCertificateStatus(ctx, "cert-1")
	if err != nil || status.Status != CertificateStatusRevoked || status.Version != 2 {
		t.Fatalf("unexpected revoked status: status=%+v err=%v", status, err)
	}
	if err := contract.UpdateCertificate(ctx, certificateJSON(t, "cert-1", hash)); err == nil {
		t.Fatal("UpdateCertificate() should reject a revoked certificate")
	}

	stub.txID = "tx-3"
	stub.timestamp = timestamppb.New(time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC))
	if _, err := contract.DeleteCertificate(ctx, "cert-1", "retention policy tombstone"); err != nil {
		t.Fatalf("DeleteCertificate() error = %v", err)
	}
	status, err = contract.GetCertificateStatus(ctx, "cert-1")
	if err != nil || status.Status != CertificateStatusDeleted || status.Version != 3 {
		t.Fatalf("unexpected deleted status: status=%+v err=%v", status, err)
	}

	history, err := contract.GetCertificateHistory(ctx, "cert-1")
	if err != nil {
		t.Fatalf("GetCertificateHistory() error = %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("GetCertificateHistory() length = %d, want 3", len(history))
	}
	if history[0].Value.Status != CertificateStatusActive || history[1].Value.Status != CertificateStatusRevoked || history[2].Value.Status != CertificateStatusDeleted {
		t.Fatalf("unexpected lifecycle history: %+v", history)
	}
}

func TestCertificateUniquenessAndAuthorization(t *testing.T) {
	contract := &CertificateContract{}
	stub := newTestStub()
	issuer := &testClientIdentity{mspID: "UniversityMSP", clientID: "issuer-1"}
	ctx := testContext(stub, issuer)
	hash := strings.Repeat("b", 64)

	if _, err := contract.IssueCertificate(ctx, certificateJSON(t, "cert-1", hash)); err != nil {
		t.Fatal(err)
	}
	stub.txID = "tx-2"
	if _, err := contract.IssueCertificate(ctx, certificateJSON(t, "cert-2", hash)); err == nil {
		t.Fatal("IssueCertificate() should reject a duplicate certificate hash")
	}

	ctx.SetClientIdentity(&testClientIdentity{mspID: "UniversityMSP", clientID: "other-user"})
	if _, err := contract.RevokeCertificate(ctx, "cert-1", "unauthorized attempt"); err == nil {
		t.Fatal("RevokeCertificate() should reject a non-issuer")
	}

	ctx.SetClientIdentity(&testClientIdentity{
		mspID:      "UniversityMSP",
		clientID:   "org-admin",
		attributes: map[string]string{"certificateAdmin": "true"},
	})
	if _, err := contract.RevokeCertificate(ctx, "cert-1", "approved by organization administrator"); err != nil {
		t.Fatalf("RevokeCertificate() admin override error = %v", err)
	}
}

func TestContractMetadataBuilds(t *testing.T) {
	if _, err := contractapi.NewChaincode(&CertificateContract{}); err != nil {
		t.Fatalf("contractapi.NewChaincode() error = %v", err)
	}
}

func TestOptionalLifecycleFieldsAreAlwaysSerializedForContractSchema(t *testing.T) {
	requiredFields := []string{
		"revoked_at",
		"revoked_by",
		"revocation_reason",
		"deleted_at",
		"deleted_by",
		"deletion_reason",
	}

	assertFields := func(name string, value any) {
		t.Helper()
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		var document map[string]any
		if err := json.Unmarshal(encoded, &document); err != nil {
			t.Fatalf("unmarshal %s: %v", name, err)
		}
		for _, field := range requiredFields {
			if _, exists := document[field]; !exists {
				t.Fatalf("%s omitted schema-required field %q: %s", name, field, encoded)
			}
		}
	}

	assertFields("CertificateOnChain", CertificateOnChain{})
	assertFields("CertificateStatusResponse", CertificateStatusResponse{})
}
