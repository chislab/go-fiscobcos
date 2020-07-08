package client

import (
	"encoding/json"
	"fmt"

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

type OnBlockFunc func(groupID uint64, blockNumber uint64)

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

type BlockNotifyResponse struct {
	GroupID     string `json:"groupID"`
	BlockNumber string `json:"blockNumber"`
}

// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *jsonrpcMessage) String() string {
	b, _ := json.Marshal(msg)
	return string(b)
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	return err.Code
}

type jsonrpcFiscoMsg struct {
	ID      json.RawMessage `json:"id,omitempty"`
	Jsonrpc string          `json:"jsonrpc"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  struct {
		CurrentBlockNumber string          `json:"currentBlockNumber,omitempty"`
		Output             json.RawMessage `json:"output,omitempty"`
		Status             string          `json:"status,omitempty"`
		ExtraData          []interface{}   `json:"extraData"`
		GasLimit           string          `json:"gasLimit"`
		GasUsed            string          `json:"gasUsed"`
		Hash               string          `json:"hash"`
		LogsBloom          string          `json:"logsBloom"`
		Number             string          `json:"number"`
		ParentHash         string          `json:"parentHash"`
		Sealer             string          `json:"sealer"`
		SealerList         []string        `json:"sealerList"`
		StateRoot          string          `json:"stateRoot"`
		Timestamp          string          `json:"timestamp"`
		Transactions       []struct {
			BlockHash        string      `json:"blockHash"`
			BlockNumber      string      `json:"blockNumber"`
			From             string      `json:"from"`
			Gas              string      `json:"gas"`
			GasPrice         string      `json:"gasPrice"`
			Hash             string      `json:"hash"`
			Input            string      `json:"input"`
			Nonce            string      `json:"nonce"`
			To               interface{} `json:"to"`
			TransactionIndex string      `json:"transactionIndex"`
			Value            string      `json:"value"`
		} `json:"transactions"`
		TransactionsRoot string `json:"transactionsRoot"`
	} `json:"result"`
}

type rpcResponse struct {
	C      chan *jsonrpcMessage
	Method string
}
