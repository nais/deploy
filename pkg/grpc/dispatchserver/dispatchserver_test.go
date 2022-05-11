package dispatchserver

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	presharedkey_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/presharedkey"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/github"
	"github.com/nais/deploy/pkg/pb"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func bufDialer(b *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(context.Context, string) (net.Conn, error) {
		return b.Dial()
	}
}

func TestInterceptors(t *testing.T) {
	ctx := context.Background()

	deploymentStore := database.MockDeploymentStore{}
	ds := New(&deploymentStore, github.FakeClient())
	deploymentStore.On("HistoricDeployments", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	presharedkeyInterceptor := &presharedkey_interceptor.ServerInterceptor{
		Keys: []string{"secret"},
	}

	b := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(
		grpc.StreamInterceptor(presharedkeyInterceptor.StreamServerInterceptor),
		grpc.UnaryInterceptor(presharedkeyInterceptor.UnaryServerInterceptor),
	)

	pb.RegisterDispatchServer(srv, ds)

	go func(srv *grpc.Server) {
		err := srv.Serve(b)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Error(err)
		}
	}(srv)

	t.Run("test correct password gets deployment reques", func(t *testing.T) {
		pskClientInterceptor := &presharedkey_interceptor.ClientInterceptor{RequireTLS: false, Key: "secret"}
		conn, _ := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer(b)), grpc.WithInsecure(), grpc.WithPerRPCCredentials(pskClientInterceptor))

		client := pb.NewDispatchClient(conn)
		deploymentsClient, err := client.Deployments(ctx, &pb.GetDeploymentOpts{Cluster: "test"})
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(1 * time.Second)
		ds.SendDeploymentRequest(ctx, &pb.DeploymentRequest{
			Cluster: "test",
			Team:    "test",
		})

		r, err := deploymentsClient.Recv()
		if err != nil {
			t.Fatal(err)
		}

		if r.Team != "test" {
			t.Error("invalid deployments request received")
		}
	})

	t.Run("test wrong password does not get deployment request", func(t *testing.T) {
		pskClientInterceptor := &presharedkey_interceptor.ClientInterceptor{RequireTLS: false, Key: "wrong"}
		conn, _ := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer(b)), grpc.WithInsecure(), grpc.WithPerRPCCredentials(pskClientInterceptor))

		client := pb.NewDispatchClient(conn)
		deploymentsClient, err := client.Deployments(ctx, &pb.GetDeploymentOpts{Cluster: "test2"})
		if err != nil {
			t.Fatal("failed to get deployments client", err)
		}

		if deploymentsClient == nil {
			t.Fatal("deployments client should not be nil")
		}

		time.Sleep(1 * time.Second)
		// This should not be received
		ds.SendDeploymentRequest(ctx, &pb.DeploymentRequest{
			Cluster: "test2",
			Team:    "test2",
		})

		req, err := deploymentsClient.Recv()
		if status.Code(err) != codes.PermissionDenied {
			t.Error("should have gotten permission denied error when unauthenticated", err)
		}

		if req != nil {
			t.Error("we should not get a deployment request when unauthenticated")
		}
	})
}
