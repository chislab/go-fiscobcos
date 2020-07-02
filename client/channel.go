package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"sync"

	"github.com/FISCO-BCOS/crypto/tls"
	"github.com/FISCO-BCOS/crypto/x509"
	"github.com/chislab/go-fiscobcos/common/hexutil"
	"github.com/chislab/go-fiscobcos/core/types"
	"github.com/tidwall/gjson"
)

type channelClient struct {
	conn            *tls.Conn
	buffer          []byte
	exit            chan struct{}
	pending         sync.Map
	watcher         sync.Map
	onBlock         []OnBlockFunc
	notifyBlockOnce sync.Once
}

func newChannel(conf *Config) (*channelClient, error) {
	caBytes, err := ioutil.ReadFile(conf.CAFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(conf.CertFile, conf.KeyFile)
	if err != nil {
		return nil, err
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caBytes); !ok {
		return nil, errors.New("import ca fail")
	}

	tlsConf := &tls.Config{
		RootCAs:                  caPool,
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		InsecureSkipVerify:       true,
	}
	tlsConf.CurvePreferences = append(tlsConf.CurvePreferences, tls.CurveSecp256k1)
	conn, err := tls.Dial("tcp", conf.Endpoint, tlsConf)
	if err != nil {
		return nil, err
	}
	cli := channelClient{
		conn:   conn,
		buffer: make([]byte, 256*1024),
		exit:   make(chan struct{}),
	}
	go cli.readResponse()
	//msg, err := cli.ReadBlockHeight()
	//fmt.Printf("msg: %s\nerror:%v\n", msg, err)
	return &cli, nil
}

func (c *channelClient) Close() {
	c.conn.Close()
}

func (c *channelClient) Send(typ int, topic string, data interface{}) (string, error) {
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

func (c *channelClient) ReadBlockHeight() (string, error) {
	req := make(map[string]interface{})
	req["jsonrpc"] = "2.0"
	req["id"] = 1
	req["method"] = "getBlockNumber"
	req["params"] = []int{1}

	return c.Send(TypeRPCRequest, "", req)
}

func (c *channelClient) readResponse() {
	for {
		//TODO 有可能阻塞在 c.conn.Read 从而导致不能及时 <-c.exit
		select {
		case <-c.exit:
			return
		default:
			cnt, err := c.conn.Read(c.buffer)
			if err != nil {
				if err == io.EOF {
					return
				}
				fmt.Printf("ssl read error %v\n", err)
				continue
			}
			msg, err := DecodeMessage(c.buffer[:cnt])
			if err != nil {
				fmt.Printf("decode message error %v\n", err)
				continue
			}
			switch msg.Type {
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
					ch := ch.(chan *types.Log)
					for _, log := range eventLog.Logs {
						blkNumber, _ := hexutil.DecodeUint64(log.BlockNumber)
						data, _ := hexutil.Decode(log.Data)
						txIndex, _ := hexutil.DecodeUint64(log.TxIndex)
						logIndex, _ := hexutil.DecodeUint64(log.Index)
						ch <- &types.Log{
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

func (c *channelClient) SubEventLogs(arg RegisterEventLogRequest) (chan *types.Log, error) {
	if len(arg.FilterID) != 32 {
		return nil, errors.New("filterID invalid")
	}
	if ch, ok := c.watcher.Load(arg.FilterID); ok {
		return ch.(chan *types.Log), nil
	}
	msgSeq, err := c.Send(TypeRegisterEventLog, "", arg)
	if err != nil {
		return nil, err
	}
	pch := make(chan error)
	c.pending.Store(msgSeq, pch)
	err = <-pch
	c.pending.Delete(msgSeq)
	if err != nil {
		return nil, err
	}
	mch := make(chan *types.Log)
	c.watcher.Store(arg.FilterID, mch)
	return mch, nil
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
