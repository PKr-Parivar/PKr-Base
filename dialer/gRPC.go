package dialer

import (
	"context"
	"fmt"

	"github.com/PKr-Parivar/PKr-Base/logger"
	pb "github.com/PKr-Parivar/PKr-Base/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func GetNewGRPCClient(address string) (pb.CliServiceClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Println("Error while Creating New gRPC Client:", err)
		fmt.Println("Source: GetNewGRPCClient()")
		return nil, err
	}
	return pb.NewCliServiceClient(conn), nil
}

func CheckForNewChanges(grpc_client pb.CliServiceClient, workspace_name, workspace_owner_name, listener_username, listener_password string, last_push_num int) (bool, error) {
	logger.LOGGER.Println("Preparing gRPC Request ...")
	// Prepare req
	req := &pb.GetLastPushNumOfWorkspaceRequest{
		WorkspaceOwner:   workspace_owner_name,
		WorkspaceName:    workspace_name,
		ListenerUsername: listener_username,
		ListenerPassword: listener_password,
	}

	// Request Timeout
	ctx, cancelFunc := context.WithTimeout(context.Background(), CONTEXT_TIMEOUT)
	defer cancelFunc()

	logger.LOGGER.Println("Sending gRPC Request ...")
	// Sending Request ...
	res, err := grpc_client.GetLastPushNumOfWorkspace(ctx, req)
	if err != nil {
		logger.LOGGER.Println("Error:", err)
		logger.LOGGER.Println("Description: Error in Getting Last Push Number from Server")
		logger.LOGGER.Println("Source: CheckForNewChanges()")
		return false, err
	}
	logger.LOGGER.Println("Latest Push Num Received from Server:", res.LastPushNum)
	logger.LOGGER.Println("My Latest Push Num:", last_push_num)
	return res.LastPushNum != int32(last_push_num), nil
}
