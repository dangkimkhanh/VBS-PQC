package blockchain

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/vnkmasc/Kmasc/app/backend/internal/common"
	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
)

type FabricConfig struct {
	MSPID               string
	GatewayEndpoint     string
	TLSAuthority        string
	ChannelName         string
	ChaincodeName       string
	ContractName        string
	TLSCertPath         string
	TLSClientCertPath   string
	TLSClientKeyPath    string
	ClientCertPath      string
	ClientKeyPath       string
	ConnectTimeout      time.Duration
	EvaluateTimeout     time.Duration
	EndorseTimeout      time.Duration
	SubmitTimeout       time.Duration
	CommitStatusTimeout time.Duration
	StartupCheck        bool
}

func NewFabricConfigFromEnv() *FabricConfig {
	return &FabricConfig{
		MSPID:               getEnv("FABRIC_MSP_ID", "UniversityMSP"),
		GatewayEndpoint:     getEnv("FABRIC_GATEWAY_ENDPOINT", "136.114.78.203:7095"),
		TLSAuthority:        getEnv("FABRIC_TLS_AUTHORITY_OVERRIDE", ""),
		ChannelName:         getEnv("FABRIC_CHANNEL_NAME", "pqc-channel"),
		ChaincodeName:       getEnv("FABRIC_CHAINCODE_NAME", "vbs-chaincode"),
		ContractName:        getEnv("FABRIC_CONTRACT_NAME", "CertificateContract"),
		TLSCertPath:         getEnv("FABRIC_TLS_CERT_PATH", "secrets/fabric/peer-tls-ca.pem"),
		TLSClientCertPath:   getEnv("FABRIC_TLS_CLIENT_CERT_PATH", ""),
		TLSClientKeyPath:    getEnv("FABRIC_TLS_CLIENT_KEY_PATH", ""),
		ClientCertPath:      getEnv("FABRIC_CLIENT_CERT_PATH", "secrets/fabric/client-cert.pem"),
		ClientKeyPath:       getEnv("FABRIC_CLIENT_KEY_PATH", "secrets/fabric/client-key.pem"),
		ConnectTimeout:      getEnvDuration("FABRIC_CONNECT_TIMEOUT", 10*time.Second),
		EvaluateTimeout:     getEnvDuration("FABRIC_EVALUATE_TIMEOUT", 10*time.Second),
		EndorseTimeout:      getEnvDuration("FABRIC_ENDORSE_TIMEOUT", 15*time.Second),
		SubmitTimeout:       getEnvDuration("FABRIC_SUBMIT_TIMEOUT", 10*time.Second),
		CommitStatusTimeout: getEnvDuration("FABRIC_COMMIT_STATUS_TIMEOUT", 30*time.Second),
		StartupCheck:        !strings.EqualFold(getEnv("FABRIC_STARTUP_CHECK", "true"), "false"),
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("invalid %s=%q, using %s", key, value, fallback)
		return fallback
	}
	return duration
}

type FabricClient struct {
	cfg        *FabricConfig
	connection *grpc.ClientConn
	gateway    *client.Gateway
	contract   *client.Contract
}

func NewFabricClient(cfg *FabricConfig) (*FabricClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("fabric config is required")
	}
	if err := validateFabricConfig(cfg); err != nil {
		return nil, err
	}

	tlsPEM, err := os.ReadFile(filepath.Clean(cfg.TLSCertPath))
	if err != nil {
		return nil, fmt.Errorf("read Fabric TLS CA %s: %w", cfg.TLSCertPath, err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(tlsPEM) {
		return nil, fmt.Errorf("Fabric TLS CA contains no valid X.509 certificate: %s", cfg.TLSCertPath)
	}

	certificatePEM, err := os.ReadFile(filepath.Clean(cfg.ClientCertPath))
	if err != nil {
		return nil, fmt.Errorf("read Fabric client certificate %s: %w", cfg.ClientCertPath, err)
	}
	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		return nil, fmt.Errorf("parse Fabric client certificate: %w", err)
	}
	if now := time.Now(); now.Before(certificate.NotBefore) || now.After(certificate.NotAfter) {
		return nil, fmt.Errorf("Fabric client certificate is not valid at %s (valid from %s to %s)", now.UTC(), certificate.NotBefore.UTC(), certificate.NotAfter.UTC())
	}

	privateKeyPEM, err := os.ReadFile(filepath.Clean(cfg.ClientKeyPath))
	if err != nil {
		return nil, fmt.Errorf("read Fabric client private key %s: %w", cfg.ClientKeyPath, err)
	}
	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse Fabric client private key: %w", err)
	}
	if err := verifyCertificateMatchesKey(certificate, privateKey); err != nil {
		return nil, err
	}

	clientIdentity, err := identity.NewX509Identity(cfg.MSPID, certificate)
	if err != nil {
		return nil, fmt.Errorf("create Fabric identity: %w", err)
	}
	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		return nil, fmt.Errorf("create Fabric signer: %w", err)
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
		ServerName: cfg.TLSAuthority,
	}
	if cfg.TLSClientCertPath != "" || cfg.TLSClientKeyPath != "" {
		if cfg.TLSClientCertPath == "" || cfg.TLSClientKeyPath == "" {
			return nil, fmt.Errorf("FABRIC_TLS_CLIENT_CERT_PATH and FABRIC_TLS_CLIENT_KEY_PATH must be configured together")
		}
		tlsClientCertificate, err := tls.LoadX509KeyPair(cfg.TLSClientCertPath, cfg.TLSClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load Fabric TLS client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{tlsClientCertificate}
	}
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancelConnect()
	connection, err := grpc.DialContext(
		connectCtx,
		cfg.GatewayEndpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithBlock(),
		grpc.WithNoProxy(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to Fabric Gateway %s: %w", cfg.GatewayEndpoint, err)
	}

	gw, err := client.Connect(
		clientIdentity,
		client.WithSign(sign),
		client.WithHash(hash.SHA256),
		client.WithClientConnection(connection),
		client.WithEvaluateTimeout(cfg.EvaluateTimeout),
		client.WithEndorseTimeout(cfg.EndorseTimeout),
		client.WithSubmitTimeout(cfg.SubmitTimeout),
		client.WithCommitStatusTimeout(cfg.CommitStatusTimeout),
	)
	if err != nil {
		connection.Close()
		return nil, fmt.Errorf("create Fabric Gateway session: %w", err)
	}

	network := gw.GetNetwork(cfg.ChannelName)
	contract := network.GetContractWithName(cfg.ChaincodeName, cfg.ContractName)
	fabricClient := &FabricClient{cfg: cfg, connection: connection, gateway: gw, contract: contract}

	if cfg.StartupCheck {
		metadataContract := network.GetContractWithName(cfg.ChaincodeName, "org.hyperledger.fabric")
		metadata, err := metadataContract.EvaluateTransaction("GetMetadata")
		if err != nil {
			fabricClient.Close()
			return nil, fmt.Errorf("Fabric startup check failed for channel=%s chaincode=%s: %w", cfg.ChannelName, cfg.ChaincodeName, err)
		}
		requiredTransactions := []string{
			"IssueCertificate",
			"ReadCertificate",
			"UpdateCertificate",
			"RevokeCertificate",
			"DeleteCertificate",
			"GetCertificateByHash",
			"GetCertificateStatus",
			"GetCertificateHistory",
		}
		missingTransactions := make([]string, 0)
		for _, transaction := range requiredTransactions {
			if !bytes.Contains(metadata, []byte(`"`+transaction+`"`)) {
				missingTransactions = append(missingTransactions, transaction)
			}
		}
		if len(missingTransactions) > 0 {
			fabricClient.Close()
			return nil, fmt.Errorf(
				"Fabric chaincode %s contract %s is missing required transactions: %s; deploy the new chaincode image and increment its sequence",
				cfg.ChaincodeName,
				cfg.ContractName,
				strings.Join(missingTransactions, ", "),
			)
		}
	}

	log.Printf("connected to Fabric Gateway %s (MSP=%s channel=%s chaincode=%s contract=%s)", cfg.GatewayEndpoint, cfg.MSPID, cfg.ChannelName, cfg.ChaincodeName, cfg.ContractName)
	return fabricClient, nil
}

func validateFabricConfig(cfg *FabricConfig) error {
	required := map[string]string{
		"FABRIC_MSP_ID":           cfg.MSPID,
		"FABRIC_GATEWAY_ENDPOINT": cfg.GatewayEndpoint,
		"FABRIC_CHANNEL_NAME":     cfg.ChannelName,
		"FABRIC_CHAINCODE_NAME":   cfg.ChaincodeName,
		"FABRIC_CONTRACT_NAME":    cfg.ContractName,
		"FABRIC_TLS_CERT_PATH":    cfg.TLSCertPath,
		"FABRIC_CLIENT_CERT_PATH": cfg.ClientCertPath,
		"FABRIC_CLIENT_KEY_PATH":  cfg.ClientKeyPath,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	return nil
}

func verifyCertificateMatchesKey(certificate *x509.Certificate, privateKey crypto.PrivateKey) error {
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("Fabric client private key does not implement crypto.Signer")
	}
	certificatePublicKey, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal Fabric certificate public key: %w", err)
	}
	privatePublicKey, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return fmt.Errorf("marshal Fabric private key public component: %w", err)
	}
	if !bytes.Equal(certificatePublicKey, privatePublicKey) {
		return fmt.Errorf("Fabric client certificate and private key do not match")
	}
	return nil
}

func waitForReady(connection *grpc.ClientConn, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	connection.Connect()
	for {
		state := connection.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown {
			return fmt.Errorf("gRPC connection shut down")
		}
		if !connection.WaitForStateChange(ctx, state) {
			return ctx.Err()
		}
	}
}

func (fc *FabricClient) Close() error {
	if fc == nil {
		return nil
	}
	if fc.gateway != nil {
		fc.gateway.Close()
	}
	if fc.connection != nil {
		return fc.connection.Close()
	}
	return nil
}

func (fc *FabricClient) submit(transactionName string, args ...string) ([]byte, error) {
	if fc == nil || fc.contract == nil {
		return nil, fmt.Errorf("fabric contract is not available")
	}
	return fc.contract.SubmitTransaction(transactionName, args...)
}

func (fc *FabricClient) evaluate(transactionName string, args ...string) ([]byte, error) {
	if fc == nil || fc.contract == nil {
		return nil, fmt.Errorf("fabric contract is not available")
	}
	return fc.contract.EvaluateTransaction(transactionName, args...)
}

func (fc *FabricClient) IssueCertificate(cert any) (string, error) {
	certBytes, err := json.Marshal(cert)
	if err != nil {
		return "", fmt.Errorf("marshal certificate: %w", err)
	}
	result, err := fc.submit("IssueCertificate", string(certBytes))
	if err != nil {
		return "", fmt.Errorf("invoke IssueCertificate: %w", err)
	}
	return string(result), nil
}

func (fc *FabricClient) IssueEDiplomaBatch(batch models.EDiplomaBatchOnChain) (string, error) {
	batchBytes, err := json.Marshal(batch)
	if err != nil {
		return "", fmt.Errorf("marshal eDiploma batch: %w", err)
	}
	result, err := fc.submit("IssueEDiplomaBatch", string(batchBytes))
	if err != nil {
		errMessage := err.Error()
		if strings.Contains(errMessage, "already exists") {
			return "", common.ErrAlreadyOnChain
		}
		return "", fmt.Errorf("invoke IssueEDiplomaBatch: %w", err)
	}
	return string(result), nil
}

// ErrNotRevokedOnChain means the ledger holds no revocation for the diploma.
var ErrNotRevokedOnChain = errors.New("diploma is not revoked on the ledger")

func (fc *FabricClient) RevokeEDiplomaBatch(revocation models.EDiplomaRevocationOnChain) (string, error) {
	payload, err := json.Marshal(revocation)
	if err != nil {
		return "", fmt.Errorf("marshal eDiploma revocation: %w", err)
	}
	result, err := fc.submit("RevokeEDiplomaBatch", string(payload))
	if err != nil {
		return "", fmt.Errorf("invoke RevokeEDiplomaBatch: %w", err)
	}
	return string(result), nil
}

func (fc *FabricClient) GetEDiplomaRevocation(diplomaID string) (*models.EDiplomaRevocationStatus, error) {
	result, err := fc.evaluate("GetEDiplomaRevocation", diplomaID)
	if err != nil {
		if strings.Contains(err.Error(), "is not revoked") {
			return nil, ErrNotRevokedOnChain
		}
		return nil, fmt.Errorf("invoke GetEDiplomaRevocation: %w", err)
	}
	var status models.EDiplomaRevocationStatus
	if err := json.Unmarshal(result, &status); err != nil {
		return nil, fmt.Errorf("unmarshal eDiploma revocation: %w", err)
	}
	return &status, nil
}

func (fc *FabricClient) IssueCertificateBatch(batch models.CertificateBatchOnChain) (string, error) {
	batchBytes, err := json.Marshal(batch)
	if err != nil {
		return "", fmt.Errorf("marshal certificate batch: %w", err)
	}
	result, err := fc.submit("IssueCertificateBatch", string(batchBytes))
	if err != nil {
		errMessage := err.Error()
		if strings.Contains(errMessage, "already exists") {
			return "", common.ErrAlreadyOnChain
		}
		return "", fmt.Errorf("invoke IssueCertificateBatch: %w", err)
	}
	return string(result), nil
}

func (fc *FabricClient) GetCertificateByID(certID string) (*models.CertificateOnChain, error) {
	result, err := fc.evaluate("ReadCertificate", certID)
	if err != nil {
		return nil, fmt.Errorf("invoke ReadCertificate: %w", err)
	}
	var certificate models.CertificateOnChain
	if err := json.Unmarshal(result, &certificate); err != nil {
		return nil, fmt.Errorf("unmarshal certificate: %w", err)
	}
	return &certificate, nil
}

func (fc *FabricClient) UpdateCertificate(cert any) error {
	certBytes, err := json.Marshal(cert)
	if err != nil {
		return fmt.Errorf("marshal certificate: %w", err)
	}
	if _, err := fc.submit("UpdateCertificate", string(certBytes)); err != nil {
		return fmt.Errorf("invoke UpdateCertificate: %w", err)
	}
	return nil
}

func (fc *FabricClient) GetEDiplomaBatch(batchID string) (*models.EDiplomaBatchOnChain, error) {
	result, err := fc.evaluate("ReadEDiplomaBatch", batchID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, fmt.Errorf("batch %s does not exist", batchID)
		}
		return nil, fmt.Errorf("invoke ReadEDiplomaBatch: %w", err)
	}
	var batch models.EDiplomaBatchOnChain
	if err := json.Unmarshal(result, &batch); err != nil {
		return nil, fmt.Errorf("unmarshal eDiploma batch: %w", err)
	}
	return &batch, nil
}

func (fc *FabricClient) GetCertificateBatch(batchID string) (*models.CertificateBatchOnChain, error) {
	result, err := fc.evaluate("ReadCertificateBatch", batchID)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, fmt.Errorf("batch %s does not exist", batchID)
		}
		return nil, fmt.Errorf("invoke ReadCertificateBatch: %w", err)
	}
	var batch models.CertificateBatchOnChain
	if err := json.Unmarshal(result, &batch); err != nil {
		return nil, fmt.Errorf("unmarshal certificate batch: %w", err)
	}
	return &batch, nil
}

func (fc *FabricClient) RevokeCertificate(certID, reason string) (string, error) {
	result, err := fc.submit("RevokeCertificate", certID, reason)
	if err != nil {
		return "", fmt.Errorf("invoke RevokeCertificate: %w", err)
	}
	return string(result), nil
}

func (fc *FabricClient) DeleteCertificate(certID, reason string) (string, error) {
	result, err := fc.submit("DeleteCertificate", certID, reason)
	if err != nil {
		return "", fmt.Errorf("invoke DeleteCertificate: %w", err)
	}
	return string(result), nil
}

func (fc *FabricClient) GetCertificateByHash(certHash string) (*models.CertificateOnChain, error) {
	result, err := fc.evaluate("GetCertificateByHash", certHash)
	if err != nil {
		return nil, fmt.Errorf("invoke GetCertificateByHash: %w", err)
	}
	var certificate models.CertificateOnChain
	if err := json.Unmarshal(result, &certificate); err != nil {
		return nil, fmt.Errorf("unmarshal certificate by hash: %w", err)
	}
	return &certificate, nil
}

func (fc *FabricClient) GetCertificateStatus(certID string) (*models.CertificateStatusResponse, error) {
	result, err := fc.evaluate("GetCertificateStatus", certID)
	if err != nil {
		return nil, fmt.Errorf("invoke GetCertificateStatus: %w", err)
	}
	var status models.CertificateStatusResponse
	if err := json.Unmarshal(result, &status); err != nil {
		return nil, fmt.Errorf("unmarshal certificate status: %w", err)
	}
	return &status, nil
}

func (fc *FabricClient) GetCertificateHistory(certID string) ([]models.CertificateHistoryEntry, error) {
	result, err := fc.evaluate("GetCertificateHistory", certID)
	if err != nil {
		return nil, fmt.Errorf("invoke GetCertificateHistory: %w", err)
	}
	var history []models.CertificateHistoryEntry
	if err := json.Unmarshal(result, &history); err != nil {
		return nil, fmt.Errorf("unmarshal certificate history: %w", err)
	}
	return history, nil
}
