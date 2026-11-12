module github.com/web3chat/txchat/im

go 1.23.0

toolchain go1.24.5

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Terry-Mao/goim v0.0.0-20210523140626-e742c99ad76e
	github.com/gin-contrib/pprof v1.3.0
	github.com/gin-gonic/gin v1.10.1
	github.com/go-sql-driver/mysql v1.7.1
	github.com/golang/protobuf v1.5.4
	github.com/gomodule/redigo v2.0.0+incompatible
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.4.2
	github.com/mitchellh/mapstructure v1.2.2
	github.com/oofpgDLD/dtask v1.0.2
	github.com/opentracing/opentracing-go v1.2.0
	github.com/rs/zerolog v1.21.0
	github.com/txchat/im-pkg v0.0.1
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/web3chat/txchat/dtalk v0.1.0
	github.com/web3chat/txchat/imparse v0.1.0
	github.com/zhenjl/cityhash v0.0.0-20131128155616-cdd6a94144ab
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/Shopify/sarama.v1 v1.19.0
)

require (
	github.com/Shopify/sarama v1.19.0 // indirect
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/bytedance/sonic v1.13.3 // indirect
	github.com/bytedance/sonic/loader v0.2.4 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ethereum/go-ethereum v1.10.8 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.26.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.2.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/haltingstate/secp256k1-go v0.0.0-20151224084235-572209b26df6 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.0 // indirect
	golang.org/x/arch v0.18.0 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// 同仓库多 module 构建：使用本地目录，避免依赖未发布的版本。
replace (
	github.com/web3chat/txchat/dtalk => ../dtalk
	github.com/web3chat/txchat/im => ./
	github.com/web3chat/txchat/imparse => ../imparse
)
