package handler

import (
	"math/rand"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PKr-Parivar/PKr-Base/dialer"
	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/PKr-Parivar/PKr-Base/utils"

	"github.com/ButterHost69/kcp-go"
)

type ClientHandlerNameManager struct {
	sync.Mutex
	RandomStringList []string
}

var clientHandlerNameManager = ClientHandlerNameManager{
	RandomStringList: []string{},
}

func HandleNotifyToPunchRequest(peer_public_ip, peer_public_port string, peer_private_ip string, peer_private_port string) (string, string, string, string, error) {
	local_port := rand.Intn(16384) + 16384
	logger.LOGGER.Println("My Local Port:", local_port)

	// Get My Public IP
	my_public_IP, err := dialer.GetMyPublicIP(local_port)
	if err != nil {
		logger.LOGGER.Println("Error while Getting my Public IP:", err)
		logger.LOGGER.Println("Source: HandleNotifyToPunch()")
		return "", "", "", "", err
	}
	logger.LOGGER.Println("My Public IP Addr:", my_public_IP)

	ip_port_split := strings.Split(my_public_IP, ":")
	my_public_IP_only := ip_port_split[0]
	my_public_port_only := ip_port_split[1]
	my_private_ip, err := dialer.GetMyPrivateIP()
	if err != nil {
		logger.LOGGER.Println("Error while Getting Private IP:", err)
		logger.LOGGER.Println("Source: HandleNotifyToPunch()")
		return "", "", "", "", err
	}

	udp_local_addr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(local_port))
	if err != nil {
		logger.LOGGER.Println("Error while Resolving UDP Addr for Random Local Port:", err)
		logger.LOGGER.Println("Source: HandleNotifyToPunch()")
		return "", "", "", "", err
	}

	// Creating UDP Conn to Perform UDP NAT Hole Punching
	udp_conn, err := net.ListenUDP("udp", udp_local_addr)
	if err != nil {
		logger.LOGGER.Printf("Error while Listening to %d: %v\n", local_port, err)
		logger.LOGGER.Println("Source: HandleNotifyToPunch()")
		return "", "", "", "", err
	}

	// Creating Unique ClientHandlerName
	client_handler_name := utils.RandomString(4)
	for slices.Contains(clientHandlerNameManager.RandomStringList, client_handler_name) {
		client_handler_name = utils.RandomString(4)
	}

	go func() {
		defer udp_conn.Close()
		time.Sleep(5 * time.Second)
		logger.LOGGER.Println("Initializing UDP NAT Hole Punching")

		var workspace_owner_ip string
		if peer_public_ip == my_public_IP_only {
			logger.LOGGER.Println("Sending Request via Private IP ...")
			workspace_owner_ip = peer_private_ip + ":" + peer_private_port
		} else {
			logger.LOGGER.Println("Sending Request via Public IP ...")
			workspace_owner_ip = peer_public_ip + ":" + peer_public_port
		}

		err = dialer.WorkspaceOwnerUdpNatPunching(udp_conn, workspace_owner_ip, client_handler_name)
		if err != nil {
			logger.LOGGER.Println("Error while Performing UDP NAT Hole Punching:", err)
			logger.LOGGER.Println("Source: HandleNotifyToPunch()")
			udp_conn.Close()
			return
		}

		logger.LOGGER.Println("Starting New New Server `Connection` server on local port:", local_port)
		StartNewNewServer(udp_conn, client_handler_name)
	}()

	// Sending Response to Server
	return my_public_IP_only, my_public_port_only, my_private_ip, strconv.Itoa(local_port), nil
}

func StartNewNewServer(udp_conn *net.UDPConn, clientHandlerName string) {
	logger.LOGGER.Println("ClientHandler"+clientHandlerName, "Started")
	err := RegisterName("ClientHandler"+clientHandlerName, &ClientHandler{})
	if err != nil {
		logger.LOGGER.Println("Error while Register ClientHandler:", err)
		logger.LOGGER.Println("Source: StartNewNewServer()")
		return
	}

	kcp_lis, err := kcp.ListenWithOptionsAndConn(udp_conn, nil, 0, 0)
	if err != nil {
		logger.LOGGER.Println("Error while Listening KCP With Options & Conn:", err)
		logger.LOGGER.Println("Source: StartNewNewServer()")
		return
	}
	logger.LOGGER.Println("Started New KCP Server Started ...")

	err = kcp_lis.SetReadDeadline(time.Now().Add(5 * time.Minute))
	if err != nil {
		logger.LOGGER.Println("Error while Setting Deadline for KCP Listener:", err)
		logger.LOGGER.Println("Source: StartNewNewServer()")
		return
	}

	for {
		kcp_session, err := kcp_lis.AcceptKCP()
		if err != nil {
			logger.LOGGER.Println("Error while Accepting KCP from KCP Listener:", err)
			logger.LOGGER.Println("Source: StartNewNewServer()")
			// TODO: Only Close KCP Listener if there's Timeout Error
			kcp_lis.Close()
			logger.LOGGER.Println("Closing NewNewServer with Local Port:", udp_conn.LocalAddr().String())
			return
		}
		logger.LOGGER.Println("New Incoming Connection in NewNewServer from:", kcp_session.RemoteAddr())

		// KCP Params for Congestion Control
		kcp_session.SetWindowSize(128, 1024)
		kcp_session.SetNoDelay(1, 10, 2, 1)
		kcp_session.SetACKNoDelay(true)
		kcp_session.SetDSCP(46)

		go func() {
			defer kcp_session.Close()
			logger.LOGGER.Println("Deciding the Type of Session ...")

			var buff [3]byte
			_, err = kcp_session.Read(buff[:])
			if err != nil {
				logger.LOGGER.Println("Error while Reading the type of Session(KCP-RPC or KCP-Plain):", err)
				logger.LOGGER.Println("Source: StartNewNewServer()")
				return
			}
			logger.LOGGER.Println("Type of Session Received from Listener ...")

			kcp_buff := [3]byte{'K', 'C', 'P'}
			rpc_buff := [3]byte{'R', 'P', 'C'}

			switch buff {
			case kcp_buff:
				logger.LOGGER.Println("KCP-Plain:", kcp_session.RemoteAddr().String())
				GetDataHandler(kcp_session)
			case rpc_buff:
				logger.LOGGER.Println("KCP-RPC:", kcp_session.RemoteAddr().String())
				ServeConn(kcp_session)
			default:
				logger.LOGGER.Println("Unknown Type of Session Sent:", string(buff[:]))
			}
		}()
	}
}
