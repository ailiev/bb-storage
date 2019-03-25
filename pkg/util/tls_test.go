package util

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	pb "github.com/buildbarn/bb-storage/pkg/proto/configuration/server"
	"github.com/stretchr/testify/require"
)

func TestValidateGood(t *testing.T) {
	cfg := &pb.TLSConfiguration{KeyFile: "testdata/key.pem", CertFile: "testdata/cert.pem"}
	require.Nil(t, ValidateTls(cfg))
}

func TestValidateNotEnabled(t *testing.T) {
	require.Nil(t, ValidateTls(nil))
}

func TestValidateMissingKey(t *testing.T) {
	cfg := &pb.TLSConfiguration{KeyFile: "", CertFile: "testdata/cert.pem"}
	require.Equal(t, fmt.Errorf("Must supply both cert file and key file if using TLS"),
		ValidateTls(cfg))
}

func TestLoadTlsFiles(t *testing.T) {
	cfg := &pb.TLSConfiguration{KeyFile: "testdata/key.pem", CertFile: "testdata/cert.pem"}
	creds, err := MakeGrpcCreds(cfg)
	require.Nil(t, err)
	require.NotNil(t, creds)

	info := creds.Info()
	require.Equal(t, info.SecurityProtocol, "tls")
	require.Equal(t, info.SecurityVersion, "1.2")
}

func TestLoadNoTlsFiles(t *testing.T) {
	creds, err := MakeGrpcCreds(nil)
	require.Nil(t, err)
	require.Nil(t, creds)
}

func TestLoadTlsFileMissing(t *testing.T) {
	cert, key := "testdata/cert.pem", "testdata/keyNOT.pem"
	cfg := &pb.TLSConfiguration{KeyFile: key, CertFile: cert}
	creds, err := MakeGrpcCreds(cfg)
	require.Nil(t, creds)
	require.Equal(t, fmt.Errorf(
		"Can't load X509 certificate and keypair with config key_file:\"testdata/keyNOT.pem\" cert_file:\"testdata/cert.pem\" : open testdata/keyNOT.pem: no such file or directory"),
		err)
}

func TestStartHttpNoTls(t *testing.T) {
	successfulHttpStart(t, nil)
}

func TestStartHttpWithTls(t *testing.T) {
	successfulHttpStart(t, &pb.TLSConfiguration{
		CertFile: "testdata/cert.pem", KeyFile: "testdata/key.pem"})
}

func successfulHttpStart(t *testing.T, cfg *pb.TLSConfiguration) {
	addr := ":0"
	srv, listener, listenErr := makeHttpServer(&addr, nil)
	require.Nil(t, listenErr)
	defer listener.Close()

	go func() {
		serveErr := httpServe(srv, listener, cfg)
		t.Logf("serve failed or completed with: %v", serveErr)

	}()
	defer srv.Close()

	time.Sleep(1 * time.Second)

	// check the server is actually in good shape
	withTls := cfg != nil
	var prot string
	if withTls {
		prot = "https"
	} else {
		prot = "http"
	}
	actualAddr := listener.Addr().String()
	_, getErr := http.Get(prot + "://" + actualAddr + "/")
	if !withTls {
		require.Nil(t, getErr)
	} else {
		// will not be fully successful with the self-published certificate,
		// so success is a specific X509 error.
		switch errSub := getErr.(type) {
		case *url.Error:
			switch errSub.Err.(type) {
			case x509.HostnameError:
				// as good as it gets
			default:
				t.Errorf("TLS URL error had unexpected cause, was %v", getErr)
			}
		default:
			t.Errorf("Unexpected http-TLS request error %v", getErr)
		}
	}
}
