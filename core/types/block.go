// Copyright 2014 The go-fiscobcos Authors
// This file is part of the go-fiscobcos library.
//
// The go-fiscobcos library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-fiscobcos library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-fiscobcos library. If not, see <http://www.gnu.org/licenses/>.

// Package types contains data types related to FiscoBcos consensus.
package types

import (
	"github.com/chislab/go-fiscobcos/common"
	"github.com/chislab/go-fiscobcos/rlp"
	"golang.org/x/crypto/sha3"
)

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

type ClientVersion struct {
	BuildTime        string `json:"Build Time"`
	BuildType        string `json:"Build Type"`
	ChainId          string `json:"Chain Id"`
	Version          string `json:"FISCO-BCOS Version"`
	Branch           string `json:"Git Branch"`
	CommitHash       string `json:"Git Commit Hash"`
	SupportedVersion string `json:"Supported Version"`
}


type Block struct {
	DbHash       string        `json:"dbHash"`
	ExtraData    []interface{} `json:"extraData"`
	GasLimit     string        `json:"gasLimit"`
	GasUsed      string        `json:"gasUsed"`
	Hash         string        `json:"hash"`
	LogsBloom    string        `json:"logsBloom"`
	Number       string        `json:"number"`
	ParentHash   string        `json:"parentHash"`
	ReceiptsRoot string        `json:"receiptsRoot"`
	Sealer       string        `json:"sealer"`
	SealerList   []string      `json:"sealerList"`
	StateRoot    string        `json:"stateRoot"`
	Timestamp    string        `json:"timestamp"`
	Transactions [] BlockTx `json:"transactions"`
	TransactionsRoot string `json:"transactionsRoot"`
}

type BlockTx struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Value            string `json:"value"`
}

type TotalTransactionCount struct {
	BlockNumber string `json:"blockNumber"`
	TxSum       string `json:"txSum"`
}

type TransactionByHash struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Value            string `json:"value"`
}


type PeerStatus struct {
	IPAndPort string        `json:"IPAndPort"`
	Topic     []interface{} `json:"Topic"`
	NodeID    string        `json:"nodeId"`
}

type PendingTx struct {
	From     common.Hash `json:"from"`
	Gas      string      `json:"gas"`
	GasPrice string      `json:"gasPrice"`
	Hash     common.Hash `json:"hash"`
	Input    string      `json:"input"`
	Nonce    string      `json:"nonce"`
	To       common.Hash `json:"to"`
	Value    string      `json:"value"`
}


type SyncStatus struct {
	BlockNumber        int    `json:"blockNumber"`
	GenesisHash        string `json:"genesisHash"`
	IsSyncing          bool   `json:"isSyncing"`
	KnownHighestNumber int    `json:"knownHighestNumber"`
	KnownLatestHash    string `json:"knownLatestHash"`
	LatestHash         string `json:"latestHash"`
	NodeID             string `json:"nodeId"`
	Peers              []Peer `json:"peers"`
	ProtocolID         int    `json:"protocolId"`
	TxPoolSize         string `json:"txPoolSize"`
}


type Peer struct {
	BlockNumber int         `json:"blockNumber"`
	GenesisHash common.Hash `json:"genesisHash"`
	LatestHash  common.Hash `json:"latestHash"`
	NodeID      string      `json:"nodeId"`
}
