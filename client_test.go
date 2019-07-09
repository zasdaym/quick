package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	// TODO: abort the test if these two ports are occupied
	addrNotListened = "https://127.0.0.1:11111"
	addrListened    = "https://127.0.0.1:28443"
)

type ClientSuite struct {
	suite.Suite
}

func (suite *ClientSuite) SetupTest() {
	saveArgs()
}

func TestClientTestSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (suite *ClientSuite) TearDownTest() {
	resetArgs()
}

func (suite *ClientSuite) TestConnectTimeout() {
	address = addrNotListened
	connectTimeout = 10 * time.Millisecond

	t := suite.T()
	err := run()
	assert.NotNil(t, err)
	assert.Equal(t, "Get "+address+": connect timeout", err.Error())
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}

var (
	tlsCfg = generateTLSConfig()
)

func startServer(done chan struct{}, handler http.Handler) {
	netAddr, err := url.Parse(addrListened)
	if err != nil {
		panic(err)
	}

	server := &h2quic.Server{
		Server: &http.Server{
			Addr:    netAddr.Host,
			Handler: handler,
		},
	}
	server.TLSConfig = tlsCfg

	err = server.Serve(nil)
	if err != nil {
		panic(err)
	}
	<-done
	err = server.Close()
	if err != nil {
		panic(err)
	}
}

func (suite *ClientSuite) TestMaxTime() {
	address = addrListened
	insecure = true
	connectTimeout = 20 * time.Millisecond
	maxTime = 30 * time.Millisecond

	done := make(chan struct{})
	defer func() { close(done) }()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1000 * time.Millisecond)
	})
	go func() {
		startServer(done, handler)
	}()

	t := suite.T()
	err := run()
	if err == nil {
		assert.NotNil(t, err)
	} else {
		assert.Equal(t, "Get "+address+": context deadline exceeded", err.Error())
	}
}
