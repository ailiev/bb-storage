package util

import (
	"fmt"
	"crypto/tls"

	"google.golang.org/grpc/credentials"
)

// Create TLS credentials from file names.
// return nil,nil to indicate no TLS if neither parameter is configured (empty string)
func MakeCreds(certFile * string, keyFile * string) (credentials.TransportCredentials, error) {
	if *certFile == "" && *keyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		return nil, fmt.Errorf("Can't load X509 certificate and keypair '%s', '%s': %s", *certFile, *keyFile, err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	creds := credentials.NewTLS(cfg)
	return creds, nil
}
