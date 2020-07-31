module gitlab.com/elixxir/gateway

go 1.13

require (
	github.com/golang/protobuf v1.4.2
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/jinzhu/gorm v1.9.15
	github.com/mitchellh/mapstructure v1.3.0 // indirect
	github.com/pelletier/go-toml v1.7.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/smartystreets/assertions v1.1.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.6.3
	gitlab.com/elixxir/comms v0.0.0-20200731202002-0d5c9fa8eefe
	gitlab.com/elixxir/crypto v0.0.0-20200721213839-b026955c55c0
	gitlab.com/elixxir/primitives v0.0.0-20200731184040-494269b53b4d
	gitlab.com/xx_network/collections/ring v0.0.0-00010101000000-000000000000 // indirect
	gitlab.com/xx_network/comms v0.0.0-20200730220144-eea32e8b696d
	google.golang.org/grpc v1.30.0
	gopkg.in/ini.v1 v1.55.0 // indirect
)

replace (
	gitlab.com/xx_network/collections/ring => gitlab.com/xx_network/collections/ring.git v0.0.1
	google.golang.org/grpc => github.com/grpc/grpc-go v1.27.1
)
