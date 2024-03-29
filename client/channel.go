package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chislab/go-fiscobcos"
	"github.com/chislab/go-fiscobcos/accounts/abi/bind"
	"github.com/chislab/go-fiscobcos/client/tls"
	"github.com/chislab/go-fiscobcos/common"
	"github.com/chislab/go-fiscobcos/common/hexutil"
	"github.com/chislab/go-fiscobcos/core/types"
	"github.com/chislab/go-fiscobcos/crypto"
	"github.com/chislab/go-fiscobcos/rlp"
	"github.com/tidwall/gjson"
)

var _ = crypto.S256()

type channelClient struct {
	conn            *tls.Connection
	buffer          []byte
	exit            chan struct{}
	pending         sync.Map
	watcher         sync.Map
	onBlock         []OnBlockFunc
	notifyBlockOnce sync.Once
	idCounter       uint64
	rpcResponse     sync.Map
	groupID         uint64
	isClosed        int32
}

func newChannel(caFile, certFile, keyFile, endpoint string, groupID uint64) (*channelClient, error) {
	conn, err := tls.Dial(caFile, certFile, keyFile, endpoint)
	if err != nil {
		return nil, err
	}
	cli := channelClient{
		conn:    conn,
		buffer:  make([]byte, 32*1024),
		exit:    make(chan struct{}),
		groupID: groupID,
	}
	go cli.readResponse()
	go cli.heartbeat()
	//msg, err := cli.ReadBlockHeight()
	//fmt.Printf("msg: %s\nerror:%v\n", msg, err)
	return &cli, nil
}

func (c *channelClient) Close() {
	if !atomic.CompareAndSwapInt32(&c.isClosed, 0, 1) {
		return
	}
	c.conn.Close()
	c.exit <- struct{}{}
}

func (c *channelClient) heartbeat() {
	heartbeatTicker := time.NewTicker(25 * time.Second)
	defer heartbeatTicker.Stop()
	for {
		<-heartbeatTicker.C
		c.Send(TypeHeartBeat, "", map[string]string{
			"heartbeat": "0",
		})
	}
}

func (c *channelClient) Send(typ int, topic string, data interface{}) (string, error) {
	if atomic.LoadInt32(&c.isClosed) != 0 {
		return "", fmt.Errorf("use of closed network connection")
	}
	msg, err := NewMessage(typ, topic, data)
	if err != nil {
		return "", err
	}
	msgBytes := msg.Encode()
	cnt, err := c.conn.Write(msgBytes)
	if err != nil {
		return "", err
	}
	if cnt != len(msgBytes) {
		return "", errors.New("data is not completely written")
	}
	return msg.Seq, nil
}

func (c *channelClient) sendRPC(msg *jsonrpcMessage) (*jsonrpcMessage, error) {
	ch := make(chan *jsonrpcMessage, 1)
	c.rpcResponse.Store(string(msg.ID), rpcResponse{
		C:      ch,
		Method: msg.Method,
	})
	defer c.rpcResponse.Delete(string(msg.ID))
	_, err := c.Send(TypeRPCRequest, "", msg)
	if err != nil {
		return nil, err
	}
	var resp *jsonrpcMessage
	timeout := time.After(20 * time.Second)
	select {
	case resp = <-ch:
	case <-timeout:
		err = fmt.Errorf("request timeout")
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *channelClient) rpcCall(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	if result != nil && reflect.TypeOf(result).Kind() != reflect.Ptr {
		return fmt.Errorf("call result parameter must be pointer or nil interface: %v", result)
	}
	msg, err := c.newRpcMessage(method, args...)
	if err != nil {
		return err
	}
	resp, err := c.sendRPC(msg)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	if len(resp.Result) == 0 {
		return fiscobcos.NotFound
	}
	return json.Unmarshal(resp.Result, &result)
}

func (c *channelClient) nextID() json.RawMessage {
	id := atomic.AddUint64(&c.idCounter, 1)
	return strconv.AppendUint(nil, id, 10)
}

func (c *channelClient) newRpcMessage(method string, paramsIn ...interface{}) (*jsonrpcMessage, error) {
	msg := &jsonrpcMessage{Version: "2.0", ID: c.nextID(), Method: method}
	if paramsIn != nil {
		var err error
		if msg.Params, err = json.Marshal(paramsIn); err != nil {
			return nil, err
		}
	}
	return msg, nil
}

func (c *channelClient) readResponse() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	readCh := make(chan *tls.ReadResult, 8)
	for {
		go c.conn.ReadWithChannel(c.buffer, readCh)
		select {
		case <-c.exit:
			return
		case readResult := <-readCh:
			cnt, err := readResult.Count, readResult.Error
			if err != nil {
				for err != nil {
					select {
					case <-c.exit:
						return
					case <-ticker.C:
						fmt.Printf("block chain connection error: %v, reconnecting...\n", err)
						err = c.conn.Reconnecte()
					}
				}
				continue
			}
			var msg Message
			totalLen := getMessageLength(c.buffer[:cnt])
			if totalLen > cnt {
				all := make([]byte, 0, cnt*5)
				all = append(all, c.buffer[:cnt]...)
				for len(all) < totalLen {
					cnt, err := c.conn.Read(c.buffer)
					if err != nil {
						break
					}
					all = append(all, c.buffer[:cnt]...)
				}
				msg, err = DecodeMessage(all)
			} else {
				msg, err = DecodeMessage(c.buffer[:cnt])
			}
			if err != nil {
				fmt.Printf("decode message error %v\n", err)
				continue
			}
			switch msg.Type {
			case TypeRPCRequest:
				id := gjson.GetBytes(msg.Data, "id").Raw
				respItf, ok := c.rpcResponse.Load(id)
				if !ok {
					continue
				}
				respBody, ok := respItf.(rpcResponse)
				if !ok {
					continue
				}
				var respmsg jsonrpcMessage
				if respBody.Method == "call" {
					var fiscomsg jsonrpcFiscoMsg
					if err = json.Unmarshal(msg.Data, &fiscomsg); err != nil {
						continue
					}
					respmsg.Version = fiscomsg.Jsonrpc
					respmsg.ID = fiscomsg.ID
					respmsg.Result = fiscomsg.Result.Output
				} else {
					if err = json.Unmarshal(msg.Data, &respmsg); err != nil {
						fmt.Printf("decode rpc response failed %v\n", err)
						continue
					}
				}
				select {
				case respBody.C <- &respmsg:
				default:
				}
			case TypeRegisterEventLog:
				res := gjson.GetBytes(msg.Data, "result").Int()
				if ch, ok := c.pending.Load(msg.Seq); ok {
					ch := ch.(chan error)
					if res == 0 {
						ch <- nil
					} else {
						ch <- fmt.Errorf("register failed, code %v", res)
					}
				}
			case TypeEventLog:
				var eventLog EventLogResponse
				if err := json.Unmarshal(msg.Data, &eventLog); err != nil {
					fmt.Printf("event log unmarshal fail %v\n", err)
					continue
				}
				if ch, ok := c.watcher.Load(eventLog.FilterID); ok {
					ch := ch.(chan<- types.Log)
					for _, log := range eventLog.Logs {
						blkNumber, _ := hexutil.DecodeUint64(log.BlockNumber)
						data, _ := hexutil.Decode(log.Data)
						txIndex, _ := hexutil.DecodeUint64(log.TxIndex)
						logIndex, _ := hexutil.DecodeUint64(log.Index)
						ch <- types.Log{
							Address:     log.Address,
							Topics:      log.Topics,
							Data:        data,
							BlockNumber: blkNumber,
							TxHash:      log.TxHash,
							TxIndex:     uint(txIndex),
							BlockHash:   log.BlockHash,
							Index:       uint(logIndex),
							Removed:     log.Removed,
						}
					}
				}
			case TypeBlockNotify:
				if len(c.onBlock) == 0 {
					continue
				}
				var blockNotify BlockNotifyResponse
				if err := json.Unmarshal(msg.Data, &blockNotify); err != nil {
					fmt.Printf("block notify unmarshal fail %v\n", err)
					continue
				}
				groupID, err := strconv.ParseUint(blockNotify.GroupID, 10, 64)
				if err != nil {
					fmt.Printf("block notify parse groupID failed: %s: %v", blockNotify.GroupID, err)
					continue
				}
				blockNumber, err := strconv.ParseUint(blockNotify.BlockNumber, 10, 64)
				if err != nil {
					fmt.Printf("block notify parse blockNumber failed: %s: %v", blockNotify.BlockNumber, err)
					continue
				}
				for _, onBlock := range c.onBlock {
					if onBlock != nil {
						onBlock(groupID, blockNumber)
					}
				}
			default:
				//fmt.Printf("other msg: %s(0x%x)\n", msg.Data, msg.Type)
			}
		}
	}
}

func (c *channelClient) CheckReceipt(ctx context.Context, tx *types.Transaction) (receipt *types.Receipt, err error) {
	if tx == nil {
		return nil, fmt.Errorf("tx is nil")
	}
	queryTicker := time.NewTicker(time.Second)
	defer queryTicker.Stop()
	for {
		receipt, err = c.TransactionReceipt(ctx, tx.Hash())
		if err == nil && receipt != nil {
			break
		}
		// Wait for the next round.
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-queryTicker.C:
		}
	}
	if receipt.Status != "0x0" {
		err = fmt.Errorf("receipt status:%s, tx hash:%s, output:%s", receipt.Status, receipt.TxHash.String(), getReceiptOutput(receipt.Output))
		return
	}
	return
}

func (c *channelClient) CheckTx(ctx context.Context, tx *types.Transaction) error {
	_, err := c.CheckReceipt(ctx, tx)
	return err
}

func (c *channelClient) ClientVersion(ctx context.Context) (*types.ClientVersion, error) {
	var result *types.ClientVersion
	err := c.rpcCall(ctx, &result, "getClientVersion")
	return result, err
}

func (c *channelClient) BlockNumber(ctx context.Context) (*big.Int, error) {
	var result string
	err := c.rpcCall(ctx, &result, "getBlockNumber", c.groupID)
	if err != nil {
		return nil, err
	}
	height, err := hexutil.DecodeUint64(result)
	return big.NewInt(int64(height)), err
}

func (c *channelClient) PbftView(ctx context.Context) (string, error) {
	var result string
	err := c.rpcCall(ctx, &result, "getPbftView", c.groupID)
	return result, err
}

func (c *channelClient) SealerList(ctx context.Context) ([]string, error) {
	var result []string
	err := c.rpcCall(ctx, &result, "getSealerList", c.groupID)
	return result, err
}

func (c *channelClient) ObserverList(ctx context.Context) ([]string, error) {
	var result []string
	err := c.rpcCall(ctx, &result, "getObserverList", c.groupID)
	return result, err
}

func (c *channelClient) ConsensusStatus(ctx context.Context) ([]interface{}, error) {
	var result []interface{}
	err := c.rpcCall(ctx, &result, "getConsensusStatus", c.groupID)
	return result, err
}

func (c *channelClient) SyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	var result *types.SyncStatus
	err := c.rpcCall(ctx, &result, "getSyncStatus", c.groupID)
	return result, err
}

func (c *channelClient) Peers(ctx context.Context) ([]types.PeerStatus, error) {
	var result []types.PeerStatus
	err := c.rpcCall(ctx, &result, "getPeers", c.groupID)
	return result, err
}

func (c *channelClient) GroupPeers(ctx context.Context) ([]string, error) {
	var result []string
	err := c.rpcCall(ctx, &result, "getGroupPeers", c.groupID)
	return result, err
}

func (c *channelClient) NodeIDList(ctx context.Context) ([]string, error) {
	var result []string
	err := c.rpcCall(ctx, &result, "getNodeIDList", c.groupID)
	return result, err
}

func (c *channelClient) GroupList(ctx context.Context) ([]int64, error) {
	var result []int64
	err := c.rpcCall(ctx, &result, "getGroupList", c.groupID)
	return result, err
}

func (c *channelClient) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	var result *types.Block
	err := c.rpcCall(ctx, &result, "getBlockByHash", c.groupID, hash, true)
	return result, err
}

func (c *channelClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	var result *types.Block
	err := c.rpcCall(ctx, &result, "getBlockByNumber", c.groupID, toBlockNumArg(number), true)
	return result, err
}
func (c *channelClient) BlockHashByNumber(ctx context.Context, blockNumber uint64) (*common.Hash, error) {
	var result *common.Hash
	err := c.rpcCall(ctx, &result, "getBlockHashByNumber", c.groupID, string(blockNumber))
	return result, err
}

func (c *channelClient) TransactionByHash(ctx context.Context, transactionHash string) (*types.TransactionByHash, error) {
	var result *types.TransactionByHash
	err := c.rpcCall(ctx, &result, "getTransactionByHash", c.groupID, transactionHash)
	return result, err
}

func (c *channelClient) TransactionByBlockHashAndIndex(ctx context.Context, blockHash string, transactionIndex string) (*types.TransactionByHash, error) {
	var result *types.TransactionByHash
	err := c.rpcCall(ctx, &result, "getTransactionByBlockHashAndIndex", c.groupID, blockHash, transactionIndex)
	return result, err
}

func (c *channelClient) TransactionByBlockNumberAndIndex(ctx context.Context, blockNumber string, transactionIndex string) (*types.TransactionByHash, error) {
	var result *types.TransactionByHash
	err := c.rpcCall(ctx, &result, "getTransactionByBlockNumberAndIndex", c.groupID, blockNumber, transactionIndex)
	return result, err
}

func (c *channelClient) PendingTransactions(ctx context.Context) ([]types.PendingTx, error) {
	var result []types.PendingTx
	err := c.rpcCall(ctx, &result, "getPendingTransactions", c.groupID)
	return result, err
}

func (c *channelClient) PendingTxSize(ctx context.Context) (string, error) {
	var result string
	err := c.rpcCall(ctx, &result, "getPendingTxSize", c.groupID)
	return result, err
}

func (c *channelClient) Code(ctx context.Context, contract string) (string, error) {
	var result string
	err := c.rpcCall(ctx, &result, "getCode", c.groupID, contract)
	return result, err
}

func (c *channelClient) TotalTransactionCount(ctx context.Context) (*types.TotalTransactionCount, error) {
	var result *types.TotalTransactionCount
	err := c.rpcCall(ctx, &result, "getTotalTransactionCount", c.groupID)
	return result, err
}

func (c *channelClient) SystemConfigByKey(ctx context.Context, key string) (string, error) {
	var result string
	err := c.rpcCall(ctx, &result, "getSystemConfigByKey", c.groupID, key)
	return result, err
}

func (c *channelClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	var result *types.Receipt
	err := c.rpcCall(ctx, &result, "getTransactionReceipt", c.groupID, txHash)
	return result, err
}

func (c *channelClient) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	var result hexutil.Bytes
	err := c.rpcCall(ctx, &result, "getCode", c.groupID, account, toBlockNumArg(blockNumber))
	return result, err
}

func (c *channelClient) CallContract(ctx context.Context, msg fiscobcos.CallMsg, blockNumber *big.Int) ([]byte, error) {
	var hex hexutil.Bytes
	err := c.rpcCall(ctx, &hex, "call", msg.GroupId, toCallArg(msg.Msg))
	return hex, err
}

func (c *channelClient) FilterLogs(ctx context.Context, query fiscobcos.FilterQuery) ([]types.Log, error) {
	return nil, fmt.Errorf("not supported")
}

func (c *channelClient) SubscribeFilterLogs(ctx context.Context, query fiscobcos.FilterQuery, ch chan<- types.Log) (fiscobcos.Subscription, error) {
	fromBlock := "latest"
	if query.FromBlock != nil {
		fromBlock = query.FromBlock.Text(10)
	}
	addresses := make([]string, len(query.Addresses))
	for i, addr := range query.Addresses {
		addresses[i] = addr.String()
	}
	topics := make([]string, 0)
	for _, topicArray := range query.Topics {
		for _, topic := range topicArray {
			topics = append(topics, topic.String())
		}
	}
	arg := RegisterEventLogRequest{
		FromBlock: fromBlock,
		ToBlock:   "latest",
		Addresses: addresses,
		Topics:    topics,
		GroupID:   strconv.FormatUint(c.groupID, 10),
		FilterID:  NewFilterID(),
	}
	err := c.SubEventLogs(arg, ch)
	if err != nil {
		return nil, err
	}

	return new(ChannelSubscription), nil
}

func (c *channelClient) UpdateBlockLimit(ctx context.Context, opt *bind.TransactOpts) error {
	blkNumber, err := c.BlockNumber(ctx)
	if err != nil {
		return err
	}
	opt.BlockLimit = blkNumber.Uint64() + 1000
	return nil
}

func (c *channelClient) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return err
	}
	return c.rpcCall(ctx, nil, "sendRawTransaction", c.groupID, common.ToHex(data))
}

func (c *channelClient) SubEventLogs(arg RegisterEventLogRequest, ch chan<- types.Log) error {
	if len(arg.FilterID) != 32 {
		return errors.New("filterID invalid")
	}
	if _, ok := c.watcher.Load(arg.FilterID); ok {
		c.watcher.Store(arg.FilterID, ch)
		return nil
	}
	msgSeq, err := c.Send(TypeRegisterEventLog, "", arg)
	if err != nil {
		return err
	}
	pch := make(chan error)
	c.pending.Store(msgSeq, pch)
	err = <-pch
	c.pending.Delete(msgSeq)
	if err != nil {
		return err
	}
	c.watcher.Store(arg.FilterID, ch)
	return nil
}

func (c *channelClient) BlockNotify(onBlock OnBlockFunc) error {
	var err error
	c.notifyBlockOnce.Do(func() {
		_, err = c.Send(TypeBlockNotify, "", nil)
	})
	if err != nil {
		return err
	}
	c.onBlock = append(c.onBlock, onBlock)
	return nil
}
