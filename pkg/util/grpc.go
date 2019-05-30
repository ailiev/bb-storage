package util

import (
	"os"

	"google.golang.org/grpc/internal/binarylog"
)

// UseBinaryLogTempFileSink Configure process to use /tmp file for grpc binary logging
func UseBinaryLogTempFileSink() error {
	// Do not create the temp file if logging not enabled.
	_, present := os.LookupEnv("GRPC_BINARY_LOG_FILTER")
	if !present {
		return nil
	}
	sink, err := binarylog.NewTempFileSink()
	if err != nil {
		return err
	}
	binarylog.SetDefaultSink(sink)
	return nil
}
