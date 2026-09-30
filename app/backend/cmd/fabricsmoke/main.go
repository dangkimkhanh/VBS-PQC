package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/blockchain"
)

func main() {
	_ = godotenv.Load()
	if !strings.EqualFold(os.Getenv("FABRIC_SMOKE_WRITE"), "true") {
		log.Fatal("refusing to write ledger data: set FABRIC_SMOKE_WRITE=true for an authorized test network")
	}

	client, err := blockchain.NewFabricClient(blockchain.NewFabricConfigFromEnv())
	must("connect", err)
	defer client.Close()

	runID := time.Now().UTC().Format("20060102T150405.000000000Z")
	certificateID := "SMOKE-CERT-" + runID
	originalHash := digest("certificate-original-" + runID)
	updatedHash := digest("certificate-updated-" + runID)

	certificate := models.CertificateOnChain{
		CertID:              certificateID,
		CertHash:            originalHash,
		HashFile:            digest("file-" + runID),
		UniversitySignature: "SMOKE-TEST-SIGNATURE",
		DateOfIssuing:       "2026-08-01",
		SerialNumber:        "SMOKE-SERIAL-" + runID,
		RegNo:               "SMOKE-REG-" + runID,
	}

	issueTxID, err := client.IssueCertificate(certificate)
	must("IssueCertificate", err)
	log.Printf("PASS IssueCertificate tx=%s cert=%s", issueTxID, certificateID)

	issued, err := client.GetCertificateByID(certificateID)
	must("ReadCertificate after issue", err)
	require("issued certificate fields", issued.CertID == certificateID && issued.CertHash == originalHash)
	require("issued status/version", issued.Status == "ACTIVE" && issued.Version == 1)
	log.Printf("PASS ReadCertificate status=%s version=%d", issued.Status, issued.Version)

	byOriginalHash, err := client.GetCertificateByHash(originalHash)
	must("GetCertificateByHash after issue", err)
	require("hash index after issue", byOriginalHash.CertID == certificateID)
	log.Printf("PASS GetCertificateByHash hash=%s", originalHash)

	status, err := client.GetCertificateStatus(certificateID)
	must("GetCertificateStatus after issue", err)
	require("status after issue", status.Status == "ACTIVE" && status.Version == 1)
	log.Printf("PASS GetCertificateStatus status=%s version=%d", status.Status, status.Version)

	duplicate := certificate
	duplicate.CertID = "SMOKE-DUPLICATE-" + runID
	_, err = client.IssueCertificate(duplicate)
	require("duplicate hash rejected", err != nil && strings.Contains(err.Error(), "hash already belongs"))
	log.Printf("PASS duplicate hash rejected")

	updated := certificate
	updated.CertHash = updatedHash
	updated.SerialNumber += "-UPDATED"
	updated.RegNo += "-UPDATED"
	must("UpdateCertificate", client.UpdateCertificate(updated))
	log.Printf("PASS UpdateCertificate")

	readUpdated, err := client.GetCertificateByID(certificateID)
	must("ReadCertificate after update", err)
	require("updated fields/status/version", readUpdated.CertHash == updatedHash && readUpdated.Status == "ACTIVE" && readUpdated.Version == 2)
	_, oldHashErr := client.GetCertificateByHash(originalHash)
	require("old hash index removed", oldHashErr != nil)
	byUpdatedHash, err := client.GetCertificateByHash(updatedHash)
	must("new hash index", err)
	require("new hash points to certificate", byUpdatedHash.CertID == certificateID)
	log.Printf("PASS hash index moved old=%s new=%s", originalHash, updatedHash)

	revokeTxID, err := client.RevokeCertificate(certificateID, "authorized Fabric smoke test revoke")
	must("RevokeCertificate", err)
	status, err = client.GetCertificateStatus(certificateID)
	must("GetCertificateStatus after revoke", err)
	require("status after revoke", status.Status == "REVOKED" && status.Version == 3 && status.RevocationReason != "")
	log.Printf("PASS RevokeCertificate tx=%s status=%s version=%d", revokeTxID, status.Status, status.Version)

	err = client.UpdateCertificate(updated)
	require("update revoked certificate rejected", err != nil && strings.Contains(err.Error(), "cannot be updated"))
	log.Printf("PASS update after revoke rejected")

	deleteTxID, err := client.DeleteCertificate(certificateID, "authorized Fabric smoke test logical delete")
	must("DeleteCertificate", err)
	status, err = client.GetCertificateStatus(certificateID)
	must("GetCertificateStatus after delete", err)
	require("status after delete", status.Status == "DELETED" && status.Version == 4 && status.DeletionReason != "")
	log.Printf("PASS DeleteCertificate tx=%s status=%s version=%d", deleteTxID, status.Status, status.Version)

	deletedCertificate, err := client.GetCertificateByID(certificateID)
	must("ReadCertificate after logical delete", err)
	require("logical delete retained record", deletedCertificate.Status == "DELETED")
	byUpdatedHash, err = client.GetCertificateByHash(updatedHash)
	must("GetCertificateByHash after logical delete", err)
	require("hash lookup retained after logical delete", byUpdatedHash.Status == "DELETED")
	log.Printf("PASS logical delete retains read and hash lookup")

	history, err := client.GetCertificateHistory(certificateID)
	must("GetCertificateHistory", err)
	require("history contains issue/update/revoke/delete", len(history) == 4)
	// Fabric history is returned newest-first by this peer implementation.
	wantStatuses := []string{"DELETED", "REVOKED", "ACTIVE", "ACTIVE"}
	for index, wantStatus := range wantStatuses {
		if history[index].Value == nil {
			log.Printf("HISTORY entry=%d tx=%s timestamp=%s is_delete=%t value=nil", index+1, history[index].TxID, history[index].Timestamp, history[index].IsDelete)
		} else {
			log.Printf("HISTORY entry=%d tx=%s timestamp=%s is_delete=%t status=%s version=%d", index+1, history[index].TxID, history[index].Timestamp, history[index].IsDelete, history[index].Value.Status, history[index].Value.Version)
		}
		require(fmt.Sprintf("history entry %d", index+1), history[index].Value != nil && history[index].Value.Status == wantStatus)
	}
	log.Printf("PASS GetCertificateHistory entries=%d statuses=%s", len(history), strings.Join(wantStatuses, ","))

	_, err = client.RevokeCertificate(certificateID, "must fail after delete")
	require("revoke deleted certificate rejected", err != nil)
	log.Printf("PASS revoke after delete rejected")

	certificateBatch := models.CertificateBatchOnChain{
		BatchID:           "SMOKE-CERT-BATCH-" + runID,
		UniversityID:      "SMOKE-UNIVERSITY",
		FacultyID:         "SMOKE-FACULTY",
		CertificateType:   "SMOKE-CERTIFICATE",
		Course:            "SMOKE-2026",
		AggregateInfoHash: digest("certificate-batch-info-" + runID),
		AggregateFileHash: digest("certificate-batch-file-" + runID),
		Count:             2,
	}
	certificateBatchTxID, err := client.IssueCertificateBatch(certificateBatch)
	must("IssueCertificateBatch", err)
	readCertificateBatch, err := client.GetCertificateBatch(certificateBatch.BatchID)
	must("ReadCertificateBatch", err)
	require("certificate batch fields", readCertificateBatch.BatchID == certificateBatch.BatchID && readCertificateBatch.AggregateInfoHash == certificateBatch.AggregateInfoHash && readCertificateBatch.Count == certificateBatch.Count)
	log.Printf("PASS IssueCertificateBatch/ReadCertificateBatch tx=%s batch=%s", certificateBatchTxID, certificateBatch.BatchID)

	eDiplomaBatch := models.EDiplomaBatchOnChain{
		BatchID:           "SMOKE-EDIPLOMA-BATCH-" + runID,
		UniversityID:      "SMOKE-UNIVERSITY",
		FacultyID:         "SMOKE-FACULTY",
		CertificateType:   "SMOKE-EDIPLOMA",
		Course:            "SMOKE-2026",
		AggregateInfoHash: digest("ediploma-batch-info-" + runID),
		AggregateFileHash: digest("ediploma-batch-file-" + runID),
		Count:             3,
	}
	eDiplomaBatchTxID, err := client.IssueEDiplomaBatch(eDiplomaBatch)
	must("IssueEDiplomaBatch", err)
	readEDiplomaBatch, err := client.GetEDiplomaBatch(eDiplomaBatch.BatchID)
	must("ReadEDiplomaBatch", err)
	require("eDiploma batch fields", readEDiplomaBatch.BatchID == eDiplomaBatch.BatchID && readEDiplomaBatch.AggregateInfoHash == eDiplomaBatch.AggregateInfoHash && readEDiplomaBatch.Count == eDiplomaBatch.Count)
	log.Printf("PASS IssueEDiplomaBatch/ReadEDiplomaBatch tx=%s batch=%s", eDiplomaBatchTxID, eDiplomaBatch.BatchID)

	log.Printf("FABRIC SMOKE TEST PASSED run=%s certificate=%s certificate_batch=%s ediploma_batch=%s", runID, certificateID, certificateBatch.BatchID, eDiplomaBatch.BatchID)
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func must(step string, err error) {
	if err != nil {
		log.Fatalf("FAIL %s: %v", step, err)
	}
}

func require(step string, condition bool) {
	if !condition {
		log.Fatalf("FAIL %s", step)
	}
}
