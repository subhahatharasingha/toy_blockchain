package storage

import (
	"encoding/json"
	"os"

	"toy-blockchain/blockchain"
)

// Save writes blockchain data to disk at the specified path.
func Save(bc *blockchain.Blockchain, path string) error {
	data, err := json.MarshalIndent(bc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Load reads blockchain data from disk at the specified path.
func Load(path string) (*blockchain.Blockchain, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return blockchain.NewBlockchain(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var bc blockchain.Blockchain
	err = json.Unmarshal(data, &bc)
	if err != nil {
		return nil, err
	}

	return &bc, nil
}