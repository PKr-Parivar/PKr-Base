package ws

import (
	"encoding/json"
	"time"

	"github.com/PKr-Parivar/PKr-Base/config"
	"github.com/PKr-Parivar/PKr-Base/handler"
	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/PKr-Parivar/PKr-Base/models"

	"github.com/gorilla/websocket"
)

const (
	PONG_WAIT_TIME = 5 * time.Minute
	PING_WAIT_TIME = (PONG_WAIT_TIME * 9) / 10
)

var RequestPunchFromReceiverResponseMap = models.RequestPunchFromReceiverResponseMap{Map: map[string]models.RequestPunchFromReceiverResponse{}}

func handleNotifyToPunchRequest(conn *websocket.Conn, msg models.WSMessage) {
	msg_bytes, err := json.Marshal(msg.Message)
	if err != nil {
		logger.LOGGER.Println("Error while marshaling:", err)
		logger.LOGGER.Println("Source: handleNotifyToPunchRequest()")
		return
	}
	var noti_to_punch_req models.NotifyToPunchRequest
	if err := json.Unmarshal(msg_bytes, &noti_to_punch_req); err != nil {
		logger.LOGGER.Println("Error while unmarshaling:", err)
		logger.LOGGER.Println("Source: handleNotifyToPunchRequest()")
		return
	}

	my_public_ip, my_public_port, my_private_ip, my_private_port, err := handler.HandleNotifyToPunchRequest(noti_to_punch_req.ListenerPublicIp, noti_to_punch_req.ListenerPublicPort, noti_to_punch_req.ListenerPrivateIp, noti_to_punch_req.ListenerPrivatePort)
	if err != nil {
		logger.LOGGER.Println("Error while Handling NotifyToPunch:", err)
		logger.LOGGER.Println("Source: handleNotifyToPunchRequest()")
		return
	}

	noti_to_punch_res := models.NotifyToPunchResponse{
		WorkspaceOwnerPublicIp:    my_public_ip,
		WorkspaceOwnerPublicPort:  my_public_port,
		ListenerUsername:          noti_to_punch_req.ListenerUsername,
		WorkspaceOwnerPrivateIp:   my_private_ip,
		WorkspaceOwnerPrivatePort: my_private_port,
	}

	res := models.WSMessage{
		MessageType: "NotifyToPunchResponse",
		Message:     noti_to_punch_res,
	}

	err = conn.WriteJSON(res)
	if err != nil {
		logger.LOGGER.Println("Error while writing Response of Notify To Punch:", err)
		logger.LOGGER.Println("Source: handleNotifyToPunchRequest()")
		return
	}
}

func handleNotifyNewPushToListeners(msg models.WSMessage, conn *websocket.Conn) {
	msg_bytes, err := json.Marshal(msg.Message)
	if err != nil {
		logger.LOGGER.Println("Error while marshaling:", err)
		logger.LOGGER.Println("Source: handleNotifyNewPushToListeners()")
		return
	}
	var noti_new_push models.NotifyNewPushToListeners
	if err := json.Unmarshal(msg_bytes, &noti_new_push); err != nil {
		logger.LOGGER.Println("Error while unmarshaling:", err)
		logger.LOGGER.Println("Source: handleNotifyNewPushToListeners()")
		return
	}

	err = PullWorkspace(noti_new_push.WorkspaceOwnerUsername, noti_new_push.WorkspaceName, conn)
	if err != nil {
		if err.Error() == "workspace owner is offline" {
			logger.LOGGER.Println("Workspace Owner is Offline, Server'll notify when he's online")
			return
		}
		if err.Error() == "you already've latest version of workspace" {
			logger.LOGGER.Println("You've Lastest Version of Workspace, No Need to Transfer Data")
			return
		}
		logger.LOGGER.Println("Error while Pulling Data:", err)
		logger.LOGGER.Println("Source: handleNotifyNewPushToListeners()")

		// Try Again only once after 5 minutes
		logger.LOGGER.Println("Will Try Again after 5 minutes")
		time.Sleep(5 * time.Minute)
		err = PullWorkspace(noti_new_push.WorkspaceOwnerUsername, noti_new_push.WorkspaceName, conn)
		if err != nil {
			logger.LOGGER.Println("Error while Pulling Data Again:", err)
			logger.LOGGER.Println("Source: handleNotifyNewPushToListeners()")
		}
	}
}

func handleRequestPunchFromReceiverResponse(msg models.WSMessage) {
	msg_bytes, err := json.Marshal(msg.Message)
	if err != nil {
		logger.LOGGER.Println("Error while marshaling:", err)
		logger.LOGGER.Println("Source: handleNotifyToPunchResponse()")
		return
	}
	var msg_obj models.RequestPunchFromReceiverResponse
	if err := json.Unmarshal(msg_bytes, &msg_obj); err != nil {
		logger.LOGGER.Println("Error while unmarshaling:", err)
		logger.LOGGER.Println("Source: handleNotifyToPunchResponse()")
		return
	}
	RequestPunchFromReceiverResponseMap.Lock()
	RequestPunchFromReceiverResponseMap.Map[msg_obj.WorkspaceOwnerUsername] = msg_obj
	RequestPunchFromReceiverResponseMap.Unlock()
}

func handleWorkspaceOwnerIsOnline(msg models.WSMessage, conn *websocket.Conn) {
	msg_bytes, err := json.Marshal(msg.Message)
	if err != nil {
		logger.LOGGER.Println("Error while marshaling:", err)
		logger.LOGGER.Println("Source: handleWorkspaceOwnerIsOnline()")
		return
	}

	var msg_obj models.WorkspaceOwnerIsOnline
	if err := json.Unmarshal(msg_bytes, &msg_obj); err != nil {
		logger.LOGGER.Println("Error while unmarshaling:", err)
		logger.LOGGER.Println("Source: handleWorkspaceOwnerIsOnline()")
		return
	}

	user_conf, err := config.ReadFromUserConfigFile()
	if err != nil {
		logger.LOGGER.Println("Error while Reading User Config File:", err)
		logger.LOGGER.Println("Source: handleWorkspaceOwnerIsOnline()")
		return
	}

	for _, workspace := range user_conf.GetWorkspaces {
		if workspace.WorkspaceOwnerName == msg_obj.WorkspaceOwnerName {
			err := PullWorkspace(msg_obj.WorkspaceOwnerName, workspace.WorkspaceName, conn)
			if err != nil {
				logger.LOGGER.Println("Error while Pulling Data:", err)
				logger.LOGGER.Println("Source: handleWorkspaceOwnerIsOnline()")
			}
		}
	}
}

func ReadJSONMessage(done chan struct{}, conn *websocket.Conn) {
	defer close(done)

	conn.SetReadDeadline(time.Now().Add(PONG_WAIT_TIME))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(PONG_WAIT_TIME))
		return nil
	})

	for {
		var msg models.WSMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			logger.LOGGER.Println("Read WebSocket Error from Server:", err)
			logger.LOGGER.Println("Source: ReadJSONMessage()")
			return
		}

		logger.LOGGER.Printf("Message Received from Server's WS: %#v\n", msg)

		switch msg.MessageType {
		case "NotifyToPunchRequest":
			logger.LOGGER.Println("NotifyToPunchRequest Called")
			go handleNotifyToPunchRequest(conn, msg)
		case "NotifyNewPushToListeners":
			logger.LOGGER.Println("NotifyNewPushToListeners Called")
			go handleNotifyNewPushToListeners(msg, conn)
		case "RequestPunchFromReceiverResponse":
			logger.LOGGER.Println("RequestPunchFromReceiverResponse Called")
			go handleRequestPunchFromReceiverResponse(msg)
		case "WorkspaceOwnerIsOnline":
			logger.LOGGER.Println("Workspace Owner is Online Called")
			go handleWorkspaceOwnerIsOnline(msg, conn)
		}
	}
}

func PingPongWriter(done chan struct{}, conn *websocket.Conn) {
	defer close(done)

	ticker := time.NewTicker(PING_WAIT_TIME)
	for {
		<-ticker.C
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			logger.LOGGER.Println("No response of Ping from Server")
			return
		}
	}
}
