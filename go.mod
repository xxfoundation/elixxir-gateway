module gitlab.com/elixxir/gateway

go 1.13

require (
	github.com/go-delve/delve v1.5.0 // indirect
	github.com/golang/protobuf v1.4.2
	github.com/jinzhu/gorm v1.9.15
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/smartystreets/assertions v1.1.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0
	github.com/spf13/viper v1.6.3
	gitlab.com/elixxir/client v1.5.0 // indirect
	gitlab.com/elixxir/comms v0.0.1
	gitlab.com/elixxir/crypto v0.0.1
	gitlab.com/elixxir/primitives v0.0.1
	gitlab.com/xx_network/comms v0.0.1
	gitlab.com/xx_network/crypto v0.0.1
	gitlab.com/xx_network/primitives v0.0.1
	gopkg.in/ini.v1 v1.55.0 // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.1
