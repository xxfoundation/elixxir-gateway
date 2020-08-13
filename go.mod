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
	gitlab.com/elixxir/comms v0.0.0-20200811195348-7b218ca6d8c5
	gitlab.com/elixxir/crypto v0.0.0-20200811195343-de268a55c7c4
	gitlab.com/elixxir/primitives v0.0.0-20200805174810-86b366d1dd2d
	gitlab.com/xx_network/comms v0.0.0-20200811195337-736c2a8e60c6
	gitlab.com/xx_network/crypto v0.0.0-20200806235322-ede3c15881ce
	gitlab.com/xx_network/primitives v0.0.0-20200804183002-f99f7a7284da
	google.golang.org/grpc v1.31.0
	gopkg.in/ini.v1 v1.55.0 // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.1
