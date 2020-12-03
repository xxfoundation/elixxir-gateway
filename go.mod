module gitlab.com/elixxir/gateway

go 1.13

require (
	github.com/golang/protobuf v1.4.3
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
	gitlab.com/elixxir/bloomfilter v0.0.0-20200930191214-10e9ac31b228
	gitlab.com/elixxir/comms v0.0.4-0.20201201195523-61ba033e1daa
	gitlab.com/elixxir/crypto v0.0.6
	gitlab.com/elixxir/primitives v0.0.3-0.20201116174806-97f190989704
	gitlab.com/xx_network/comms v0.0.4-0.20201119231004-a67d08045535
	gitlab.com/xx_network/crypto v0.0.5-0.20201124194022-366c10b1bce0
	gitlab.com/xx_network/primitives v0.0.3-0.20201116234927-44e42fc91e7c
	gitlab.com/xx_network/ring v0.0.3-0.20201120004140-b0e268db06d1 // indirect
	gopkg.in/ini.v1 v1.55.0 // indirect
)

replace google.golang.org/grpc => github.com/grpc/grpc-go v1.27.1
