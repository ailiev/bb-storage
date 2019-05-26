package builder

import (
	"context"
	"io"
	"log"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

	"github.com/buildbarn/bb-storage/pkg/util"

	"google.golang.org/grpc"
)

type forwardingBuildQueue struct {
	capabilitiesClient remoteexecution.CapabilitiesClient
	executionClient    remoteexecution.ExecutionClient
}

// NewForwardingBuildQueue creates a GRPC service for the Capbilities and
// Execution service that simply forwards all requests to a GRPC client. This
// may be used by the frontend processes to forward execution requests to
// scheduler processes in unmodified form.
//
// Details: https://github.com/grpc/grpc-go/issues/2297
func NewForwardingBuildQueue(client *grpc.ClientConn) BuildQueue {
	return &forwardingBuildQueue{
		capabilitiesClient: remoteexecution.NewCapabilitiesClient(client),
		executionClient:    remoteexecution.NewExecutionClient(client),
	}
}

func (bq *forwardingBuildQueue) GetCapabilities(ctx context.Context, in *remoteexecution.GetCapabilitiesRequest) (*remoteexecution.ServerCapabilities, error) {
	log.Printf("forwarding_build_queue: fwd getcapreq: %s", in.String())
	return bq.capabilitiesClient.GetCapabilities(ctx, in)
}

func forwardOperations(client remoteexecution.Execution_ExecuteClient, server remoteexecution.Execution_ExecuteServer) error {
	for {
		operation, err := client.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("forwarding_build_queue: upstream executor recv operation returned EOF")
				return nil
			}
			return util.StatusWrap(err, "forwarding_build_queue: upstream executor recv operation")
		}
		log.Printf("forwarding_build_queue: fwd Op from server: %s", operation.String())

		if err := server.Send(operation); err != nil {
			log.Printf("forwarding_build_queue: sending operation to our client failed: %q", err)
			return err
		}
	}
}

func (bq *forwardingBuildQueue) Execute(in *remoteexecution.ExecuteRequest, out remoteexecution.Execution_ExecuteServer) error {
	log.Printf("forwarding_build_queue: fwd ExReq: %s", in.String())
	client, err := bq.executionClient.Execute(out.Context(), in)
	if err != nil {
		return util.StatusWrap(err, "forwarding_build_queue Execute")
	}
	return forwardOperations(client, out)
}

func (bq *forwardingBuildQueue) WaitExecution(in *remoteexecution.WaitExecutionRequest, out remoteexecution.Execution_WaitExecutionServer) error {
	log.Printf("forwarding_build_queue: fwd WaitExReq: %s", in.String())
	client, err := bq.executionClient.WaitExecution(out.Context(), in)
	if err != nil {
		return util.StatusWrap(err, "forwarding_build_queue WaitExecution")
	}
	return forwardOperations(client, out)
}
