grpc-out:
	protoc ./proto/*.proto --go_out=. --go-grpc_out=.

get-new-kcp:
	go get github.com/PKr-Parivar/kcp-go@latest

generate-icon:
	go install github.com/akavel/rsrc@latest
	rsrc -ico .\PKrBase.ico -o PKrBase.syso

generate-exe-with-no-terminal:
	go build -ldflags -H=windowsgui -o NoTerminal.exe
