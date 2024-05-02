package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/i101dev/golang-blockchain/blockchain"
	"github.com/vrecan/death"
)

const (
	protocol      = "tcp"
	version       = 1
	commandLength = 12
)

var (
	nodeAddress     string
	minerAddress    string
	KnownNodes      = []string{"localhost:3000"}
	blocksInTransit = [][]byte{}
	memoryPool      = make(map[string]blockchain.Transaction)
)

type Addr struct {
	AddrList []string
}

type Block struct {
	AddrFrom string
	Block    []byte
}

type GetBlocks struct {
	AddrFrom string
}

type GetData struct {
	AddrFrom string
	Type     string
	ID       []byte
}

type Inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type Tx struct {
	AddrFrom    string
	Transaction []byte
}

type Version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

func CmdToBytes(cmd string) []byte {

	var bytes [commandLength]byte

	for i, c := range cmd {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

func BytesToCmd(bytes []byte) string {
	var cmd []byte

	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}

	return string(cmd)
}

func ExtractCmd(request []byte) []byte {
	return request[:commandLength]
}

func SendAddr(address string) {

	nodes := Addr{KnownNodes}
	nodes.AddrList = append(nodes.AddrList, nodeAddress)

	payload := GobEncode(nodes)
	request := append(CmdToBytes("addr"), payload...)

	SendData(address, request)
}

func SendBlock(addr string, b *blockchain.Block) {

	data := Block{nodeAddress, b.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("block"), payload...)

	SendData(addr, request)
}

func SendInv(address string, kind string, items [][]byte) {
	inventory := Inv{nodeAddress, kind, items}
	payload := GobEncode(inventory)
	request := append(CmdToBytes("inv"), payload...)

	SendData(address, request)
}

func SendTx(address string, txn *blockchain.Transaction) {
	data := Tx{nodeAddress, txn.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("tx"), payload...)

	SendData(address, request)
}

func SendVersion(address string, chain *blockchain.Blockchain) {
	bestHeight := chain.GetBestHeight()
	payload := GobEncode(Version{version, bestHeight, nodeAddress})
	request := append(CmdToBytes("version"), payload...)

	SendData(address, request)
}

func SendGetBlocks(address string) {
	payload := GobEncode(GetBlocks{nodeAddress})
	request := append(CmdToBytes("getblocks"), payload...)
	SendData(address, request)
}

func SendGetData(address string, kind string, id []byte) {
	payload := GobEncode(GetData{nodeAddress, kind, id})
	request := append(CmdToBytes("getdata"), payload...)
	SendData(address, request)
}

func SendData(addr string, data []byte) {

	conn, err := net.Dial(protocol, addr)

	if err != nil {
		fmt.Printf("[%s] is not available\n", addr)
		var updatedNodes []string

		for _, node := range KnownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}

		KnownNodes = updatedNodes

		return
	}

	defer conn.Close()

	_, err = io.Copy(conn, bytes.NewReader(data))

	if err != nil {
		log.Panic(err)
	}
}

func GobEncode(data interface{}) []byte {

	// Designate the target location for data to be encoded
	var buff bytes.Buffer

	// Build the machine that will do the encoding
	enc := gob.NewEncoder(&buff)

	// Use the machine, and deliver the encoded `data` to `buff`
	if err := enc.Encode(data); err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func CloseDB(chain *blockchain.Blockchain) {

	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}

func HandleConnection(conn net.Conn, chain *blockchain.Blockchain) {

	req, err := io.ReadAll(conn)

	defer conn.Close()

	if err != nil {
		log.Panic(err)
	}

	command := BytesToCmd(req[:commandLength])

	fmt.Printf("Received [%s] command\n", command)

	switch command {
	//
	case "addr":
		HandleAddr(req)
	case "block":
		HandleBlock(req, chain)
	case "inv":
		HandleInv(req, chain)
	case "getblocks":
		HandleGetBlocks(req, chain)
	case "getdata":
		HandleGetData(req, chain)
	case "tx":
		HandleTx(req, chain)
	case "version":
		HandleVersion(req, chain)
		//
	default:
		fmt.Println("Unknown command")
	}
}

func HandleAddr(request []byte) {

	var buff bytes.Buffer
	var payload Addr

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	KnownNodes = append(KnownNodes, payload.AddrList...)

	fmt.Printf("\n*** >>> there are [%d] known nodes\n", len(KnownNodes))

	RequestBlocks()
}

func HandleBlock(request []byte, chain *blockchain.Blockchain) {

	var buff bytes.Buffer
	var payload Block

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	blockData := payload.Block
	block := blockchain.Deserialize(blockData)

	fmt.Println("\n*** >>> Received a new block")

	chain.AddBlock(block)

	fmt.Printf("Added block %x\n", block.Hash)

	if len(blocksInTransit) > 0 {
		//
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
		//
	} else {
		//
		UTXOset := blockchain.UTXOSet{Blockchain: chain}
		UTXOset.Reindex()
	}
}

func HandleGetBlocks(request []byte, chain *blockchain.Blockchain) {

	var buff bytes.Buffer
	var payload GetBlocks

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	blocks := chain.GetBlockHashes()

	SendInv(payload.AddrFrom, "block", blocks)
}

func HandleGetData(request []byte, chain *blockchain.Blockchain) {

	var buff bytes.Buffer
	var payload GetData

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	if payload.Type == "block" {
		block, err := chain.GetBlock([]byte(payload.ID))
		if err != nil {
			return
		}

		SendBlock(payload.AddrFrom, &block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]

		SendTx(payload.AddrFrom, &tx)
	}
}

func HandleVersion(request []byte, chain *blockchain.Blockchain) {

	var buff bytes.Buffer
	var payload Version

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight

	if bestHeight < otherHeight {
		SendGetBlocks(payload.AddrFrom)

	} else if bestHeight > otherHeight {
		SendVersion(payload.AddrFrom, chain)
	}

	if !NodeIsKnown(payload.AddrFrom) {
		KnownNodes = append(KnownNodes, payload.AddrFrom)
	}
}

func HandleTx(request []byte, chain *blockchain.Blockchain) {

	var buff bytes.Buffer
	var payload Tx

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	txData := payload.Transaction
	tx := blockchain.DeserializeTransaction(txData)

	memoryPool[hex.EncodeToString(tx.ID)] = tx

	fmt.Printf("%s, %d\n", nodeAddress, len(memoryPool))

	if nodeAddress == KnownNodes[0] {
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.AddrFrom {
				SendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		if len(memoryPool) >= 2 && len(minerAddress) > 0 {
			MineTx(chain)
		}
	}
}

func HandleInv(request []byte, chain *blockchain.Blockchain) {

	var buff bytes.Buffer
	var payload Inv

	buff.Write(request[commandLength:])

	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)

	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("\n*** >>> Received inventory with %d %s\n", len(payload.Items), payload.Type)

	if payload.Type == "block" {

		blocksInTransit = payload.Items
		blockHash := payload.Items[0]

		SendGetData(payload.AddrFrom, "block", blockHash)

		newInTransit := [][]byte{}

		for _, b := range blocksInTransit {
			if bytes.Compare(b, blockHash) != 0 {
				newInTransit = append(newInTransit, b)
			}
		}

		blocksInTransit = newInTransit
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]

		if memoryPool[hex.EncodeToString(txID)].HashID == nil {
			SendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

func MineTx(chain *blockchain.Blockchain) {

	var txs []*blockchain.Transaction

	for id := range memoryPool {

		fmt.Printf("tx: %s\n", memoryPool[id].HashID)

		tx := memoryPool[id]

		if chain.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
		}
	}

	if len(txs) == 0 {
		fmt.Println("\n*** >>> All transactions are invalid")
		return
	}

	cbTx := blockchain.CoinbaseTx(minerAddress, "")
	txs = append(txs, cbTx)

	newBlock := chain.MineBlock(txs)
	UTXOset := blockchain.UTXOSet{chain}

	UTXOset.Reindex()

	fmt.Println("\n** >>> New block mined")

	for _, tx := range tx {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}

	for _, node := range KnownNodes {
		if node != nodeAddress {
			SendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}

	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

func NodeIsKnown(addr string) bool {
	for _, node := range KnownNodes {
		if node == addr {
			return true
		}
	}
	return false
}

func RequestBlocks() {
	for _, nodeAddr := range KnownNodes {
		SendGetBlocks(nodeAddr)
	}
}

func StartServer(nodeID string, minerAddr string) {

	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	minerAddress = minerAddr

	ln, err := net.Listen(protocol, nodeAddress)

	if err != nil {
		log.Panic(err)
	}

	defer ln.Close()

	chain := blockchain.ContinueBlockChain(nodeID)

	defer chain.Database.Close()

	go CloseDB(chain)

	if nodeAddress != KnownNodes[0] {
		SendVersion(KnownNodes[0], chain)
	}

	for {
		conn, err := ln.Accept()

		if err != nil {
			log.Panic(err)
		}

		go HandleConnection(conn, chain)
	}
}