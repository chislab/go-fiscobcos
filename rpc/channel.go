package rpc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"github.com/chislab/go-fiscobcos/rpc/tls"
	"github.com/chislab/go-fiscobcos/crypto/x509"
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

func DialChanWithDialer(ctx context.Context, conf *ClientConfig) (*Client, error) {
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

	return newClient(ctx, func(ctx context.Context) (ServerCodec, error) {
		conn, err := tls.Dial("tcp", conf.Endpoint, tlsConf)
		if err != nil {
			return nil, err
		}
		return NewFuncCodec(conn, conn.WriteBytes, conn.ReadJson), nil
	})
}

type Message struct {
	Length      uint32
	Type        uint16
	Seq         string
	Result      int32
	TopicLength byte
	Topic       string
	Data        []byte
}

func NewMessage(typ int, topic string, data interface{}) (*Message, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	msg := &Message{
		Length:      uint32(42 + len(d)),
		Type:        uint16(typ),
		Seq:         newSeq(),
		TopicLength: byte(len(topic) + 1),
		Topic:       topic,
		Result:      0,
		Data:        d,
	}
	if typ == TypeRegisterEventLog {
		msg.Length += uint32(1 + len(topic))
	}
	return msg, nil
}

func (msg *Message) Encode() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, msg.Length)
	binary.Write(buf, binary.BigEndian, msg.Type)
	binary.Write(buf, binary.LittleEndian, []byte(msg.Seq))
	binary.Write(buf, binary.BigEndian, msg.Result)
	if msg.Type == TypeRegisterEventLog {
		buf.WriteByte(msg.TopicLength)
		buf.WriteString(msg.Topic)
	}
	binary.Write(buf, binary.LittleEndian, msg.Data)
	return buf.Bytes()
}

func newSeq() string {
	var buf [16]byte
	io.ReadFull(rand.Reader, buf[:])
	return hex.EncodeToString(buf[:])
}
