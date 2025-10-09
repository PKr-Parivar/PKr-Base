package ws

import (
	"bufio"
	"errors"
	"fmt"

	"math/rand"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PKr-Parivar/PKr-Base/config"
	"github.com/PKr-Parivar/PKr-Base/dialer"
	"github.com/PKr-Parivar/PKr-Base/encrypt"
	"github.com/PKr-Parivar/PKr-Base/filetracker"
	"github.com/PKr-Parivar/PKr-Base/handler"
	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/PKr-Parivar/PKr-Base/models"
	"github.com/ButterHost69/kcp-go"
	"github.com/gen2brain/beeep"
	"github.com/gorilla/websocket"
)

const DATA_CHUNK = handler.DATA_CHUNK

var MY_USERNAME string
var MY_SERVER_IP string

func connectToAnotherUser(workspace_owner_username string, conn *websocket.Conn) (string, string, *net.UDPConn, *kcp.UDPSession, error) {
	local_port := rand.Intn(16384) + 16384
	logger.LOGGER.Println("My Local Port:", local_port)

	// Get My Public IP
	my_public_IP, err := dialer.GetMyPublicIP(local_port)
	if err != nil {
		logger.LOGGER.Println("Error while Getting my Public IP:", err)
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		return "", "", nil, nil, err
	}
	logger.LOGGER.Println("My Public IP Addr:", my_public_IP)

	my_public_IP_split := strings.Split(my_public_IP, ":")
	my_public_IP_only := my_public_IP_split[0]
	my_public_port_only := my_public_IP_split[1]

	private_ip, err := dialer.GetMyPrivateIP()
	if err != nil {
		logger.LOGGER.Println("Error while Getting My Private IP:", err)
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		return "", "", nil, nil, err
	}

	var req_punch_from_receiver_request models.RequestPunchFromReceiverRequest
	req_punch_from_receiver_request.WorkspaceOwnerUsername = workspace_owner_username
	req_punch_from_receiver_request.ListenerUsername = MY_USERNAME
	req_punch_from_receiver_request.ListenerPublicIp = my_public_IP_only
	req_punch_from_receiver_request.ListenerPublicPort = my_public_port_only
	req_punch_from_receiver_request.ListenerPrivatePort = strconv.Itoa(local_port)
	req_punch_from_receiver_request.ListenerPrivateIp = private_ip

	logger.LOGGER.Println("Calling RequestPunchFromReceiverRequest ...")
	err = conn.WriteJSON(models.WSMessage{
		MessageType: "RequestPunchFromReceiverRequest",
		Message:     req_punch_from_receiver_request,
	})
	if err != nil {
		logger.LOGGER.Println("Error while Sending RequestPunchFromReceiverRequest to WS Server:", err)
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		return "", "", nil, nil, err

	}

	var req_punch_from_receiver_response models.RequestPunchFromReceiverResponse
	var ok, invalid_flag bool
	count := 0

	for {
		time.Sleep(5 * time.Second)
		RequestPunchFromReceiverResponseMap.Lock()
		req_punch_from_receiver_response, ok = RequestPunchFromReceiverResponseMap.Map[workspace_owner_username]
		RequestPunchFromReceiverResponseMap.Unlock()
		if ok {
			RequestPunchFromReceiverResponseMap.Lock()
			delete(RequestPunchFromReceiverResponseMap.Map, workspace_owner_username)
			RequestPunchFromReceiverResponseMap.Unlock()
			break
		}

		if count == 6 {
			invalid_flag = true
			break
		}
		count += 1
	}

	if invalid_flag {
		logger.LOGGER.Println("Error: Workspace Owner isn't Responding\nSource: connectToAnotherUser()")
		return "", "", nil, nil, errors.New("workspace owner isn't responding")
	}

	if req_punch_from_receiver_response.Error != "" {
		logger.LOGGER.Println("Error Received from Server's WS:", req_punch_from_receiver_response.Error)
		logger.LOGGER.Println("Description: Could Not Request Punch From Receiver")
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		return "", "", nil, nil, errors.New(req_punch_from_receiver_response.Error)
	}

	// Creating UDP Conn to Perform UDP NAT Hole Punching
	udp_conn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: local_port,
		IP:   net.IPv4zero, // or nil
	})
	if err != nil {
		logger.LOGGER.Printf("Error while Listening to %d: %v\n", local_port, err)
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		return "", "", nil, nil, err
	}

	logger.LOGGER.Println("Starting UDP NAT Hole Punching ...")
	var workspace_owner_ip, client_handler_name string
	if req_punch_from_receiver_response.WorkspaceOwnerPublicIp == my_public_IP_only {
		logger.LOGGER.Println("Sending Request via Private IP ...")
		workspace_owner_ip = req_punch_from_receiver_response.WorkspaceOwnerPrivateIp + ":" + req_punch_from_receiver_response.WorkspaceOwnerPrivatePort
	} else {
		logger.LOGGER.Println("Sending Request via Public IP ...")
		workspace_owner_ip = req_punch_from_receiver_response.WorkspaceOwnerPublicIp + ":" + req_punch_from_receiver_response.WorkspaceOwnerPublicPort
	}

	client_handler_name, err = dialer.WorkspaceListenerUdpNatHolePunching(udp_conn, workspace_owner_ip)
	if err != nil {
		logger.LOGGER.Println("Error while UDP NAT Hole Punching:", err)
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		udp_conn.Close()
		return "", "", nil, nil, err
	}
	logger.LOGGER.Println("UDP NAT Hole Punching Completed Successfully")

	// Creating KCP-Conn, KCP = Reliable UDP
	kcp_conn, err := kcp.DialWithConnAndOptions(workspace_owner_ip, nil, 0, 0, udp_conn)
	if err != nil {
		logger.LOGGER.Println("Error while Dialing KCP Connection to Remote Addr:", err)
		logger.LOGGER.Println("Source: connectToAnotherUser()")
		return "", "", nil, nil, err
	}

	// KCP Params for Congestion Control
	kcp_conn.SetWindowSize(128, 1024)
	kcp_conn.SetNoDelay(1, 10, 2, 1)
	kcp_conn.SetACKNoDelay(true)
	kcp_conn.SetDSCP(46)

	return client_handler_name, workspace_owner_ip, udp_conn, kcp_conn, nil
}

func fetchAndStoreDataIntoWorkspace(workspace_owner_ip, workspace_name string, udp_conn *net.UDPConn, res models.GetMetaDataResponse) error {
	// Decrypting AES Key
	key, err := encrypt.RSADecryptData(string(res.KeyBytes))
	if err != nil {
		logger.LOGGER.Println("Error while Decrypting Key:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	// Decrypting AES IV
	iv, err := encrypt.RSADecryptData(string(res.IVBytes))
	if err != nil {
		logger.LOGGER.Println("Error while Decrypting 'IV':", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	workspace_path, err := config.GetGetWorkspaceFilePath(workspace_name)
	if err != nil {
		logger.LOGGER.Println("Error while Fetching Workspace Path from Config:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}
	logger.LOGGER.Println("Workspace Path: ", workspace_path)

	zip_file_path := filepath.Join(workspace_path, ".PKr", "Contents", res.RequestPushRange+".zip")
	// Create Zip File
	zip_file_obj, err := os.Create(zip_file_path)
	if err != nil {
		logger.LOGGER.Println("Failed to Open & Create Zipped File:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}
	defer zip_file_obj.Close()

	// To Write Decrypted Data in Chunks
	writer := bufio.NewWriter(zip_file_obj)

	// Now Transfer Data using KCP ONLY, No RPC in chunks
	logger.LOGGER.Println("Connecting Again to Workspace Owner")
	kcp_conn, err := kcp.DialWithConnAndOptions(workspace_owner_ip, nil, 0, 0, udp_conn)
	if err != nil {
		logger.LOGGER.Println("Error while Dialing Workspace Owner to Get Data:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}
	defer kcp_conn.Close()
	logger.LOGGER.Println("Connected Successfully to Workspace Owner")

	// KCP Params for Congestion Control
	kcp_conn.SetWindowSize(128, 1024)
	kcp_conn.SetNoDelay(1, 10, 2, 1)
	kcp_conn.SetACKNoDelay(true)
	kcp_conn.SetDSCP(46)

	// Sending the Type of Session
	kpc_buff := [3]byte{'K', 'C', 'P'}
	_, err = kcp_conn.Write(kpc_buff[:])
	if err != nil {
		logger.LOGGER.Println("Error while Writing the type of Session(KCP-RPC or KCP-Plain):", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	logger.LOGGER.Println("Sending Workspace Name, Push Num & Data Request Type(Pull/Clone) to Workspace Owner")
	// Sending Workspace Name & Push Num
	_, err = kcp_conn.Write([]byte(workspace_name))
	if err != nil {
		logger.LOGGER.Println("Error while Sending Workspace Name to Workspace Owner:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	_, err = kcp_conn.Write([]byte(res.RequestPushRange))
	if err != nil {
		logger.LOGGER.Println("Error while Sending Workspace Name to Workspace Owner:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}
	logger.LOGGER.Println("Workspace Name, Push Num & Data Request Type(Pull/Clone) Sent to Workspace Owner")

	_, err = kcp_conn.Write([]byte("Pull"))
	if err != nil {
		logger.LOGGER.Println("Error while Sending 'Pull' to Workspace Owner:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	buffer := make([]byte, DATA_CHUNK)

	logger.LOGGER.Println("Len Data Bytes:", res.LenData)
	offset := 0

	logger.LOGGER.Println("Now Reading Data from Workspace Owner ...")
	for offset < res.LenData {
		n, err := kcp_conn.Read(buffer)
		if err != nil {
			logger.LOGGER.Println("\nError while Reading from Workspace Owner:", err)
			logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
			return err
		}

		// Check for Errors on Workspace Owner's Side
		if n < 30 {
			msg := string(buffer[:n])
			if msg == "Incorrect Workspace Name/Push Num" || msg == "Internal Server Error" {
				logger.LOGGER.Println("\nError while Reading from Workspace on his/her side:", msg)
				logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
				return errors.New(msg)
			}
		}

		// Decrypt Data
		decrypted_data, err := encrypt.EncryptDecryptChunk(buffer[:n], []byte(key), []byte(iv))
		if err != nil {
			logger.LOGGER.Println("Error while Decrypting Chunk:", err)
			logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
			return err
		}

		// Store data in chunks using 'writer'
		_, err = writer.Write(decrypted_data)
		if err != nil {
			logger.LOGGER.Println("Error while Writing Decrypted Data in Chunks:", err)
			logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
			return err
		}

		// Flush buffer to disk after 'FLUSH_AFTER_EVERY_X_MB'
		if offset%handler.FLUSH_AFTER_EVERY_X_MB == 0 {
			err = writer.Flush()
			if err != nil {
				logger.LOGGER.Println("Error flushing 'writer' buffer:", err)
				logger.LOGGER.Println("Soure: fetchAndStoreDataIntoWorkspace()")
				return err
			}
		}

		offset += n
	}
	logger.LOGGER.Println("Data Transfer Completed ...")

	// Flush buffer to disk at end
	err = writer.Flush()
	if err != nil {
		logger.LOGGER.Println("Error flushing 'writer' buffer:", err)
		logger.LOGGER.Println("Soure: fetchAndStoreDataIntoWorkspace()")
		return err
	}
	zip_file_obj.Close()

	_, err = kcp_conn.Write([]byte("Data Received"))
	if err != nil {
		logger.LOGGER.Println("Error while Sending Data Received Message:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		// Not Returning Error because, we got data, we don't care if workspace owner now is offline or not responding
	}

	unzip_dest := filepath.Join(workspace_path, ".PKr", "Contents", res.RequestPushRange)
	err = os.MkdirAll(unzip_dest, 0700)
	if err != nil {
		logger.LOGGER.Println("Error while Creating .PKr/Push Num Directory:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	// Unzip Content
	if err = filetracker.UnzipData(zip_file_path, unzip_dest); err != nil {
		logger.LOGGER.Println("Error while Unzipping Data into Workspace:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	err = filetracker.UpdateFilesFromWorkspace(workspace_path, unzip_dest, res.Updates)
	if err != nil {
		logger.LOGGER.Println("Error while Updating Files From Workspace:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	// Remove Zip File After Unzipping it
	err = os.Remove(zip_file_path)
	if err != nil {
		logger.LOGGER.Println("Error while Removing the Zip File After Use:", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}

	// Remove files from the place where changes were temporarily un-zipped
	err = os.RemoveAll(unzip_dest)
	if err != nil {
		logger.LOGGER.Println("Error while Removing the Files from '.PKr/Push Num/':", err)
		logger.LOGGER.Println("Source: fetchAndStoreDataIntoWorkspace()")
		return err
	}
	return nil
}

func PullWorkspace(workspace_owner_username, workspace_name string, conn *websocket.Conn) error {
	logger.LOGGER.Println("Pulling Workspace:", workspace_name)
	logger.LOGGER.Println("Workspace Owner:", workspace_owner_username)

	client_handler_name, workspace_owner_ip, udp_conn, kcp_conn, err := connectToAnotherUser(workspace_owner_username, conn)
	if err != nil {
		logger.LOGGER.Println("Error while Connecting to Another User:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}
	defer udp_conn.Close()
	defer kcp_conn.Close()

	rpc_buff := [3]byte{'R', 'P', 'C'}
	_, err = kcp_conn.Write(rpc_buff[:])
	if err != nil {
		logger.LOGGER.Println("Error while Writing the type of Session(KCP-RPC or KCP-Plain):", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}

	user_conf, err := config.ReadFromUserConfigFile()
	if err != nil {
		logger.LOGGER.Println("Error while Reading User Config File:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}

	var workspace_password string
	var last_push_num int
	for _, workspace := range user_conf.GetWorkspaces {
		if workspace.WorkspaceName == workspace_name && workspace.WorkspaceOwnerName == workspace_owner_username {
			workspace_password = workspace.WorkspacePassword
			last_push_num = workspace.LastPushNum
		}
	}

	// Creating RPC Client
	rpc_client := rpc.NewClient(kcp_conn)
	defer rpc_client.Close()
	rpcClientHandler := dialer.ClientCallHandler{}

	// Get Public Key of Workspace Owner
	logger.LOGGER.Println("Fetching Public Key of Workspace Owner from Config")
	public_key, err := config.GetPublicKeyUsingUsername(workspace_owner_username)
	if err != nil {
		logger.LOGGER.Println("Error while Getting Public Key of Workspace Owner:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}

	// Encrypting Workspace Password with Public Key
	encrypted_password, err := encrypt.RSAEncryptData(workspace_password, string(public_key))
	if err != nil {
		logger.LOGGER.Println("Error while Encrypting Workspace Password via Public Key:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}

	logger.LOGGER.Println("Calling GetMetaData ...")
	// Calling GetMetaData
	res, err := rpcClientHandler.CallGetMetaData(MY_USERNAME, MY_SERVER_IP, workspace_name, encrypted_password, client_handler_name, last_push_num, rpc_client)
	if err != nil {
		if err == handler.ErrUserAlreadyHasLatestWorkspace {
			return nil
		}
		logger.LOGGER.Println("Error while Calling GetMetaData:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}
	logger.LOGGER.Println("Get Data Responded, now storing files into workspace")
	logger.LOGGER.Println("Len Data:", res.LenData)
	logger.LOGGER.Println("Request Push Range:", res.RequestPushRange)
	logger.LOGGER.Println("Last Push Num:", res.LastPushNum)
	logger.LOGGER.Println("Last Push Desc:", res.LastPushDesc)

	kcp_conn.Close()
	rpc_client.Close()

	err = fetchAndStoreDataIntoWorkspace(workspace_owner_ip, workspace_name, udp_conn, *res)
	if err != nil {
		logger.LOGGER.Println("Error while Fetching Data & Storing it in Workspace:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}

	// Update user-config.json
	err = config.UpdateLastPushNumInGetWorkspaceFolderToUserConfig(workspace_name, res.LastPushNum)
	if err != nil {
		logger.LOGGER.Println("Error while Registering New GetWorkspace:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		return err
	}

	// Send Notification about new changes're fetched
	noti_msg := fmt.Sprintf("New Updates of Workspace: %s from User: %s're Fetched!", workspace_name, workspace_owner_username)
	err = beeep.Notify("Picker", noti_msg, "")
	if err != nil {
		logger.LOGGER.Println("Error while Sending Push Notification:", err)
		logger.LOGGER.Println("Source: pullWorkspace()")
		// Not Return Error, else it'll pull workspace again after sometime
	}

	logger.LOGGER.Println("Pull Done")
	return nil
}
