package util

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"

	pb "github.com/buildbarn/bb-storage/pkg/proto/configuration/server"

	"google.golang.org/grpc/credentials"
)

// ValidateTls checks params are in order, return non-nil if not.
// must be called before using.
func ValidateTls(p *pb.TLSConfiguration) error {
	if p == nil {
		return nil
	}
	if p.CertFile == "" || p.KeyFile == "" {
		return errors.New("Must supply both cert file and key file if using TLS")
	}
	return nil
}

// MakeGrpcCreds Create TLS credentials for GRPC
// return nil,nil to indicate no TLS if neither parameter is configured (empty string)
func MakeGrpcCreds(tlsParams *pb.TLSConfiguration) (credentials.TransportCredentials, error) {
	if tlsParams == nil {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(tlsParams.CertFile, tlsParams.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("Can't load X509 certificate and keypair with config %v: %s",
			tlsParams, err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	creds := credentials.NewTLS(cfg)
	return creds, nil
}

// several helpers to help with testing
func makeHttpServer(addr *string, handler http.Handler) (*http.Server, net.Listener, error) {
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		return nil, nil, err
	}
	s := &http.Server{
		Addr:    *addr,
		Handler: handler,
	}
	return s, l, nil
}

func httpServe(server *http.Server, listener net.Listener, tlsParams *pb.TLSConfiguration) error {
	var err error
	if tlsParams == nil {
		err = server.Serve(listener)
	} else {
		if valErr := ValidateTls(tlsParams); valErr != nil {
			return valErr
		}
		err = server.ServeTLS(listener, tlsParams.CertFile, tlsParams.KeyFile)
	}
	return err
}

// HttpListenAndServe wrapper around http ListenAndServe and ListenAndServeTLS,
// depending on whether TLS materials were specified (non-empty strings)
func HttpListenAndServe(addr *string, cfg *pb.TLSConfiguration, handler http.Handler) error {
	server, listener, err := makeHttpServer(addr, handler)
	if err != nil {
		return StatusWrapf(err, "Failed to listen on %s", *addr)
	}
	defer listener.Close()
	return httpServe(server, listener, cfg)
}
