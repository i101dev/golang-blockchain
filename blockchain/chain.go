package blockchain

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/i101dev/blockchain-Tensor/types"
	"github.com/i101dev/blockchain-Tensor/util"
)

const (
	DB_PATH = "../tmp/blocks_%d"
	DB_FILE = "../tmp/blocks/MANIFEST"

	LAST_HASH_KEY = "lastHash"

	GENESIS_DATA = "** >> GENESIS TRANSACTION << **"
)

type NullLogger struct{}

func (l *NullLogger) Errorf(string, ...interface{})   {}
func (l *NullLogger) Warningf(string, ...interface{}) {}
func (l *NullLogger) Infof(string, ...interface{})    {}
func (l *NullLogger) Debugf(string, ...interface{})   {}

type Blockchain struct {
	Path     string
	LastHash []byte
	Database *badger.DB
}

func OpenDB(chain *Blockchain) *badger.DB {

	opts := badger.DefaultOptions(chain.Path)
	opts.Logger = &NullLogger{}

	db, err := badger.Open(opts)
	util.Handle(err, "Open BadgerDB 1")

	chain.Database = db

	return db
}

func NewChain(nodeID uint16) *Blockchain {

	path := fmt.Sprintf(DB_PATH, nodeID)
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		log.Fatal(fmt.Sprintf("Error Creating Dir: %s", err))
	}

	return &Blockchain{
		Path: path,
	}
}

func (chain *Blockchain) CloseDB() {

	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		err := chain.Database.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}

	err := chain.Database.Close()
	util.Handle(err, "Close 1")
}

func (chain *Blockchain) GetLastHash(db *badger.DB) ([]byte, error) {

	var lastHash []byte

	err := db.View(func(txn *badger.Txn) error {

		// ----------------------------------------------------------
		item, err := txn.Get([]byte(LAST_HASH_KEY))
		if err != nil {
			return fmt.Errorf("failed to get last hash from bytes")
		}

		// ----------------------------------------------------------
		lastHash, err = item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed item ValueCopy")
		}

		return nil
	})

	return lastHash, err
}

func (chain *Blockchain) PostBlockToDB(lastHash []byte, newBlock *Block, db *badger.DB) error {

	return db.Update(func(dbTXN *badger.Txn) error {

		// ----------------------------------------------------------
		serializedBlock, err := newBlock.Serialize()
		if err != nil {
			return err
		}

		// ----------------------------------------------------------
		err = dbTXN.Set(newBlock.Hash, serializedBlock)
		if err != nil {
			return fmt.Errorf("failed to set serialized block in database")
		}

		// ----------------------------------------------------------
		err = dbTXN.Set([]byte(LAST_HASH_KEY), newBlock.Hash)
		if err != nil {
			return fmt.Errorf("failed to set LAST_HASH in database")
		}

		chain.LastHash = newBlock.Hash

		return nil
	})
}

func (chain *Blockchain) AddBlock(data *types.AddBlockReq) error {

	db := OpenDB(chain)
	defer chain.CloseDB()

	// ----------------------------------------------------------
	lastHash, err := chain.GetLastHash(db)
	if err != nil {
		return err
	}

	// ----------------------------------------------------------
	newBlock, err := CreateBlock([]*Transaction{}, lastHash)
	if err != nil {
		return err
	}

	return chain.PostBlockToDB(lastHash, newBlock, db)
}

func (chain *Blockchain) GetBlockByHash(db *badger.DB, hash []byte) (*Block, error) {

	var block *Block

	err := db.View(func(txn *badger.Txn) error {

		// ----------------------------------------------------------
		item, err := txn.Get(hash)
		if err != nil {
			return fmt.Errorf("HASH NOT FOUND")
		}

		// ----------------------------------------------------------
		encodedBlock, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("FAILED TO ENCODE")
		}

		// ----------------------------------------------------------
		block, err = Deserialize(encodedBlock)

		return err
	})

	return block, err
}

// func InitBlockChain_NEW(ownerAddress string, nodeID uint16) (*Blockchain, error) {

// 	if DBexists() {
// 		return nil, fmt.Errorf("Blockchain already exists")
// 	}

// 	newChain := NewChain(nodeID)
// 	database := OpenDB(newChain)
// 	defer newChain.CloseDB()

// 	err := database.Update(func(txn *badger.Txn) error {

// 		// --------------------------------------------------------------
// 		cbtx := CoinbaseTX(ownerAddress, GENESIS_DATA)
// 		genesis, err := Genesis(cbtx)
// 		if err != nil {
// 			return err
// 		}

// 		// --------------------------------------------------------------
// 		serializedGenesisBlock, err := genesis.Serialize()
// 		if err != nil {
// 			return err
// 		}

// 		// --------------------------------------------------------------
// 		err = txn.Set(genesis.Hash, serializedGenesisBlock)
// 		if err != nil {
// 			return fmt.Errorf("*** >> ERROR: failed to set genesis hash")
// 		}

// 		// --------------------------------------------------------------
// 		err = txn.Set([]byte(LAST_HASH_KEY), genesis.Hash)
// 		if err != nil {
// 			return fmt.Errorf("*** >> ERROR: failed to set LAST_HASH_KEY")

// 		}

// 		fmt.Println("\n*** >>> New chain initialized && Genesis block created <<< ***")

// 		newChain.LastHash = genesis.Hash

// 		return nil

// 	})

// 	return newChain, err
// }

// func ContinueBlockChain(address string, nodeID uint16) (*Blockchain, error) {

// 	if !DBexists() {
// 		return nil, fmt.Errorf("*** >>> No existing blockchain found, create one")
// 	}

// 	newChain := NewChain(nodeID)
// 	db := OpenDB(newChain)
// 	defer newChain.CloseDB()

// 	lastHash, err := newChain.GetLastHash(db)

// 	newChain.LastHash = lastHash

// 	return newChain, err
// }

func InitBlockchain(address string, nodeID uint16) (*Blockchain, error) {

	if DBexists() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	path := fmt.Sprintf(DB_PATH, nodeID)

	// Ensure the directory exists ---------------------------
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		log.Panicf(fmt.Sprintf("Error Creating Dir: %s", err))
	}

	newChain := &Blockchain{
		Path: path,
	}

	// -------------------------------------------------------
	db := OpenDB(newChain)
	defer newChain.CloseDB()

	var lastHash []byte
	err := db.Update(func(dbTXN *badger.Txn) error {

		if _, err := dbTXN.Get([]byte(LAST_HASH_KEY)); err == badger.ErrKeyNotFound {

			// ----------------------------------------------------------
			cbtx := CoinbaseTX(address, GENESIS_DATA)
			genesis, err := Genesis(cbtx)
			if err != nil {
				return err
			}

			// ----------------------------------------------------------
			serializedBlock, err := genesis.Serialize()
			if err != nil {
				return err
			}

			// ----------------------------------------------------------
			err = dbTXN.Set(genesis.Hash, serializedBlock)
			if err != nil {
				return fmt.Errorf("failed to set serialized block in database")
			}

			// ----------------------------------------------------------
			err = dbTXN.Set([]byte(LAST_HASH_KEY), genesis.Hash)
			if err != nil {
				return fmt.Errorf("failed to set LAST_HASH in database")
			}

			lastHash = genesis.Hash

			return nil
		}

		last, err := newChain.GetLastHash(db)

		lastHash = last

		return err
	})

	newChain.LastHash = lastHash

	return newChain, err
}

func DBexists() bool {
	if _, err := os.Stat(DB_FILE); os.IsNotExist(err) {
		return false
	}
	return true
}

// -----------------------------------------------------------------------
type BlockchainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
	Chain       *Blockchain
}

func (chain *Blockchain) NewIterator() *BlockchainIterator {
	return &BlockchainIterator{
		CurrentHash: chain.LastHash,
		Database:    chain.Database,
		Chain:       chain,
	}
}

func (iter *BlockchainIterator) IterateNext() (*Block, error) {

	block, err := iter.Chain.GetBlockByHash(iter.Database, iter.CurrentHash)
	if err != nil {
		return nil, err
	}

	iter.CurrentHash = block.PrevHash

	return block, nil
}
