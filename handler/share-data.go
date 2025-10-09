package handler

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/PKr-Parivar/PKr-Base/config"
	"github.com/PKr-Parivar/PKr-Base/encrypt"
	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/PKr-Parivar/kcp-go"
)

const DATA_CHUNK = encrypt.DATA_CHUNK
const FLUSH_AFTER_EVERY_X_MB = encrypt.FLUSH_AFTER_EVERY_X_MB

func sendErrorMessage(kcp_session *kcp.UDPSession, error_msg string) {
	_, err := kcp_session.Write([]byte(error_msg))
	if err != nil {
		logger.LOGGER.Println("Error while Sending Error Message:", err)
		logger.LOGGER.Println("Source: sendMessage()")
	}
}

func handleClone(kcp_session *kcp.UDPSession, zip_path string, len_data_bytes int, workspace_path string) {
	curr_dir := filepath.Join(workspace_path, ".PKr", "Files", "Current") + string(filepath.Separator)
	key, err := os.ReadFile(curr_dir + "AES_KEY")
	if err != nil {
		logger.LOGGER.Println("Error while Reading AES Key:", err)
		logger.LOGGER.Println("Source: handleClone()")
		return
	}

	iv, err := os.ReadFile(curr_dir + "AES_IV")
	if err != nil {
		logger.LOGGER.Println("Error while Reading AES IV:", err)
		logger.LOGGER.Println("Source: handleClone()")
		return
	}

	zip_file_obj, err := os.Open(zip_path)
	if err != nil {
		logger.LOGGER.Println("Error while Opening Destination File:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		sendErrorMessage(kcp_session, "Internal Server Error")
		return
	}
	defer zip_file_obj.Close()

	var buff [512]byte
	buffer := make([]byte, DATA_CHUNK)
	reader := bufio.NewReader(zip_file_obj)
	logger.LOGGER.Println("Length of File:", len_data_bytes)

	logger.LOGGER.Println("Preparing to Transfer Data for Clone")
	for {
		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				logger.LOGGER.Println("Done Sent, now waiting for ack from listener ...")
				n, err := kcp_session.Read(buff[:])
				if err != nil {
					logger.LOGGER.Println("Error while Reading 'Data Received' Message from Listener:", err)
					logger.LOGGER.Println("Source: GetDataHandler()")
					return
				}
				// Data Received
				msg := string(buff[:n])
				if msg == "Data Received" {
					logger.LOGGER.Println("Data Transfer Completed")
					return
				}
				logger.LOGGER.Println("Received Unexpected Message:", msg)
				return
			}
			logger.LOGGER.Println("Error while Sending Workspace Chunk:", err)
			logger.LOGGER.Println("Source: GetDataHandler()")
			sendErrorMessage(kcp_session, "Internal Server Error")
			return
		}

		if n > 0 {
			buffer, err = encrypt.EncryptDecryptChunk(buffer[:n], key, iv)
			if err != nil {
				logger.LOGGER.Println("Error while Encrypting Data Chunk ...:", err)
				logger.LOGGER.Println("Source: handleClone()")
				return
			}

			_, err := kcp_session.Write([]byte(buffer[:n]))
			if err != nil {
				logger.LOGGER.Println("Error while Sending Data:", err)
				logger.LOGGER.Println("Source: GetDataHandler()")
				sendErrorMessage(kcp_session, "Internal Server Error")
				return
			}
		}
	}
}

func GetDataHandler(kcp_session *kcp.UDPSession) {
	logger.LOGGER.Println("Get Data Handler Called ...")
	logger.LOGGER.Println("Reading Workspace Name ...")

	var buff [512]byte
	n, err := kcp_session.Read(buff[:])
	if err != nil {
		logger.LOGGER.Println("Error while Reading Workspace Name:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		return
	}
	workspace_name := string(buff[:n])
	logger.LOGGER.Println("Workspace Name:", workspace_name)
	logger.LOGGER.Println("Reading Workspace Push Num ...")

	n, err = kcp_session.Read(buff[:])
	if err != nil {
		logger.LOGGER.Println("Error while Reading Workspace Push Num:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		return
	}
	workspace_push_num := string(buff[:n])
	logger.LOGGER.Println("Workspace Push Num:", workspace_push_num)
	logger.LOGGER.Println("Reading Type of Data Request(Pull/Clone) ...")

	// Read Data Request Type (Pull/Clone)
	n, err = kcp_session.Read(buff[:])
	if err != nil {
		logger.LOGGER.Println("Error while Reading Type of Data Request Type:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		return
	}
	data_req_type := string(buff[:n])
	logger.LOGGER.Println("Data Request Type(Clone/Pull):", data_req_type)

	workspace_path, err := config.GetSendWorkspaceFilePath(workspace_name)
	if err != nil {
		logger.LOGGER.Println("Failed to Get Workspace Path from Config:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		sendErrorMessage(kcp_session, "Internal Server Error")
		return
	}
	logger.LOGGER.Println("Workspace Path:", workspace_path)

	if data_req_type == "Clone" {
		zip_path := filepath.Join(workspace_path, ".PKr", "Files", "Current", workspace_push_num+".zip")
		fileInfo, err := os.Stat(zip_path)
		if err == nil {
			logger.LOGGER.Println("Destination File Exists")
		} else if os.IsNotExist(err) {
			logger.LOGGER.Println("Destination File does not Exists")
			sendErrorMessage(kcp_session, "Incorrect Workspace Name/Push Num")
			return
		} else {
			logger.LOGGER.Println("Error while checking Existence of Destination file:", err)
			logger.LOGGER.Println("Source: GetDataHandler()")
			sendErrorMessage(kcp_session, "Internal Server Error")
			return
		}

		handleClone(kcp_session, zip_path, int(fileInfo.Size()), workspace_path)
		return
	} else if data_req_type != "Pull" {
		logger.LOGGER.Println("Invalid Data Request Type Sent from User")
		logger.LOGGER.Println("Source: GetDataHandler()")
		sendErrorMessage(kcp_session, "Invalid Data Request Type Sent")
		return
	}

	zip_enc_path := filepath.Join(workspace_path, ".PKr", "Files", "Changes", workspace_push_num, workspace_push_num+".enc")
	logger.LOGGER.Println("Zip Enc FilePath to share:", zip_enc_path)

	fileInfo, err := os.Stat(zip_enc_path)
	if err == nil {
		logger.LOGGER.Println("Destination File Exists")
	} else if os.IsNotExist(err) {
		logger.LOGGER.Println("Destination File does not Exists")
		sendErrorMessage(kcp_session, "Incorrect Workspace Name/Push Num Range")
		return
	} else {
		logger.LOGGER.Println("Error while checking Existence of Destination file:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		sendErrorMessage(kcp_session, "Internal Server Error")
		return
	}

	zip_file_obj, err := os.Open(zip_enc_path)
	if err != nil {
		logger.LOGGER.Println("Error while Opening Destination File:", err)
		logger.LOGGER.Println("Source: GetDataHandler()")
		sendErrorMessage(kcp_session, "Internal Server Error")
		return
	}
	defer zip_file_obj.Close()

	buffer := make([]byte, DATA_CHUNK)
	reader := bufio.NewReader(zip_file_obj)

	len_data_bytes := int(fileInfo.Size())
	logger.LOGGER.Println("Length of File:", len_data_bytes)

	logger.LOGGER.Println("Preparing to Transfer Data for Pull")
	for {
		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				logger.LOGGER.Println("Done Sent, now waiting for ack from listener ...")
				n, err := kcp_session.Read(buff[:])
				if err != nil {
					logger.LOGGER.Println("Error while Reading 'Data Received' Message from Listener:", err)
					logger.LOGGER.Println("Source: GetDataHandler()")
					return
				}
				// Data Received
				msg := string(buff[:n])
				if msg == "Data Received" {
					logger.LOGGER.Println("Data Transfer Completed")
					return
				}
				logger.LOGGER.Println("Received Unexpected Message:", msg)
				return
			}
			logger.LOGGER.Println("Error while Sending Workspace Chunk:", err)
			logger.LOGGER.Println("Source: GetDataHandler()")
			sendErrorMessage(kcp_session, "Internal Server Error")
			return
		}

		if n > 0 {
			_, err := kcp_session.Write([]byte(buffer[:n]))
			if err != nil {
				logger.LOGGER.Println("Error while Sending Data:", err)
				logger.LOGGER.Println("Source: GetDataHandler()")
				sendErrorMessage(kcp_session, "Internal Server Error")
				return
			}
		}
	}
}
