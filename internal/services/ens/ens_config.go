package ens

import (
	"os"
)

type ENSConfig struct {
	RegistryAddress       string
	ControllerAddress     string
	PublicResolverAddress string
	NetworkName           string
}

func LoadENSConfig() ENSConfig {
	testnet := os.Getenv("TESTNET_MODE") == "true"

	if testnet {
		return ENSConfig{
			RegistryAddress:       "0x00000000000C2E074eC69A0dFb2997BA6C7d2e1e", // ENS Registry on Sepolia
			ControllerAddress:     "0xF023fC1C494c8aD7d0A16bCD022a5d229a77F86b", // ETHRegistrarController on Sepolia
			PublicResolverAddress: "0xDaaF96c344f63131acadD0Ea35170E7892d3dfBA", // Public Resolver on Sepolia
			NetworkName:           "sepolia",
		}
	}

	return ENSConfig{
		RegistryAddress:       "0x00000000000C2E074eC69A0dFb2997BA6C7d2e1e", // Mainnet Registry
		ControllerAddress:     "0x253553366Da8546fC250F225fe3d25d0C782303b", // Mainnet ETHRegistrarController
		PublicResolverAddress: "0x226159d592E2b063810a10Ebf6dcbADA94Ed68b8", // Mainnet Public Resolver
		NetworkName:           "mainnet",
	}
}
