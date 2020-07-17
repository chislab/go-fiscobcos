package tls

import (
	"errors"
	"io/ioutil"

	"github.com/FISCO-BCOS/crypto/tls"
	"github.com/FISCO-BCOS/crypto/x509"
)

type Connection struct {
	conn     *tls.Conn
	conf     *tls.Config
	endpoint string
}

func Dial(caFile, certFile, keyFile, endpoint string) (*Connection, error) {
	caBytes, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
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
	conn, err := tls.Dial("tcp", endpoint, tlsConf)
	if err != nil {
		return nil, err
	}
	return &Connection{
		conn:     conn,
		conf:     tlsConf,
		endpoint: endpoint,
	}, nil
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) ReadWithChannel(b []byte, ch chan *ReadResult) {
	cnt, err := c.conn.Read(b)
	ch <- &ReadResult{
		Count: cnt,
		Error: err,
	}
}

func (c *Connection) Read(b []byte) (int, error) {
	return c.conn.Read(b)
}

func (c *Connection) Write(b []byte) (int, error) {
	return c.conn.Write(b)
}

func (c *Connection) Reconnecte() error {
	conn, err := tls.Dial("tcp", c.endpoint, c.conf)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

type ReadResult struct {
	Count int
	Error error
}
