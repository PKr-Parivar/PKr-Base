package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/PKr-Parivar/PKr-Base/config"
	"github.com/PKr-Parivar/PKr-Base/dialer"
	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/PKr-Parivar/PKr-Base/utils"
	"github.com/PKr-Parivar/PKr-Base/ws"

	"github.com/gorilla/websocket"
)

var WEBSOCKET_SERVER_ADDR_WITH_QUERY url.URL
var USER_CONF config.UserConfig

func init() {
	args := os.Args
	if len(args) > 2 && args[1] == "debug" {
		for i, arg := range args {
			if arg == "--fp" && i+1 < len(args) {
				utils.SetUserConfigDir(args[i+1])
			}
		}
	}

	err := logger.InitLogger()
	if err != nil {
		log.Println("Error while Initializing Logger:", err)
		log.Println("Source: init()")
		os.Exit(1)
	}

	USER_CONF, err = config.ReadFromUserConfigFile()
	if err != nil {
		logger.LOGGER.Println("Failed to Read from user-config:", err)
		logger.LOGGER.Println("Source: init()")
		os.Exit(1)
	}

	escaped_username := url.QueryEscape(USER_CONF.Username)
	escaped_password := url.QueryEscape(USER_CONF.Password)
	ws.MY_USERNAME = USER_CONF.Username
	ws.MY_SERVER_IP = USER_CONF.ServerIP

	raw_query := "username=" + escaped_username + "&password=" + escaped_password
	websock_server_ip := strings.Split(USER_CONF.ServerIP, ":")[0]

	WEBSOCKET_SERVER_ADDR_WITH_QUERY = url.URL{
		Scheme:   "ws",
		Host:     websock_server_ip + ":8080",
		Path:     "/ws",
		RawQuery: raw_query,
	}
}

func main() {
	logger.LOGGER.Println("==========================================================")
	logger.LOGGER.Println("\t\t\t\tPKR-Base Has Started")
	logger.LOGGER.Println("==========================================================")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	var server_err error

	websocker_dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		NetDialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // TCP connection timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	logger.LOGGER.Println("Attempting to Connect to the Web Socket Server... | IP: ", WEBSOCKET_SERVER_ADDR_WITH_QUERY.String())
	ws_conn, _, server_err := websocker_dialer.Dial(WEBSOCKET_SERVER_ADDR_WITH_QUERY.String(), nil)
	for server_err != nil {
		logger.LOGGER.Println("WebSocket connection failed:", server_err)

		var netErr net.Error
		if errors.As(server_err, &netErr) && netErr.Timeout() {
			logger.LOGGER.Println("Connection timed out. Will retry in 15 minutes.")
		} else if opErr, ok := server_err.(*net.OpError); ok {
			if sysErr, ok := opErr.Err.(*os.SyscallError); ok && strings.Contains(sysErr.Error(), "actively refused") {
				logger.LOGGER.Println("Server refused the connection.")
			} else {
				logger.LOGGER.Println("Unexpected Error - is an opError but not 'actively refused'")
				logger.LOGGER.Println("Error while Dialing Websocket Connection to Server: ", opErr)
				logger.LOGGER.Println("Source: main()")
			}
		} else {
			logger.LOGGER.Println("Unexpected Error - Not opError or timeout")
			logger.LOGGER.Println("Error while Dialing Websocket Connection to Server: ", server_err)
			logger.LOGGER.Println("Source: main()")
		}

		select {
		case <-time.After(15 * time.Minute):
			logger.LOGGER.Println("Retrying WebSocket connection...")
		case <-interrupt:
			logger.LOGGER.Println("Interrupt received. Exiting.")
			return
		}

		ws_conn, _, server_err = websocker_dialer.Dial(WEBSOCKET_SERVER_ADDR_WITH_QUERY.String(), nil)
	}

	defer ws_conn.Close()
	logger.LOGGER.Println("Connected to Server")

	done := make(chan struct{})

	go ws.ReadJSONMessage(done, ws_conn)
	go ws.PingPongWriter(done, ws_conn)

	logger.LOGGER.Println("Preparing gRPC Client ...")
	// New GRPC Client
	gRPC_cli_service_client, err := dialer.GetNewGRPCClient(USER_CONF.ServerIP)
	if err != nil {
		logger.LOGGER.Println("Error:", err)
		logger.LOGGER.Println("Description: Cannot Create New GRPC Client")
		logger.LOGGER.Println("Source: Install()")
		return
	}

	logger.LOGGER.Println("Checking for New Changes")
	// Checking for New Changes
	for _, get_workspace := range USER_CONF.GetWorkspaces {
		logger.LOGGER.Println("GET Workspace: ")
		logger.LOGGER.Println(get_workspace)
		are_there_new_changes, err := dialer.CheckForNewChanges(gRPC_cli_service_client, get_workspace.WorkspaceName, get_workspace.WorkspaceOwnerName, USER_CONF.Username, USER_CONF.Password, get_workspace.LastPushNum)
		if err != nil {
			logger.LOGGER.Println("Error while Checking For New Changes:", err)
			logger.LOGGER.Println("Source: main()")
			continue
		}
		logger.LOGGER.Println("Are there new changes:", are_there_new_changes)

		if are_there_new_changes {
			err = ws.PullWorkspace(get_workspace.WorkspaceOwnerName, get_workspace.WorkspaceName, ws_conn)
			if err != nil {
				if err.Error() == "workspace owner is offline" {
					logger.LOGGER.Println("Workspace Owner is Offline, Server'll notify when he's online")
					break
				}
				if err.Error() == "you already've latest version of workspace" {
					logger.LOGGER.Println("You've Lastest Version of Workspace, No Need to Transfer Data")
					return
				}
				logger.LOGGER.Println("Error while Pulling Data:", err)
				logger.LOGGER.Println("Source: main()")

				logger.LOGGER.Println("Will Try Again after 5 minutes")
				// Try Again only once after 5 minutes
				time.Sleep(5 * time.Minute)
				err = ws.PullWorkspace(get_workspace.WorkspaceOwnerName, get_workspace.WorkspaceName, ws_conn)
				if err != nil {
					logger.LOGGER.Println("Error while Pulling Data Again:", err)
					logger.LOGGER.Println("Source: main()")
				}
			}
		}
	}
	logger.LOGGER.Println("Done with Checking for New Changes ...")

	select {
	case <-done:
	case <-interrupt:
		logger.LOGGER.Println("Interrupt Received, Closing Connection ...")

		err := ws_conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Bye"))
		if err != nil {
			logger.LOGGER.Println("Error while Writing Close Message to Server via WS:", err)
			logger.LOGGER.Println("Source: main()")
			return
		}
	}
}
