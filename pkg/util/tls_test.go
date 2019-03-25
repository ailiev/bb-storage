package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTlsFiles(t *testing.T) {
	cert,key := "testdata/cert.pem", "testdata/key.pem"
	creds,err := MakeCreds(&cert, &key)
	require.Nil(t, err)
	require.NotNil(t, creds)

	info := creds.Info()
	require.Equal(t, info.SecurityProtocol, "tls")
	require.Equal(t, info.SecurityVersion, "1.2")
}

func TestLoadNoTlsFiles(t *testing.T) {
	empty := ""
	creds,err := MakeCreds(&empty, &empty)
	require.Nil(t, err)
	require.Nil(t, creds)
}

func TestLoadTlsFileMissing(t *testing.T) {
	cert,key := "testdata/cert.pem", "testdata/keyNOT.pem"
	creds,err := MakeCreds(&cert, &key)
	require.Nil(t, creds)
	require.Equal(t, err, fmt.Errorf("Can't load X509 certificate and keypair 'testdata/cert.pem', 'testdata/keyNOT.pem': open testdata/keyNOT.pem: no such file or directory"))
}
