package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PKr-Parivar/PKr-Base/utils"
)

var WORKSPACE_CONFIG_FILE_PATH = filepath.Join(".PKr", "workspace-config.json")

func CreatePKRConfigIfNotExits(workspace_name string, workspace_path string) error {
	_, err := os.Stat(WORKSPACE_CONFIG_FILE_PATH)
	if err == nil {
		fmt.Println("It Seems Workspace is Already Intialized ...")
		return nil
	} else if os.IsNotExist(err) {
		fmt.Println("Creating workspace-config.json ...")
	} else {
		fmt.Println("Error while checking Existence of workspace-config file:", err)
		fmt.Println("Source: CreatePKRConfigIfNotExits()")
		return err
	}

	// Creating WORKSPACE_PATH/.PKr/Files/Current
	current_folder_path := filepath.Join(workspace_path, ".PKr", "Files", "Current")
	err = os.MkdirAll(current_folder_path, 0700)
	if err != nil {
		fmt.Printf("Error while Creating '%s' Dir: %v\n", current_folder_path, err)
		fmt.Println("Source: CreatePKRConfigIfNotExits()")
		return err
	}

	// Creating WORKSPACE_PATH/.PKr/Files/Changes
	changes_folder_path := filepath.Join(workspace_path, ".PKr", "Files", "Changes")
	err = os.MkdirAll(changes_folder_path, 0700)
	if err != nil {
		fmt.Printf("Error while Creating '%s' Dir: %v\n", changes_folder_path, err)
		fmt.Println("Source: CreatePKRConfigIfNotExits()")
		return err
	}

	pkr_config_file_path := filepath.Join(workspace_path, WORKSPACE_CONFIG_FILE_PATH)

	workspace_conf := PKRConfig{WorkspaceName: workspace_name}
	conf_bytes, err := json.Marshal(workspace_conf)
	if err != nil {
		fmt.Println("Error while Parsing workspace-config:", err)
		fmt.Println("Source: CreatePKRConfigIfNotExits()")
		return err
	}

	// Creating Workspace Config File ...
	err = os.WriteFile(pkr_config_file_path, conf_bytes, 0700)
	if err != nil {
		fmt.Println("Error while Writing in workspace-config:", err)
		fmt.Println("Source: CreatePKRConfigIfNotExits()")
		return err
	}
	return nil
}

func StorePublicKeyOfOtherUser(username string, public_key_of_other_user []byte) error {
	other_keys_path, err := utils.GetOthersKeysPath()
	if err != nil {
		fmt.Println("Error while Getting Other Keys Path:", err)
		fmt.Println("Source: StorePublicKeyOfOtherUser()")
		return err
	}

	key_path := filepath.Join(other_keys_path, username+".pem")
	err = os.WriteFile(key_path, public_key_of_other_user, 0700)
	if err != nil {
		fmt.Println("Error while Storing Public Key of Other User:", err)
		fmt.Println("Source: StorePublicKeyOfOtherUser()")
		return err
	}
	return nil
}

func ReadFromWorkspaceConfigFile(workspace_config_path string) (PKRConfig, error) {
	file, err := os.Open(workspace_config_path)
	if err != nil {
		fmt.Println("Error while opening workspace-config file:", err)
		fmt.Println("Source: ReadFromWorkspaceConfigFile()")
		return PKRConfig{}, err
	}
	defer file.Close()

	var pkrConfig PKRConfig
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&pkrConfig)
	if err != nil {
		fmt.Println("Error while Decoding JSON Data from workspace-config file:", err)
		fmt.Println("Source: ReadFromWorkspaceConfigFile()")
		return PKRConfig{}, err
	}

	return pkrConfig, nil
}

func writeToWorkspaceConfigFile(workspace_config_path string, newPKRConfing PKRConfig) error {
	jsonData, err := json.MarshalIndent(newPKRConfing, "", "	")
	if err != nil {
		fmt.Println("Error while Marshalling the workspace-config to JSON:", err)
		fmt.Println("Source: writeToWorkspaceConfigFile()")
		return err
	}

	err = os.WriteFile(workspace_config_path, jsonData, 0700)
	if err != nil {
		fmt.Println("Error while writing data in workspace-config file", err)
		fmt.Println("Source: writeToWorkspaceConfigFile()")
		return err
	}
	return nil
}

func UpdateLastPushNum(workspace_name string, last_push_num int) error {
	workspace_path, err := GetSendWorkspaceFilePath(workspace_name)
	if err != nil {
		fmt.Println("Error while Fetching File Path of 'Send Workspace':", err)
		fmt.Println("Source: UpdateLastPushNum()")
		return err
	}

	workspace_path = filepath.Join(workspace_path, WORKSPACE_CONFIG_FILE_PATH)
	workspace_json, err := ReadFromWorkspaceConfigFile(workspace_path)
	if err != nil {
		fmt.Println("Error while Reading from workspace-config:", err)
		fmt.Println("Source: UpdateLastPushNum()")
	}

	workspace_json.LastPushNum = last_push_num
	if err := writeToWorkspaceConfigFile(workspace_path, workspace_json); err != nil {
		fmt.Println("Error while Writing in workspace-config:", err)
		fmt.Println("Source: UpdateLastPushNum()")
		return err
	}
	return nil
}

func ReadMyPublicKey() ([]byte, error) {
	my_keys_path, err := utils.GetMyKeysPath()
	if err != nil {
		fmt.Println("Error while Getting My Keys Path:", err)
		fmt.Println("Source: ReadMyPublicKey()")
		return nil, err
	}

	public_key_bytes, err := os.ReadFile(filepath.Join(my_keys_path, "public.pem"))
	if err != nil {
		fmt.Println("Error while Reading My Public Key:", err)
		fmt.Println("Source: ReadMyPublicKey()")
		return nil, err
	}
	return public_key_bytes, nil
}

func AppendWorkspaceUpdates(updates Updates, workspace_path string) error {
	workspace_config_path := filepath.Join(workspace_path, WORKSPACE_CONFIG_FILE_PATH)
	workspace_json, err := ReadFromWorkspaceConfigFile(workspace_config_path)
	if err != nil {
		fmt.Println("Error while Reading from workspace-config:", err)
		fmt.Println("Source: AppendWorkspaceUpdates()")
		return err
	}

	workspace_json.AllUpdates = append(workspace_json.AllUpdates, updates)
	if err := writeToWorkspaceConfigFile(workspace_config_path, workspace_json); err != nil {
		fmt.Println("Error while Writing in workspace-config:", err)
		fmt.Println("Source: AppendWorkspaceUpdates()")
		return err
	}
	return nil
}

func MergeUpdates(workspace_path string, start_push_num, end_push_num int) ([]FileChange, error) {
	workspace_conf, err := ReadFromWorkspaceConfigFile(filepath.Join(workspace_path, WORKSPACE_CONFIG_FILE_PATH))
	if err != nil {
		fmt.Println("Error while Reading from workspace-config file:", err)
		fmt.Println("Source: MergeUpdates()")
		return nil, err
	}

	updates_list := make(map[string]FileChange)
	for i := start_push_num + 1; i <= end_push_num; i++ {
		for _, change := range workspace_conf.AllUpdates[i].Changes {
			update, exists := updates_list[change.FilePath]
			if exists {
				if update.Type == "Updated" && change.Type == "Removed" {
					delete(updates_list, change.FilePath)
				}
			} else {
				updates_list[change.FilePath] = FileChange{
					FilePath: change.FilePath,
					FileHash: change.FileHash,
					Type:     change.Type,
				}
			}
		}
	}

	merged_changes := []FileChange{}
	for _, hash_type := range updates_list {
		merged_changes = append(merged_changes, hash_type)
	}
	return merged_changes, nil
}
