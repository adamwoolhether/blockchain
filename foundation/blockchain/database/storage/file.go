package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// Ardan represents the storage implementation for reading and storing blocks
// in their own separate files on disk. This implements the database.Storage
// interface.
type Ardan struct {
	dbPath string
}

// NewArdan constructs an Ardan value for use.
func NewArdan(dbPath string) (*Ardan, error) {
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, err
	}

	return &Ardan{dbPath: dbPath}, nil
}

// Close in this implementation has nothing to do since a new file is
// written to disk for each now Block and then immediately closed.
func (ard *Ardan) Close() error {
	return nil
}

// Write takes the specified database blocks and stores it on disk in a
// file labeled with the Block number.
func (ard *Ardan) Write(dbBlock database.Block) error {

	// Need to convert the Block to the storage format.
	block := NewBlock(dbBlock)

	// Marshal the Block for writing to disk in a more human readable format.
	data, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		return err
	}

	// Create a new file for this Block and name it based on the Block number.
	f, err := os.OpenFile(ard.getPath(block.Header.Number), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write the new Block to disk.
	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}

// GetBlock searches the blockchain on disk to locate and return the
// contents of the specified Block by number.
func (ard *Ardan) GetBlock(num uint64) (database.Block, error) {

	// Open the Block file for the specified number.
	f, err := os.OpenFile(ard.getPath(num), os.O_RDONLY, 0600)
	if err != nil {
		return database.Block{}, err
	}
	defer f.Close()

	// Decode the contents of the Block.
	var block Block
	if err := json.NewDecoder(f).Decode(&block); err != nil {
		return database.Block{}, err
	}

	// Return the Block as a database Block.
	return ToDatabaseBlock(block)
}

// ForEach returns an iterator to walk through all the blocks on
// disk starting with Block number 1.
func (ard *Ardan) ForEach() database.Iterator {
	return &ArdanIterator{storage: ard}
}

// Reset will clear out the blockchain on disk.
func (ard *Ardan) Reset() error {
	return nil
}

// getPath forms the path to the specified Block.
func (ard *Ardan) getPath(blockNum uint64) string {
	name := strconv.FormatUint(blockNum, 10)
	return path.Join(ard.dbPath, fmt.Sprintf("%s.json", name))
}

// ArdanIterator represents the iteration implementation for walking
// through and reading blocks on disk. This implements the database
// Iterator interface.
type ArdanIterator struct {
	storage *Ardan // Access to the Ardan storage API.
	current uint64 // Currenet Block number being iterated over.
	eoc     bool   // Represents the iterator is at the end of the chain.
}

// Next retrieves the next Block from disk.
func (ai *ArdanIterator) Next() (database.Block, error) {
	if ai.eoc {
		return database.Block{}, errors.New("end of chain")
	}

	ai.current++
	block, err := ai.storage.GetBlock(ai.current)
	if errors.Is(err, fs.ErrNotExist) {
		ai.eoc = true
	}

	return block, nil
}

// Done returns the end of chain value.
func (ai *ArdanIterator) Done() bool {
	return ai.eoc
}
