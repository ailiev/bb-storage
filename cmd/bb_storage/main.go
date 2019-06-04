package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"go.opencensus.io/plugin/ocgrpc"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-storage/pkg/ac"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/builder"
	"github.com/buildbarn/bb-storage/pkg/cas"
	"github.com/buildbarn/bb-storage/pkg/opencensus"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	var (
		blobstoreConfig      = flag.String("blobstore-config", "/config/blobstore.conf", "Configuration for blob storage")
		webListenAddress     = flag.String("web.listen-address", ":80", "Port on which to expose metrics")
		agentEndpointURI     = flag.String("jaeger.agent-endpoint", "127.0.0.1:6831", "Jaeger agent address")
		collectorEndpointURI = flag.String("jaeger.collector-endpoint", "http://127.0.0.1:14268/api/traces", "Jaeger collector endpoint")
		serviceName          = flag.String("trace.service-name", "bb_storage", "Service name for tracing")
		alwaysSample         = flag.Bool("trace.always-sample", false, "Record all traces.")
		certFile         = flag.String("tls-cert-file", "", "Certificate file for TLS server authentication")
		keyFile          = flag.String("tls-key-file", "", "Key file for TLS server authentication")
	)
	var schedulersList util.StringList
	flag.Var(&schedulersList, "scheduler", "Backend capable of executing build actions. Example: debian8|hostname-of-debian8-scheduler:8981")
	var allowActionCacheUpdatesForInstancesList util.StringList
	flag.Var(&allowActionCacheUpdatesForInstancesList, "allow-ac-updates-for-instance", "Allow clients to write into the action cache for this instance")
	flag.Parse()

	err := util.UseBinaryLogTempFileSink()
	if err != nil {
		log.Fatalf("Failed to UseBinaryLogTempFileSink: %v", err)
	}

	// Web server for metrics and profiling.
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(*webListenAddress, nil))
	}()

	opencensus.Initialize(*agentEndpointURI, *collectorEndpointURI, *serviceName, *alwaysSample)

	// Storage access.
	contentAddressableStorageBlobAccess, actionCacheBlobAccess, err := configuration.CreateBlobAccessObjectsFromConfig(*blobstoreConfig)
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}
	actionCache := ac.NewBlobAccessActionCache(actionCacheBlobAccess)

	// Let GetCapabilities() work, even for instances that don't
	// have a scheduler attached to them, but do allow uploading
	// results into the Action Cache.
	schedulers := map[string]builder.BuildQueue{}
	allowActionCacheUpdatesForInstances := map[string]bool{}
	if len(allowActionCacheUpdatesForInstancesList) > 0 {
		fallback := builder.NewNonExecutableBuildQueue()
		for _, instance := range allowActionCacheUpdatesForInstancesList {
			schedulers[instance] = fallback
			allowActionCacheUpdatesForInstances[instance] = true
		}
	}

	// Backends capable of compiling.
	for _, schedulerEntry := range schedulersList {
		components := strings.SplitN(schedulerEntry, "|", 2)
		if len(components) != 2 {
			log.Fatal("Invalid scheduler entry: ", schedulerEntry)
		}
		scheduler, err := grpc.Dial(
			components[1],
			grpc.WithInsecure(),
			grpc.WithUnaryInterceptor(grpc_prometheus.UnaryClientInterceptor),
			grpc.WithStreamInterceptor(grpc_prometheus.StreamClientInterceptor))
		if err != nil {
			log.Fatal("Failed to create scheduler RPC client: ", err)
		}
		schedulers[components[0]] = builder.NewForwardingBuildQueue(scheduler)
	}
	buildQueue := builder.NewDemultiplexingBuildQueue(func(instance string) (builder.BuildQueue, error) {
		scheduler, ok := schedulers[instance]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "Unknown instance name")
		}
		return scheduler, nil
	})

	// RPC server.
	opts := []grpc.ServerOption {
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
	}
	creds, err := util.MakeCreds(certFile, keyFile)
	if err != nil {
		log.Fatal("Loading TLS materials failed: ", err)
	} else if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}
	s := grpc.NewServer(opts...)
	remoteexecution.RegisterActionCacheServer(s, ac.NewActionCacheServer(actionCache, allowActionCacheUpdatesForInstances))
	remoteexecution.RegisterContentAddressableStorageServer(s, cas.NewContentAddressableStorageServer(contentAddressableStorageBlobAccess))
	bytestream.RegisterByteStreamServer(s, cas.NewByteStreamServer(contentAddressableStorageBlobAccess, 1<<16))
	remoteexecution.RegisterCapabilitiesServer(s, buildQueue)
	remoteexecution.RegisterExecutionServer(s, buildQueue)
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(s)

	sock, err := net.Listen("tcp", ":8980")
	if err != nil {
		log.Fatal("Failed to create listening socket: ", err)
	}
	if err := s.Serve(sock); err != nil {
		log.Fatal("Failed to serve RPC server: ", err)
	}
}
