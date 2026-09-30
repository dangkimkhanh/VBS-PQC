package main

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/vnkmasc/Kmasc/app/backend/pkg/blockchain"
)

func main() {
	_ = godotenv.Load()

	config := blockchain.NewFabricConfigFromEnv()
	client, err := blockchain.NewFabricClient(config)
	if err != nil {
		log.Fatalf("Fabric connection check failed: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Fabric connection close failed: %v", err)
		}
	}()

	log.Printf(
		"Fabric connection check passed: MSP=%s gateway=%s channel=%s chaincode=%s contract=%s",
		config.MSPID,
		config.GatewayEndpoint,
		config.ChannelName,
		config.ChaincodeName,
		config.ContractName,
	)
}
