package dialer

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"time"

	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/ccding/go-stun/stun"
)

const (
	CLIENT_BASE_HANDLER_NAME = "ClientHandler"
	CONTEXT_TIMEOUT          = 45 * time.Second
	LONG_CONTEXT_TIMEOUT     = 10 * time.Minute
)

func GetMyPublicIP(port int) (string, error) {
	stunClient := stun.NewClient()
	stunClient.SetServerAddr("stun.l.google.com:19302")
	stunClient.SetLocalPort(port)

	_, myExtAddr, err := stunClient.Discover()
	if err != nil && err.Error() != "Server error: no changed address" {
		return "", err
	}
	return myExtAddr.String(), nil
}

func CallKCP_RPC_WithContext(ctx context.Context, args, reply any, rpc_name string, rpc_client *rpc.Client) error {
	// Create a channel to handle the RPC call with context
	done := make(chan error, 1)
	go func() {
		done <- rpc_client.Call(rpc_name, args, reply)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("RPC call timed out")
	case err := <-done:
		return err
	}
}

func GetMyPrivateIP() (string, error) {
	// Connect to a remote address (doesn't actually send data)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logger.LOGGER.Println("Error while Dialing to 8.8.8.8:80:", err)
		logger.LOGGER.Println("Source: GetMyPrivateIP()")
		return "", err
	}
	defer conn.Close()

	// Get the local address from the connection
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
