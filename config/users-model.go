package config

type UserConfig struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	ServerIP       string `json:"server_ip"`
	ServerWSPort   int    `json:"server_ws_port"`
	ServergRPCPort int    `json:"server_grpc_port"`

	SendWorkspaces []SendWorkspaceFolder `json:"send_workspace"`
	GetWorkspaces  []GetWorkspaceFolder  `json:"get_workspace"`
}

type SendWorkspaceFolder struct {
	WorkspaceName     string `json:"workspace_name"`
	WorkspacePath     string `json:"workspace_path"`
	WorkSpacePassword string `json:"workspace_password"`
}

type GetWorkspaceFolder struct {
	WorkspaceOwnerName string `json:"workspace_owner_name"`
	WorkspaceName      string `json:"workspace_name"`
	WorkspacePassword  string `json:"workspace_password"`
	WorkspacePath      string `json:"workspace_path"`
	LastPushNum        int    `json:"last_push_num"`
}
