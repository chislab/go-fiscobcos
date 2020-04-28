package channel

import (
	"github.com/chislab/go-fiscobcos/common"
)

const (
	TypeRPCRequest        = 0x12
	TypeHeartBeat         = 0x13
	TypeHandshake         = 0x14
	TypeRegisterEventLog  = 0x15
	TypeTransactionNotify = 0x1000
	TypeBlockNotify       = 0x1001
	TypeEventLog          = 0x1002
)

type RegisterEventLogRequest struct {
	FromBlock string   `json:"fromBlock"`
	ToBlock   string   `json:"toBlock"`
	Addresses []string `json:"addresses"`
	Topics    []string `json:"topics"`
	GroupID   string   `json:"groupID"`
	FilterID  string   `json:"filterID"`
}

type logEntry struct {
	// Consensus fields:
	// address of the contract that generated the event
	Address common.Address `json:"address" gencodec:"required"`
	// list of topics provided by the contract.
	Topics []common.Hash `json:"topics" gencodec:"required"`
	// supplied by the contract, usually ABI-encoded
	Data string `json:"data" gencodec:"required"`

	// Derived fields. These fields are filled in by the node
	// but not secured by consensus.
	// block in which the transaction was included
	BlockNumber string `json:"blockNumber"`
	// hash of the transaction
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
	// index of the transaction in the block
	TxIndex string `json:"transactionIndex" gencodec:"required"`
	// hash of the block in which the transaction was included
	BlockHash common.Hash `json:"blockHash"`
	// index of the log in the block
	Index string `json:"logIndex" gencodec:"required"`

	// The Removed field is true if this log was reverted due to a chain reorganisation.
	// You must pay attention to this field if you receive logs through a filter query.
	Removed bool `json:"removed"`
}

type EventLogResponse struct {
	FilterID string     `json:"filterID"`
	Logs     []logEntry `json:"logs"`
	Result   int        `json:"result"`
}
