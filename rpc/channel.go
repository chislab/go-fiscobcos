package rpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io/ioutil"

	"github.com/chislab/go-fiscobcos/rpc/tls"
	"github.com/chislab/go-fiscobcos/crypto/x509"
)
//
//type ChanClient struct {
//	conn    *tls.Conn
//	buffer  []byte
//	exit    chan struct{}
//	pending sync.Map
//	watcher sync.Map
//}

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
		return NewFuncCodec(conn, conn.WriteJSON, conn.ReadJSON), nil
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

func (msg *Message) Encode() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, msg.Length)
	binary.Write(buf, binary.BigEndian, msg.Type)
	binary.Write(buf, binary.LittleEndian, []byte(msg.Seq))
	binary.Write(buf, binary.BigEndian, msg.Result)
	buf.WriteByte(msg.TopicLength)
	buf.WriteString(msg.Topic)
	binary.Write(buf, binary.LittleEndian, msg.Data)
	return buf.Bytes()
}
