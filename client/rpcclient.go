// Copyright 2016 The go-bcos Authors
// This file is part of the go-bcos library.
//
// The go-bcos library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-bcos library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-bcos library. If not, see <http://www.gnu.org/licenses/>.

// Package client provides a client for the Bcos RPC API.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/chislab/go-fiscobcos"
	"github.com/chislab/go-fiscobcos/accounts/abi/bind"
	"github.com/chislab/go-fiscobcos/common"
	"github.com/chislab/go-fiscobcos/common/hexutil"
	"github.com/chislab/go-fiscobcos/core/types"
	"github.com/chislab/go-fiscobcos/rlp"
	"github.com/chislab/go-fiscobcos/rpc"
)

// Client defines typed wrappers for the Bcos RPC API.
type rpcClient struct {
	c       *rpc.Client
	GroupId uint64
}

// dialRPC connects a client to the given URL.
func dialRPC(rawurl string, groupID uint64) (*rpcClient, error) {
	return dialRPCContext(context.Background(), rawurl, groupID)
}

func dialRPCContext(ctx context.Context, rawurl string, groupID uint64) (*rpcClient, error) {
	c, err := rpc.DialContext(ctx, rawurl)
	if err != nil {
		return nil, err
	}
	return newRPCClient(c, groupID), nil
}

// newRPCClient creates a client that uses the given RPC client.
func newRPCClient(c *rpc.Client, groupID uint64) *rpcClient {
	return &rpcClient{c: c, GroupId: groupID}
}

func (ec *rpcClient) Close() {
	ec.c.Close()
}

func (c *rpcClient) CheckTx(ctx context.Context, tx *types.Transaction) error {
	queryTicker := time.NewTicker(time.Second)
	defer queryTicker.Stop()
	receipt := new(types.Receipt)
	var err error
	for {
		receipt, err = c.TransactionReceipt(ctx, tx.Hash())
		if err == nil && receipt != nil {
			break
		}
		// Wait for the next round.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-queryTicker.C:
		}
	}
	if receipt.Status != "0x0" {
		return fmt.Errorf("receipt status:%s, tx hash:%s, output:%s", receipt.Status, receipt.TxHash.String(), getReceiptOutput(receipt.Output))
	}
	return nil
}

func (ec *rpcClient) UpdateBlockLimit(ctx context.Context, opt *bind.TransactOpts) error {
	blkNumber, err := ec.BlockNumber(ctx)
	if err != nil {
		return err
	}
	opt.BlockLimit = blkNumber.Uint64() + 1000
	return nil
}

func (ec *rpcClient) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return ec.getBlock(ctx, "getBlockByHash", ec.GroupId, hash, true)
}

func (ec *rpcClient) ClientVersion(ctx context.Context) (*types.ClientVersion, error) {
	return ec.getClientVersion(ctx, "getClientVersion")
}

func (ec *rpcClient) BlockNumber(ctx context.Context) (*big.Int, error) {
	return ec.getBlockNumber(ctx, "getBlockNumber", ec.GroupId)
}
func (ec *rpcClient) SyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	return ec.getSyncStatus(ctx, "getSyncStatus", ec.GroupId)
}
func (ec *rpcClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return ec.getBlockByNumber(ctx, "getBlockByNumber", ec.GroupId, toBlockNumArg(number), true)
}
func (ec *rpcClient) TotalTransactionCount(ctx context.Context) (*types.TotalTransactionCount, error) {
	return ec.getTotalTransactionCount(ctx, "getTotalTransactionCount", ec.GroupId)
}
func (ec *rpcClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return ec.getTransactionReceipt(ctx, "getTransactionReceipt", ec.GroupId, txHash)
}
func (ec *rpcClient) TransactionByBlockNumberAndIndex(ctx context.Context, blockNumber string, transactionIndex string) (*types.TransactionByHash, error) {
	return ec.getTransactionByBlockNumberAndIndex(ctx, "getTransactionByBlockNumberAndIndex", ec.GroupId, blockNumber, transactionIndex)
}
func (ec *rpcClient) TransactionByBlockHashAndIndex(ctx context.Context, blockHash string, transactionIndex string) (*types.TransactionByHash, error) {
	return ec.getTransactionByBlockHashAndIndex(ctx, "getTransactionByBlockHashAndIndex", ec.GroupId, blockHash, transactionIndex)
}
func (ec *rpcClient) TransactionByHash(ctx context.Context, transactionHash string) (*types.TransactionByHash, error) {
	return ec.getTransactionByHash(ctx, "getTransactionByBlockHashAndIndex", ec.GroupId, transactionHash)
}
func (ec *rpcClient) PbftView(ctx context.Context) (string, error) {
	return ec.getPbftView(ctx, "getPbftView", ec.GroupId)
}
func (ec *rpcClient) BlockHashByNumber(ctx context.Context, blockNumber uint64) (*common.Hash, error) {
	return ec.getBlockHashByNumber(ctx, "getBlockHashByNumber", ec.GroupId, string(blockNumber))
}
func (ec *rpcClient) PendingTxSize(ctx context.Context) (string, error) {
	return ec.getPendingTxSize(ctx, "getPendingTxSize", ec.GroupId)
}

func (ec *rpcClient) Code(ctx context.Context, contraddress string) (string, error) {
	return ec.getCode(ctx, "getCode", ec.GroupId, contraddress)
}
func (ec *rpcClient) SystemConfigByKey(ctx context.Context, key string) (string, error) {
	return ec.getSystemConfigByKey(ctx, "getSystemConfigByKey", ec.GroupId, key)
}
func (ec *rpcClient) SealerList(ctx context.Context) ([]string, error) {
	return ec.getSealerList(ctx, "getSealerList", ec.GroupId)
}
func (ec *rpcClient) ObserverList(ctx context.Context) ([]string, error) {
	return ec.getObserverList(ctx, "getObserverList", ec.GroupId)
}
func (ec *rpcClient) ConsensusStatus(ctx context.Context) ([]interface{}, error) {
	return ec.getConsensusStatus(ctx, "getConsensusStatus", ec.GroupId)
}
func (ec *rpcClient) Peers(ctx context.Context) ([]types.PeerStatus, error) {
	return ec.getPeers(ctx, "getPeers", ec.GroupId)
}
func (ec *rpcClient) GroupPeers(ctx context.Context) ([]string, error) {
	return ec.getGroupPeers(ctx, "getGroupPeers", ec.GroupId)
}
func (ec *rpcClient) NodeIDList(ctx context.Context) ([]string, error) {
	return ec.getNodeIDList(ctx, "getNodeIDList", ec.GroupId)
}
func (ec *rpcClient) GroupList(ctx context.Context) ([]int64, error) {
	return ec.getGroupList(ctx, "getGroupList")
}

func (ec *rpcClient) PendingTransactions(ctx context.Context) ([]types.PendingTx, error) {
	return ec.getPendingTransactions(ctx, "getPendingTransactions", ec.GroupId)
}

func (ec *rpcClient) SubEventLogs(arg RegisterEventLogRequest) (chan *types.Log, error) {
	return nil, fmt.Errorf("not supported with rpc")
}

func (ec *rpcClient) getClientVersion(ctx context.Context, method string, args ...interface{}) (*types.ClientVersion, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.ClientVersion
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getBlock(ctx context.Context, method string, args ...interface{}) (*types.Block, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.Block
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getBlockNumber(ctx context.Context, method string, args ...interface{}) (*big.Int, error) {
	var raw string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	height, err := hexutil.DecodeUint64(raw)
	return big.NewInt(int64(height)), err
}
func (ec *rpcClient) getSyncStatus(ctx context.Context, method string, args ...interface{}) (*types.SyncStatus, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.SyncStatus
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getBlockByNumber(ctx context.Context, method string, args ...interface{}) (*types.Block, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.Block
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getTotalTransactionCount(ctx context.Context, method string, args ...interface{}) (*types.TotalTransactionCount, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.TotalTransactionCount
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getTransactionReceipt(ctx context.Context, method string, args ...interface{}) (*types.Receipt, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.Receipt
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getTransactionByBlockNumberAndIndex(ctx context.Context, method string, args ...interface{}) (*types.TransactionByHash, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.TransactionByHash
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getTransactionByBlockHashAndIndex(ctx context.Context, method string, args ...interface{}) (*types.TransactionByHash, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.TransactionByHash
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getTransactionByHash(ctx context.Context, method string, args ...interface{}) (*types.TransactionByHash, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result *types.TransactionByHash
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getPbftView(ctx context.Context, method string, args ...interface{}) (string, error) {
	var raw string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return "", err
	} else if len(raw) == 0 {
		return "", fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getBlockHashByNumber(ctx context.Context, method string, args ...interface{}) (*common.Hash, error) {
	var raw string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	blockHash := common.HexToHash(raw)
	return &blockHash, nil
}
func (ec *rpcClient) getPendingTxSize(ctx context.Context, method string, args ...interface{}) (string, error) {
	var raw string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return "", err
	} else if len(raw) == 0 {
		return "", fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getCode(ctx context.Context, method string, args ...interface{}) (string, error) {
	var raw string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return "", err
	} else if len(raw) == 0 {
		return "", fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getSystemConfigByKey(ctx context.Context, method string, args ...interface{}) (string, error) {
	var raw string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return "", err
	} else if len(raw) == 0 {
		return "", fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getSealerList(ctx context.Context, method string, args ...interface{}) ([]string, error) {
	var raw []string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getObserverList(ctx context.Context, method string, args ...interface{}) ([]string, error) {
	var raw []string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getConsensusStatus(ctx context.Context, method string, args ...interface{}) ([]interface{}, error) {
	var raw []interface{}
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getPeers(ctx context.Context, method string, args ...interface{}) ([]types.PeerStatus, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result []types.PeerStatus
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}
func (ec *rpcClient) getGroupPeers(ctx context.Context, method string, args ...interface{}) ([]string, error) {
	var raw []string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getNodeIDList(ctx context.Context, method string, args ...interface{}) ([]string, error) {
	var raw []string
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getGroupList(ctx context.Context, method string, args ...interface{}) ([]int64, error) {
	var raw []int64
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	return raw, err
}
func (ec *rpcClient) getPendingTransactions(ctx context.Context, method string, args ...interface{}) ([]types.PendingTx, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, method, args...)
	if err != nil {
		return nil, err
	} else if len(raw) == 0 {
		return nil, fiscobcos.NotFound
	}
	// Decode header and transactions.
	var result []types.PendingTx
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, err
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	return hexutil.EncodeBig(number)
}

// CodeAt returns the contract code of the given account.
// The block number can be nil, in which case the code is taken from the latest known block.
func (ec *rpcClient) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	var result hexutil.Bytes
	err := ec.c.CallContext(ctx, &result, "getCode", ec.GroupId, account, toBlockNumArg(blockNumber))
	return result, err
}

// Contract Calling

// CallContract executes a message call transaction, which is directly executed in the VM
// of the node, but never mined into the blockchain.
//
// blockNumber selects the block height at which the call runs. It can be nil, in which
// case the code is taken from the latest known block. Note that state from very old
// blocks might not be available.
func (ec *rpcClient) CallContract(ctx context.Context, msg fiscobcos.CallMsg, blockNumber *big.Int) ([]byte, error) {
	var hex hexutil.Bytes
	err := ec.c.CallContext(ctx, &hex, "call", msg.GroupId, toCallArg(msg.Msg))
	if err != nil {
		return nil, err
	}
	return hex, nil
}

// SendTransaction injects a signed transaction into the pending pool for execution.
//
// If the transaction was a contract creation use the TransactionReceipt method to get the
// contract address after the transaction has been mined.
func (ec *rpcClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}
	return ec.c.CallContext(ctx, nil, "sendRawTransaction", ec.GroupId, common.ToHex(data))
}

func toCallArg(msg fiscobcos.CallEthMsg) interface{} {
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["data"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}

	return arg
}

func (ec *rpcClient) FilterLogs(ctx context.Context, q fiscobcos.FilterQuery) ([]types.Log, error) {
	if q.ToBlock == nil {
		q.ToBlock, _ = ec.BlockNumber(ctx)
		block, _ := ec.BlockByNumber(ctx, q.ToBlock)
		for _, txHash := range block.Transactions {
			receipt, _ := ec.TransactionReceipt(ctx, common.HexToHash(txHash.Hash))
			for len(receipt.Logs) > 0 {
				var txLogs []types.Log
				for _, txLog := range receipt.Logs {
					txLogs = append(txLogs, *txLog)
				}
				return txLogs, nil
			}
		}
		q.ToBlock, _ = ec.BlockNumber(ctx)
		block, _ = ec.BlockByNumber(ctx, q.ToBlock)
	} else if q.ToBlock != nil && q.FromBlock != nil {
	}
	return nil, errors.New("FiscoBcos doesn't provide this function.")
}

// SubscribeFilterLogs subscribes to the results of a streaming filter query.
func (ec *rpcClient) SubscribeFilterLogs(ctx context.Context, q fiscobcos.FilterQuery, ch chan<- types.Log) (fiscobcos.Subscription, error) {
	return nil, errors.New("FiscoBcos doesn't provide this function.")
}
