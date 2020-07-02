package client

import (
	"github.com/chislab/go-fiscobcos/core/types"
)

type Backend interface {
	Close()

	SubEventLogs(arg RegisterEventLogRequest) (chan *types.Log, error)

	// BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	// ClientVersion(ctx context.Context) (*types.ClientVersion, error)
	// BlockNumber(ctx context.Context) (*big.Int, error)
	// SyncStatus(ctx context.Context) (*types.SyncStatus, error)
	// BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	// TotalTransactionCount(ctx context.Context) (*types.TotalTransactionCount, error)
	// TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	// TransactionByBlockNumberAndIndex(ctx context.Context, blockNumber string, transactionIndex string) (*types.TransactionByHash, error)
	// TransactionByBlockHashAndIndex(ctx context.Context, blockHash string, transactionIndex string) (*types.TransactionByHash, error)
	// TransactionByHash(ctx context.Context, transactionHash string) (*types.TransactionByHash, error)
	// PbftView(ctx context.Context) (string, error)
	// BlockHashByNumber(ctx context.Context, blockNumber uint64) (*common.Hash, error)
	// PendingTxSize(ctx context.Context) (string, error)
	// Code(ctx context.Context, contraddress string) (string, error)
	// SystemConfigByKey(ctx context.Context, key string) (string, error)
	// SealerList(ctx context.Context) ([]string, error)
	// ObserverList(ctx context.Context) ([]string, error)
	// ConsensusStatus(ctx context.Context) ([]interface{}, error)
	// Peers(ctx context.Context) ([]types.PeerStatus, error)
	// GroupPeers(ctx context.Context) ([]string, error)
	// NodeIDList(ctx context.Context) ([]string, error)
	// GroupList(ctx context.Context) ([]int64, error)
	// PendingTransactions(ctx context.Context) ([]types.PendingTx, error)
}
