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
	gitlab.com/elixxir/comms v0.0.0-20200804173225-11345b774364
	gitlab.com/elixxir/crypto v0.0.0-20200804172431-132b6336c177
	gitlab.com/elixxir/primitives v0.0.0-20200804170709-a1896d262cd9
	gitlab.com/xx_network/comms v0.0.0-20200804173440-47aa0850e752
	gitlab.com/xx_network/primitives v0.0.0-20200804174346-bfd30843a99b
	google.golang.org/grpc v1.30.0
	gopkg.in/ini.v1 v1.55.0 // indirect
)

replace (
	gitlab.com/xx_network/collections/ring => gitlab.com/xx_network/collections/ring.git v0.0.1
	google.golang.org/grpc => github.com/grpc/grpc-go v1.27.1
)
