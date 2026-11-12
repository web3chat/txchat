module github.com/web3chat/txchat/imparse

go 1.23.0

toolchain go1.24.5

require (
	github.com/golang/protobuf v1.5.4
	github.com/web3chat/txchat/im v0.1.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/Terry-Mao/goim v0.0.0-20210523140626-e742c99ad76e // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c // indirect
	google.golang.org/grpc v1.40.0 // indirect
)

// 同仓库多 module 构建：使用本地目录，避免依赖未发布的版本。
replace (
	github.com/web3chat/txchat/dtalk => ../dtalk
	github.com/web3chat/txchat/im => ../im
	github.com/web3chat/txchat/imparse => ./
)
