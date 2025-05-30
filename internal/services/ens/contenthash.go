package ens

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ipfs/go-cid"
)

// EncodeIPFSCIDToContenthash takes a CID string and returns the ENS contenthash as a 0x-prefixed hex string.
func EncodeIPFSCIDToContenthash(cidStr string) (string, error) {
	c, err := cid.Decode(cidStr)
	if err != nil {
		return "", fmt.Errorf("failed to decode CID: %w", err)
	}

	// Per EIP-1577, the contenthash for IPFS is: 0xe3 + multihash bytes
	// 0xe3 is the multicodec code for ipfs-ns (see https://github.com/multiformats/multicodec/blob/master/table.csv)
	const ipfsNsCodec = 0xe3
	contenthash := append([]byte{ipfsNsCodec}, c.Bytes()...)
	return "0x" + hex.EncodeToString(contenthash), nil
}

// Example: Reads a JSON file with a 'cid' field and prints the encoded contenthash
func ExampleEncodeFromFile(jsonPath string) error {
	data, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	var meta struct {
		CID string `json:"cid"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	if meta.CID == "" {
		return fmt.Errorf("cid field is empty")
	}
	encoded, err := EncodeIPFSCIDToContenthash(meta.CID)
	if err != nil {
		return err
	}
	fmt.Println(encoded)
	return nil
}

// If you want to run this as a standalone program for testing:
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run contenthash.go <path-to-json>")
		os.Exit(1)
	}
	if err := ExampleEncodeFromFile(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// NameHash returns the ENS namehash of a domain, e.g. "indienode.eth".
func NameHash(name string) common.Hash {
	// Start with the empty hash (node = 0x00..00)
	var node common.Hash

	if name == "" {
		return node
	}

	// Split labels and iterate right-to-left
	labels := strings.Split(name, ".")
	for i := len(labels) - 1; i >= 0; i-- {
		label := labels[i]
		// keccak256 on the label
		labelHash := crypto.Keccak256Hash([]byte(label))
		// keccak256 on the concatenation of the previous node and this labelHash
		node = crypto.Keccak256Hash(node.Bytes(), labelHash.Bytes())
	}
	return node
}
