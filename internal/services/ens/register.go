// I want to add a feature to my app that lets users register a .eth ENS name
// using MetaMask and Geth (Sepolia if testnet mode is enabled).
// The app should:
// 1. Check if the name is available using ETHRegistrarController
// 2. Handle the commit → wait → register ENS flow
// 3. Set the contenthash to point to an IPFS CID I already have
// 4. Optionally set the ETH address record to the user's wallet
// Use testnet contract addresses if TESTNET_MODE is true.
// Keep the logic modular and separate config/constants from logic.

package ens

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// CheckNameAvailability checks if a .eth name is available for registration
func CheckNameAvailability(ctx context.Context, client *ethclient.Client, config ENSConfig, name string) (bool, error) {
	// Validate the name
	if name == "" {
		return false, errors.New("name cannot be empty")
	}

	// Convert the controller address to common.Address
	controllerAddress := common.HexToAddress(config.ControllerAddress)

	// Create a new instance of the ETHRegistrarController contract
	controller, err := NewETHRegistrarController(controllerAddress, client)
	if err != nil {
		return false, fmt.Errorf("failed to create controller: %w", err)
	}

	// Create a new call options struct
	opts := &bind.CallOpts{
		Context: ctx,
	}

	// First check if the name is valid
	valid, err := controller.Valid(opts, name)
	if err != nil {
		return false, fmt.Errorf("failed to validate name: %w", err)
	}
	if !valid {
		return false, errors.New("invalid ENS name")
	}

	// Call the available function on the contract
	available, err := controller.Available(opts, name)
	if err != nil {
		return false, fmt.Errorf("failed to check availability: %w", err)
	}

	return available, nil
}

// GenerateCommitment creates a commitment hash for ENS name registration
func GenerateCommitment(name string, owner common.Address, secret [32]byte) (common.Hash, error) {
	// Create a new instance of the ETHRegistrarController contract
	// We only need this for the commitment hash generation
	controller, err := NewETHRegistrarController(common.Address{}, nil)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create controller: %w", err)
	}

	// Generate the commitment hash
	opts := &bind.CallOpts{}
	commitment, err := controller.MakeCommitment(
		opts,
		name,
		owner,
		big.NewInt(31536000), // 1 year duration
		secret,
		common.Address{}, // resolver (empty for default)
		[][]byte{},       // data (empty for default)
		false,            // no reverse record
		0,                // no owner controlled fuses
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to generate commitment: %w", err)
	}

	return commitment, nil
}

// SubmitCommitment submits a commitment to the ENS registrar
func SubmitCommitment(ctx context.Context, client *ethclient.Client, config ENSConfig, commitment common.Hash, auth *bind.TransactOpts) error {
	controllerAddress := common.HexToAddress(config.ControllerAddress)
	controller, err := NewETHRegistrarController(controllerAddress, client)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	// Submit the commitment transaction
	tx, err := controller.Commit(auth, commitment)
	if err != nil {
		return fmt.Errorf("failed to submit commitment: %w", err)
	}

	// Wait for the transaction to be mined
	_, err = bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("failed to mine commitment transaction: %w", err)
	}

	return nil
}

// WaitForCommitment waits for the minimum commitment age to pass
func WaitForCommitment(ctx context.Context, client *ethclient.Client, config ENSConfig) error {
	controllerAddress := common.HexToAddress(config.ControllerAddress)
	controller, err := NewETHRegistrarController(controllerAddress, client)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	// Get the minimum commitment age
	opts := &bind.CallOpts{Context: ctx}
	minAge, err := controller.MinCommitmentAge(opts)
	if err != nil {
		return fmt.Errorf("failed to get minimum commitment age: %w", err)
	}

	// Wait for the minimum age to pass
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(minAge.Uint64()) * time.Second):
		return nil
	}
}

// RegisterName registers an ENS name after the commitment period
func RegisterName(
	ctx context.Context,
	client *ethclient.Client,
	config ENSConfig,
	name string,
	owner common.Address,
	duration *big.Int,
	secret [32]byte,
	auth *bind.TransactOpts,
) error {
	controllerAddress := common.HexToAddress(config.ControllerAddress)
	controller, err := NewETHRegistrarController(controllerAddress, client)
	if err != nil {
		return fmt.Errorf("failed to create controller: %w", err)
	}

	// Register the name
	tx, err := controller.Register(
		auth,
		name,
		owner,
		duration,
		secret,
		common.Address{}, // resolver (empty for default)
		[][]byte{},       // data (empty for default)
		false,            // no reverse record
		0,                // no owner controlled fuses
	)
	if err != nil {
		return fmt.Errorf("failed to register name: %w", err)
	}

	// Wait for the transaction to be mined
	_, err = bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("failed to mine registration transaction: %w", err)
	}

	return nil
}

// RegisterENSName orchestrates the complete ENS registration flow
func RegisterENSName(
	ctx context.Context,
	client *ethclient.Client,
	config ENSConfig,
	name string,
	owner common.Address,
	auth *bind.TransactOpts,
) error {
	// 1. Check if the name is available
	available, err := CheckNameAvailability(ctx, client, config, name)
	if err != nil {
		return fmt.Errorf("failed to check name availability: %w", err)
	}
	if !available {
		return errors.New("name is not available for registration")
	}

	// 2. Generate a random secret for the commitment
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return fmt.Errorf("failed to generate secret: %w", err)
	}

	// 3. Generate and submit the commitment
	commitment, err := GenerateCommitment(name, owner, secret)
	if err != nil {
		return fmt.Errorf("failed to generate commitment: %w", err)
	}

	if err := SubmitCommitment(ctx, client, config, commitment, auth); err != nil {
		return fmt.Errorf("failed to submit commitment: %w", err)
	}

	// 4. Wait for the commitment period
	if err := WaitForCommitment(ctx, client, config); err != nil {
		return fmt.Errorf("failed to wait for commitment: %w", err)
	}

	// 5. Register the name
	// Default duration is 1 year (31536000 seconds)
	duration := big.NewInt(31536000)
	if err := RegisterName(ctx, client, config, name, owner, duration, secret, auth); err != nil {
		return fmt.Errorf("failed to register name: %w", err)
	}

	return nil
}
