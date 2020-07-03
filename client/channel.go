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
	rpcResponse     map[string]chan *jsonrpcMessage
	groupID         uint64
	isClosed        int32
}

func newChannel(caFile, certFile, keyFile, endpoint string, groupID uint64) (*channelClient, error) {
	conn, err := tls.Dial(caFile, certFile, keyFile, endpoint)
	if err != nil {
		return nil, err
	}
	cli := channelClient{
		conn:        conn,
		buffer:      make([]byte, 256*1024),
		exit:        make(chan struct{}),
		rpcResponse: make(map[string]chan *jsonrpcMessage),
		groupID:     groupID,
	}
	go cli.readResponse()
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
	c.rpcResponse[string(msg.ID)] = ch
	defer delete(c.rpcResponse, string(msg.ID))
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
	readCh := make(chan *tls.ReadResult, 8)
	for {
		go c.conn.Read(c.buffer, readCh)
		select {
		case <-c.exit:
			return
		case readResult := <-readCh:
			cnt, err := readResult.Count, readResult.Error
			if err != nil {
				ticker := time.NewTicker(1 * time.Second)
				for err != nil {
					select {
					case <-c.exit:
						return
					case <-ticker.C:
						fmt.Printf("block chain connection error: %v, reconnecting...", err)
						err = c.conn.Reconnecte()
					}
				}
				continue
			}
			msg, err := DecodeMessage(c.buffer[:cnt])
			if err != nil {
				fmt.Printf("decode message error %v\n", err)
				continue
			}
			switch msg.Type {
			case TypeRPCRequest:
				var respmsg jsonrpcMessage
				if err = json.Unmarshal(msg.Data, &respmsg); err != nil {
					fmt.Printf("decode rpc response failed %v\n", err)
					continue
				}
				select {
				case c.rpcResponse[string(respmsg.ID)] <- &respmsg:
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

func (c *channelClient) CheckTx(ctx context.Context, tx *types.Transaction) error {
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
