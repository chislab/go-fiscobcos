package client

import (
	"context"
	"math/big"

	"github.com/chislab/go-fiscobcos"
	"github.com/chislab/go-fiscobcos/accounts/abi/bind"
	"github.com/chislab/go-fiscobcos/common"
	"github.com/chislab/go-fiscobcos/core/types"
)

type Backend interface {
	Close()
	CheckReceipt(ctx context.Context, tx *types.Transaction) (*types.Receipt, error)
	CheckTx(ctx context.Context, tx *types.Transaction) error

	ClientVersion(ctx context.Context) (*types.ClientVersion, error)
	BlockNumber(ctx context.Context) (*big.Int, error)
	PbftView(ctx context.Context) (string, error)
	SealerList(ctx context.Context) ([]string, error)
	ObserverList(ctx context.Context) ([]string, error)
	ConsensusStatus(ctx context.Context) ([]interface{}, error)
	SyncStatus(ctx context.Context) (*types.SyncStatus, error)
	Peers(ctx context.Context) ([]types.PeerStatus, error)
	GroupPeers(ctx context.Context) ([]string, error)
	NodeIDList(ctx context.Context) ([]string, error)
	GroupList(ctx context.Context) ([]int64, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	BlockHashByNumber(ctx context.Context, blockNumber uint64) (*common.Hash, error)
	TransactionByHash(ctx context.Context, transactionHash string) (*types.TransactionByHash, error)
	TransactionByBlockHashAndIndex(ctx context.Context, blockHash string, transactionIndex string) (*types.TransactionByHash, error)
	TransactionByBlockNumberAndIndex(ctx context.Context, blockNumber string, transactionIndex string) (*types.TransactionByHash, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	PendingTransactions(ctx context.Context) ([]types.PendingTx, error)
	PendingTxSize(ctx context.Context) (string, error)
	Code(ctx context.Context, contract string) (string, error)
	CodeAt(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error)
	TotalTransactionCount(ctx context.Context) (*types.TotalTransactionCount, error)
	SystemConfigByKey(ctx context.Context, key string) (string, error)
	CallContract(ctx context.Context, msg fiscobcos.CallMsg, blockNumber *big.Int) ([]byte, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	// SendTransactionAndGetProof()
	// TransactionByHashWithProof()
	// TransactionReceiptByHashWithProof()
	// GenerateGroup()
	// StartGroup()
	// StopGroup()
	// RemoveGroup()
	// RecoverGroup()
	// QueryGroupStatus()
	FilterLogs(ctx context.Context, query fiscobcos.FilterQuery) ([]types.Log, error)
	SubscribeFilterLogs(ctx context.Context, query fiscobcos.FilterQuery, ch chan<- types.Log) (fiscobcos.Subscription, error)
	UpdateBlockLimit(ctx context.Context, opt *bind.TransactOpts) error
}
