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
	"gitlab.com/elixxir/comms/publicAddress"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"net"
	"strconv"
	"time"
)

type Params struct {
	NodeAddress      string
	Port             int
	PublicAddress    string // Gateway's public IP address (with port)
	ListeningAddress string // Gateway's local IP address (with port)
	CertPath         string
	KeyPath          string

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

	DevMode      bool
	EnableGossip bool

	retentionPeriod time.Duration
	cleanupInterval time.Duration
}

const (
	// Default time period for keeping messages, rounds and bloom filters
	// alive in storage. Anything in storage older gets deleted
	retentionPeriodDefault = 24 * 7 * time.Hour

	// Default time period for checking storage for stored items older
	// than the retention period value
	cleanupIntervalDefault = 5 * time.Minute
)

func InitParams(vip *viper.Viper) Params {
	if !validConfig {
		jww.FATAL.Panicf("Invalid Config File: %s", cfgFile)
	}

	// Print all config options
	jww.INFO.Printf("All config params: %+v", vip.AllKeys())

	certPath = viper.GetString("certPath")
	if certPath == "" {
		jww.FATAL.Panicf("Gateway.yaml certPath is required, path provided is empty.")
	}

	idfPath = viper.GetString("idfPath")
	keyPath = viper.GetString("keyPath")
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

	// Get gateway's public IP or use the IP override
	overrideIP := viper.GetString("overridePublicIP")
	gwAddress, err := publicAddress.GetIpOverride(overrideIP, gwPort)
	if err != nil {
		jww.FATAL.Panicf("Failed to get public IP: %+v", err)
	}

	// Construct listening address
	listeningIP := viper.GetString("listeningAddress")
	if listeningIP == "" {
		listeningIP = "0.0.0.0"
	}
	listeningAddress := net.JoinHostPort(listeningIP, strconv.Itoa(gwPort))

	jww.INFO.Printf("config: %+v", viper.ConfigFileUsed())
	jww.INFO.Printf("Params: \n %+v", vip.AllSettings())
	jww.INFO.Printf("Gateway port: %d", gwPort)
	jww.INFO.Printf("Gateway public IP: %s", gwAddress)
	jww.INFO.Printf("Gateway listening address: %s", listeningAddress)
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

	// Time to keep messages, rounds and filters in storage
	viper.SetDefault("keepAlive", retentionPeriodDefault)
	retentionPeriod := viper.GetDuration("retentionPeriod")

	// Time to periodically check for old objects in storage
	viper.SetDefault("cleanupInterval", cleanupIntervalDefault)
	cleanupInterval := viper.GetDuration("cleanupInterval")

	// Obtain database connection info
	rawAddr := viper.GetString("dbAddress")
	var addr, port string
	if rawAddr != "" {
		addr, port, err = net.SplitHostPort(rawAddr)
		if err != nil {
			jww.FATAL.Panicf("Unable to get database port from %s: %+v", rawAddr, err)
		}
	}

	return Params{
		Port:                  gwPort,
		PublicAddress:         gwAddress,
		ListeningAddress:      listeningAddress,
		NodeAddress:           nodeAddress,
		CertPath:              certPath,
		KeyPath:               keyPath,
		ServerCertPath:        serverCertPath,
		IDFPath:               idfPath,
		PermissioningCertPath: permissioningCertPath,
		gossipFlags:           gossipFlags,
		rateLimitParams:       bucketMapParams,
		DbName:                viper.GetString("dbName"),
		DbUsername:            viper.GetString("dbUsername"),
		DbPassword:            viper.GetString("dbPassword"),
		DbAddress:             addr,
		DbPort:                port,
		DevMode:               viper.GetBool("devMode"),
		EnableGossip:          viper.GetBool("enableGossip"),
		retentionPeriod:       retentionPeriod,
		cleanupInterval:       cleanupInterval,
	}
}
