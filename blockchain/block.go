package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

type Block struct {
	Timestamp    int64
	Nonce        int
	PrevHash     []byte
	Hash         []byte
	Transactions []*Transaction
}

func (b *Block) Print() {

	fmt.Printf("\n> Hash:		%s", hex.EncodeToString(b.Hash))
	fmt.Printf("\n> PrevHash:	%x", b.PrevHash)
	fmt.Printf("\n\n> Nonce:	%d", b.Nonce)
	fmt.Printf("\n> Timestamp:	%d", b.Timestamp)

	pow := NewProof(b)
	isValid, _ := pow.Validate()

	fmt.Printf("\n> Valid Proof: 	%s", strconv.FormatBool(isValid))
	fmt.Println("\n\n### Transactions:")
	for _, t := range b.Transactions {
		t.Print()
	}
	fmt.Println()
	// fmt.Printf("\n%s", strings.Repeat("=", 80))
}

func (b *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp    int64          `json:"timestamp"`
		Nonce        int            `json:"nonce"`
		PrevHash     string         `json:"prev_hash"`
		Hash         string         `json:"hash"`
		Transactions []*Transaction `json:"transactions"`
	}{
		Timestamp:    b.Timestamp,
		Nonce:        b.Nonce,
		PrevHash:     hex.EncodeToString(b.PrevHash),
		Hash:         hex.EncodeToString(b.Hash),
		Transactions: b.Transactions,
	})
}

func (b *Block) UnmarshalJSON(data []byte) error {
	aux := struct {
		Timestamp    int64          `json:"timestamp"`
		Nonce        int            `json:"nonce"`
		PrevHash     string         `json:"prev_hash"`
		Hash         string         `json:"hash"`
		Transactions []*Transaction `json:"transactions"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	prevHash, err := hex.DecodeString(aux.PrevHash)
	if err != nil {
		return err
	}

	hash, err := hex.DecodeString(aux.Hash)
	if err != nil {
		return err
	}

	b.Timestamp = aux.Timestamp
	b.Nonce = aux.Nonce
	b.PrevHash = prevHash
	b.Hash = hash
	b.Transactions = aux.Transactions

	return nil
}

func (b *Block) Serialize() ([]byte, error) {

	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)

	if err := encoder.Encode(b); err != nil {
		return nil, fmt.Errorf("failed to encode block to bytes")
	}

	return res.Bytes(), nil
}

func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}

	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}

func Genesis(coinbase *Transaction) (*Block, error) {
	return CreateBlock([]*Transaction{coinbase}, []byte{})
}

func CreateBlock(txs []*Transaction, prevHash []byte) (*Block, error) {

	block := &Block{
		PrevHash:     prevHash,
		Hash:         []byte{},
		Transactions: txs,
		Nonce:        0,
	}

	pow := NewProof(block)
	nonce, hash, err := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce

	return block, err
}

func Deserialize(data []byte) (*Block, error) {

	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(data))

	if err := decoder.Decode(&block); err != nil {
		return nil, fmt.Errorf("failed to decode and deserialize bytes in to Block")
	}

	return &block, nil
}
