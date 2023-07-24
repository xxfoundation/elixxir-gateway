.PHONY: update master release update_master update_release build clean version

version:
	go run main.go generate
	mv version_vars.go cmd/version_vars.go

clean:
	go mod tidy
	go mod vendor

update:
	-GOFLAGS="" go get all

build:
	go build ./...

update_release:
	GOFLAGS="" go get gitlab.com/xx_network/primitives@XX-4707/tagDiskJson
	GOFLAGS="" go get gitlab.com/elixxir/primitives@XX-4707/tagDiskJson
	GOFLAGS="" go get gitlab.com/xx_network/crypto@XX-4707/tagDiskJson
	GOFLAGS="" go get gitlab.com/elixxir/crypto@XX-4707/tagDiskJson
	GOFLAGS="" go get gitlab.com/xx_network/comms@XX-4707/tagDiskJson
	GOFLAGS="" go get gitlab.com/elixxir/comms@XX-4707/tagDiskJson
	GOFLAGS="" go get gitlab.com/elixxir/bloomfilter@release

update_master:
	GOFLAGS="" go get gitlab.com/xx_network/primitives@master
	GOFLAGS="" go get gitlab.com/elixxir/primitives@master
	GOFLAGS="" go get gitlab.com/xx_network/crypto@master
	GOFLAGS="" go get gitlab.com/elixxir/crypto@master
	GOFLAGS="" go get gitlab.com/xx_network/comms@master
	GOFLAGS="" go get gitlab.com/elixxir/comms@master
	GOFLAGS="" go get gitlab.com/elixxir/bloomfilter@master

master: update_master clean build version

release: update_release clean build
