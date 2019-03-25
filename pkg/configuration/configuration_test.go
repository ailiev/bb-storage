package configuration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFull(t *testing.T) {
	cfg, err := GetStorageConfiguration("testdata/fullconfig.json")
	require.Nil(t, err)
	require.Equal(t, "cert.pem", cfg.Tls.CertFile)
	require.Equal(t, ":4251", cfg.GrpcListenAddress)
}
