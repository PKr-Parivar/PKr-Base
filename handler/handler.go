package handler

import (
	"encoding/base64"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"os"

	"github.com/PKr-Parivar/PKr-Base/config"
	"github.com/PKr-Parivar/PKr-Base/encrypt"
	"github.com/PKr-Parivar/PKr-Base/filetracker"
	"github.com/PKr-Parivar/PKr-Base/logger"
	"github.com/PKr-Parivar/PKr-Base/models"
)

var (
	ErrIncorrectPassword             = errors.New("incorrect password")
	ErrServerNotFound                = errors.New("server not found in config")
	ErrInternalSeverError            = errors.New("internal server error")
	ErrUserAlreadyHasLatestWorkspace = errors.New("you already've latest version of workspace")
	ErrInvalidLastPushNum            = errors.New("invalid last push number")
	ErrNoSuchWorkspaceFound          = errors.New("no such workspace found")
)

type ClientHandler struct{}

func (h *ClientHandler) GetPublicKey(req models.PublicKeyRequest, res *models.PublicKeyResponse) error {
	logger.LOGGER.Println("Get Public Key Called ...")
	keyData, err := config.ReadMyPublicKey()
	if err != nil {
		logger.LOGGER.Println("Error while reading My Public Key from config:", err)
		logger.LOGGER.Println("Source: GetPublicKey()")
		return ErrInternalSeverError
	}

	res.PublicKey = []byte(keyData)
	logger.LOGGER.Println("Get Public Key Successful ...")
	return nil
}

func (h *ClientHandler) InitNewWorkSpaceConnection(req models.InitWorkspaceConnectionRequest, res *models.InitWorkspaceConnectionResponse) error {
	// 1. Decrypt password [X]
	// 2. Authenticate Request [X]
	// 3. Add the New Connection to the .PKr Config File [X]
	// 4. Store the Public Key [X]
	logger.LOGGER.Println("Init New Work Space Connection Called ...")

	password, err := encrypt.RSADecryptData(req.WorkspacePassword)
	if err != nil {
		logger.LOGGER.Println("Failed to Decrypt the Workspace Pass Received from Listener:", err)
		logger.LOGGER.Println("Source: InitNewWorkSpaceConnection()")
		return ErrInternalSeverError
	}

	_, err = config.AuthenticateWorkspaceInfo(req.WorkspaceName, password)
	if err != nil {
		if err.Error() == ErrIncorrectPassword.Error() {
			logger.LOGGER.Println("Error: Incorrect Credentials for Workspace")
			logger.LOGGER.Println("Source: InitNewWorkSpaceConnection()")
			return ErrIncorrectPassword
		}
		if err.Error() == ErrNoSuchWorkspaceFound.Error() {
			logger.LOGGER.Println("Error: No Such Workspace Found")
			logger.LOGGER.Println("Source: InitNewWorkSpaceConnection()")
			return ErrNoSuchWorkspaceFound
		}
		logger.LOGGER.Println("Failed to Authenticate Password of Listener:", err)
		logger.LOGGER.Println("Source: InitNewWorkSpaceConnection()")
		return ErrInternalSeverError
	}

	listener_public_key, err := base64.StdEncoding.DecodeString(string(req.MyPublicKey))
	if err != nil {
		logger.LOGGER.Println("Failed to Decode Public Key from base64:", err)
		logger.LOGGER.Println("Source: InitNewWorkSpaceConnection()")
		return ErrInternalSeverError
	}

	// Save Public Key
	err = config.StorePublicKeyOfOtherUser(req.MyUsername, listener_public_key)
	if err != nil {
		logger.LOGGER.Println("Failed to Store Public Keys at '.PKr\\keys':", err)
		logger.LOGGER.Println("Source: InitNewWorkSpaceConnection()")
		return ErrInternalSeverError
	}

	logger.LOGGER.Println("Init New Workspace Successful ...")
	return nil
}

func (h *ClientHandler) GetMetaData(req models.GetMetaDataRequest, res *models.GetMetaDataResponse) error {
	logger.LOGGER.Println("Get Meta Data Called ...")

	password, err := encrypt.RSADecryptData(req.WorkspacePassword)
	if err != nil {
		logger.LOGGER.Println("Failed to Decrypt the Workspace Pass Received from Listener:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	// Authenticates Workspace Name and Password and Get the Workspace File Path
	_, err = config.AuthenticateWorkspaceInfo(req.WorkspaceName, password)
	if err != nil {
		if errors.Is(err, ErrIncorrectPassword) {
			logger.LOGGER.Println("Error: Incorrect Credentials for Workspace")
			logger.LOGGER.Println("Source: GetMetaData()")
			return ErrIncorrectPassword
		}
		logger.LOGGER.Println("Failed to Authenticate Password of Listener:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrIncorrectPassword
	}
	logger.LOGGER.Printf("Data Requested For Workspace: %s\n", req.WorkspaceName)

	workspace_path, err := config.GetSendWorkspaceFilePath(req.WorkspaceName)
	if err != nil {
		logger.LOGGER.Println("Failed to Get Workspace Path from Config:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	// Reading Last Push Num from Config
	workspace_conf, err := config.ReadFromWorkspaceConfigFile(filepath.Join(workspace_path, ".PKr", "workspace-config.json"))
	if err != nil {
		logger.LOGGER.Println("Error while Reading from PKr Config File:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	logger.LOGGER.Println("Conf Last Push Num:", workspace_conf.LastPushNum)
	logger.LOGGER.Println("Req Last Push Num:", req.LastPushNum)
	if workspace_conf.LastPushNum == req.LastPushNum {
		logger.LOGGER.Println("User has the Latest Workspace, according to Last Push Num")
		logger.LOGGER.Println("No need to transfer data")
		return ErrUserAlreadyHasLatestWorkspace
	}

	if req.LastPushNum > workspace_conf.LastPushNum {
		logger.LOGGER.Println("User has Requested Invalid Last Push Num")
		return ErrInvalidLastPushNum
	}

	zip_destination_path := filepath.Join(workspace_path, ".PKr", "Files", "Current") + string(filepath.Separator)

	res.RequestPushRange = strconv.Itoa(workspace_conf.LastPushNum)
	res.Updates = nil

	// LastPushNum = -1 => Requesting for first time,i.e, Clone
	if req.LastPushNum == -1 {
		logger.LOGGER.Println("Clone")
		file_info, err := os.Stat(zip_destination_path + strconv.Itoa(workspace_conf.LastPushNum) + ".zip")
		if err != nil {
			logger.LOGGER.Println("Failed to Get FileInfo of Zip File:", err)
			logger.LOGGER.Println("Source: GetMetaData()")
			return ErrInternalSeverError
		}
		res.LenData = int(file_info.Size())
	} else {
		var zip_enc_filepath string
		res.Updates = map[string]string{}
		logger.LOGGER.Println("Pull")

		logger.LOGGER.Println("Merging Required Updates between the Pushes")
		merged_changes, err := config.MergeUpdates(workspace_path, req.LastPushNum, workspace_conf.LastPushNum)
		if err != nil {
			logger.LOGGER.Println("Unable to Merge Updates:", err)
			logger.LOGGER.Println("Source: GetMetaData()")
			return ErrInternalSeverError
		}

		logger.LOGGER.Println("Generating Changes Push Name ...")
		for _, changes := range merged_changes {
			res.Updates[changes.FilePath] = changes.Type
		}
		res.RequestPushRange = strconv.Itoa(req.LastPushNum) + "-" + strconv.Itoa(workspace_conf.LastPushNum)
		logger.LOGGER.Println("Request Push Range:", res.RequestPushRange)

		is_updates_cache_present, err := filetracker.AreUpdatesCached(workspace_path, res.RequestPushRange)
		if err != nil {
			logger.LOGGER.Println("Error while Checking Whether Updates're Already Cached or Not")
			logger.LOGGER.Println("Source: GetMetaData()")
			return ErrInternalSeverError
		}
		logger.LOGGER.Println("Is Update Cache Present:", is_updates_cache_present)

		if is_updates_cache_present {
			zip_destination_path = filepath.Join(workspace_path, ".PKr", "Files", "Changes", res.RequestPushRange) + string(filepath.Separator)
			zip_enc_filepath = zip_destination_path + res.RequestPushRange + ".enc"
		} else {
			logger.LOGGER.Println("Generating Changes Zip")
			last_push_num_str := strconv.Itoa(workspace_conf.LastPushNum)
			src_path := filepath.Join(workspace_path, ".PKr", "Files", "Current", last_push_num_str+".zip")
			dst_path := filepath.Join(workspace_path, ".PKr", "Files", "Changes", res.RequestPushRange, res.RequestPushRange+".zip")

			err = filetracker.ZipUpdates(merged_changes, src_path, dst_path)
			if err != nil {
				logger.LOGGER.Println("Error while Creating Zip for Changes:", err)
				logger.LOGGER.Println("Source: GetMetaData()")
				return ErrInternalSeverError
			}
			changes_path := filepath.Join(workspace_path, ".PKr", "Files", "Changes", res.RequestPushRange)
			logger.LOGGER.Println("Generating Keys for Changes File ...")

			changes_key, err := encrypt.AESGenerakeKey(16)
			if err != nil {
				logger.LOGGER.Println("Failed to Generate AES Keys:", err)
				logger.LOGGER.Println("Source: GetMetaData()")
				return ErrInternalSeverError
			}

			err = os.WriteFile(filepath.Join(changes_path, "AES_KEY"), changes_key, 0644)
			if err != nil {
				logger.LOGGER.Println("Failed to Write AES Key to File:", err)
				logger.LOGGER.Println("Source: GetMetaData()")
				return ErrInternalSeverError
			}

			changes_iv, err := encrypt.AESGenerateIV()
			if err != nil {
				logger.LOGGER.Println("Failed to Generate IV Keys:", err)
				logger.LOGGER.Println("Source: GetMetaData()")
				return ErrInternalSeverError
			}

			err = os.WriteFile(filepath.Join(changes_path, "AES_IV"), changes_iv, 0644)
			if err != nil {
				logger.LOGGER.Println("Failed to Write AES IV to File:", err)
				logger.LOGGER.Println("Source: GetMetaData()")
				return ErrInternalSeverError
			}

			changes_zipped_filepath := filepath.Join(changes_path, res.RequestPushRange+".zip")
			changes_enc_zip_filepath := strings.Replace(changes_zipped_filepath, ".zip", ".enc", 1)

			err = encrypt.EncryptZipFileAndStore(changes_zipped_filepath, changes_enc_zip_filepath, changes_key, changes_iv)
			if err != nil {
				logger.LOGGER.Println("Error while Encrypting Zip File of Entire Workspace, Storing it & Deleting Zip File:", err)
				logger.LOGGER.Println("Source: GetMetaData()")
				return ErrInternalSeverError
			}
			zip_destination_path = changes_path + string(filepath.Separator)
			zip_enc_filepath = zip_destination_path + res.RequestPushRange + ".enc"
		}
		file_info, err := os.Stat(zip_enc_filepath)
		if err != nil {
			logger.LOGGER.Println("Failed to Get FileInfo of Encrypted Zip File:", err)
			logger.LOGGER.Println("Source: GetMetaData()")
			return ErrInternalSeverError
		}
		res.LenData = int(file_info.Size())
	}

	key, err := os.ReadFile(zip_destination_path + "AES_KEY")
	if err != nil {
		logger.LOGGER.Println("Failed to Fetch AES Keys:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	iv, err := os.ReadFile(zip_destination_path + "AES_IV")
	if err != nil {
		logger.LOGGER.Println("Failed to Fetch IV Keys:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	logger.LOGGER.Println("Fetching Public Key of Listener from Config")
	public_key, err := config.GetPublicKeyUsingUsername(req.Username)
	if err != nil {
		logger.LOGGER.Println("Failed to Get Public Key of Listener Using Username:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	encrypt_key, err := encrypt.RSAEncryptData(string(key), string(public_key))
	if err != nil {
		logger.LOGGER.Println("Failed to Encrypt AES Keys using Listener's Public Key:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	encrypt_iv, err := encrypt.RSAEncryptData(string(iv), string(public_key))
	if err != nil {
		logger.LOGGER.Println("Failed to Encrypt IV Keys using Listener's Public Key:", err)
		logger.LOGGER.Println("Source: GetMetaData()")
		return ErrInternalSeverError
	}

	res.KeyBytes = []byte(encrypt_key)
	res.IVBytes = []byte(encrypt_iv)

	res.LastPushNum = workspace_conf.LastPushNum
	res.LastPushDesc = workspace_conf.AllUpdates[workspace_conf.LastPushNum].PushDesc

	logger.LOGGER.Println("Len Data:", res.LenData)
	logger.LOGGER.Println("Request Push Range:", res.RequestPushRange)
	logger.LOGGER.Println("Last Push Num:", res.LastPushNum)
	logger.LOGGER.Println("Last Push Desc:", res.LastPushDesc)

	logger.LOGGER.Println("Get Meta Data Successful ...")
	return nil
}
