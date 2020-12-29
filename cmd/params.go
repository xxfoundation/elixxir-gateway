///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains Params-related functionality

package cmd

import (
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"net"
	"time"
)

type Params struct {
	NodeAddress string
	Port        int
	Address     string
	CertPath    string
	KeyPath     string

	DbUsername string
	DbPassword string
	DbName     string
	DbAddress  string
	DbPort     string

	ServerCertPath        string
	IDFPath               string
	PermissioningCertPath string

	rateLimitParams *rateLimiting.MapParams
	gossipFlags     gossip.ManagerFlags
	MessageTimeout  time.Duration

	knownRoundsPath  string
	lastUpdateIdPath string

	DevMode      bool
	EnableGossip bool
}

func InitParams(vip *viper.Viper) Params {
	if !validConfig {
		jww.FATAL.Panicf("Invalid Config File: %s", cfgFile)
	}

	//print all config options
	jww.INFO.Printf("All config params: %+v", vip.AllKeys())

	certPath = viper.GetString("certPath")
	if certPath == "" {
		jww.FATAL.Panicf("Gateway.yaml certPath is required, path provided is empty.")
	}

	idfPath = viper.GetString("idfPath")
	keyPath = viper.GetString("keyPath")
	addressOverride := viper.GetString("addressOverride")
	messageTimeout = viper.GetDuration("messageTimeout")
	nodeAddress := viper.GetString("nodeAddress")
	if nodeAddress == "" {
		jww.FATAL.Panicf("Gateway.yaml nodeAddress is required, address provided is empty.")
	}
	permissioningCertPath = viper.GetString("permissioningCertPath")
	if permissioningCertPath == "" {
		jww.FATAL.Panicf("Gateway.yaml permissioningCertPath is required, path provided is empty.")
	}
	gwPort = viper.GetInt("port")
	if gwPort == 0 {
		jww.FATAL.Panicf("Gateway.yaml port is required, provided port is empty/not set.")
	}
	serverCertPath = viper.GetString("serverCertPath")
	if serverCertPath == "" {
		jww.FATAL.Panicf("Gateway.yaml serverCertPath is required, path provided is empty.")
	}

	jww.INFO.Printf("config: %+v", viper.ConfigFileUsed())
	jww.INFO.Printf("Params: \n %+v", vip.AllSettings())
	jww.INFO.Printf("Gateway port: %d", gwPort)
	jww.INFO.Printf("Gateway address override: %s", addressOverride)
	jww.INFO.Printf("Gateway node: %s", nodeAddress)

	// If the values aren't default, repopulate flag values with customized values
	// Otherwise use the default values
	gossipFlags := gossip.DefaultManagerFlags()
	if gossipFlags.BufferExpirationTime != bufferExpiration ||
		gossipFlags.MonitorThreadFrequency != monitorThreadFrequency {

		gossipFlags = gossip.ManagerFlags{
			BufferExpirationTime:   bufferExpiration,
			MonitorThreadFrequency: monitorThreadFrequency,
		}
	}

	// Construct the rate limiting params
	bucketMapParams := &rateLimiting.MapParams{
		Capacity:     capacity,
		LeakedTokens: leakedTokens,
		LeakDuration: leakDuration,
		PollDuration: pollDuration,
		BucketMaxAge: bucketMaxAge,
	}

	viper.SetDefault("knownRoundsPath", knownRoundsDefaultPath)
	krPath := viper.GetString("knownRoundsPath")

	viper.SetDefault("lastUpdateIdPath", lastUpdateIdDefaultPath)
	lastUpdateIdPath := viper.GetString("lastUpdateIdPath")

	// Obtain database connection info
	rawAddr := viper.GetString("dbAddress")
	var addr, port string
	var err error
	if rawAddr != "" {
		addr, port, err = net.SplitHostPort(rawAddr)
		if err != nil {
			jww.FATAL.Panicf("Unable to get database port from %s: %+v", rawAddr, err)
		}
	}

	return Params{
		Port:                  gwPort,
		Address:               addressOverride,
		NodeAddress:           nodeAddress,
		CertPath:              certPath,
		KeyPath:               keyPath,
		ServerCertPath:        serverCertPath,
		IDFPath:               idfPath,
		PermissioningCertPath: permissioningCertPath,
		gossipFlags:           gossipFlags,
		rateLimitParams:       bucketMapParams,
		MessageTimeout:        messageTimeout,
		knownRoundsPath:       krPath,
		DbName:                viper.GetString("dbName"),
		DbUsername:            viper.GetString("dbUsername"),
		DbPassword:            viper.GetString("dbPassword"),
		DbAddress:             addr,
		DbPort:                port,
		lastUpdateIdPath:      lastUpdateIdPath,
		DevMode:               viper.GetBool("devMode"),
		EnableGossip:          viper.GetBool("enableGossip"),
	}
}
