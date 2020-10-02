///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger.
package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"gitlab.com/elixxir/primitives/utils"
	"gitlab.com/xx_network/comms/gossip"
	"net"
	"os"
	"strings"
	"time"
)

// Default path used to KnownRounds if one is not provided
const knownRoundsDefaultPath = "/opt/xxnetwork/gateway-logs/knownRounds.json"

// Flags to import from command line or config file
var (
	cfgFile, idfPath, logPath                                string
	certPath, keyPath, serverCertPath, permissioningCertPath string
	logLevel                                                 uint // 0 = info, 1 = debug, >1 = trace
	messageTimeout                                           time.Duration
	gwPort                                                   int
	validConfig                                              bool

	kr int

	// For gossip protocol
	bufferExpiration, monitorThreadFrequency time.Duration

	// For rate limiting
	capacity, leakedTokens                   uint32
	leakDuration, pollDuration, bucketMaxAge time.Duration
)

// RootCmd represents the base command when called without any sub-commands
var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Runs a cMix gateway",
	Long:  `The cMix gateways coordinate communications between servers and clients`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		initConfig()
		initLog()
		params := InitParams(viper.GetViper())

		// Obtain database connection info
		rawAddr := viper.GetString("dbAddress")
		var addr, port string
		if rawAddr != "" {
			addr, port, err = net.SplitHostPort(rawAddr)
			if err != nil {
				jww.FATAL.Panicf("Unable to get database port: %+v", err)
			}
		}

		// Attempt to initialize the backend storage
		storage.GatewayDB, _, err = storage.NewDatabase(
			viper.GetString("dbUsername"),
			viper.GetString("dbPassword"),
			viper.GetString("dbName"),
			addr,
			port,
		)
		if err != nil {
			jww.FATAL.Panicf("Unable to initialize storage: %+v", err)
		}

		// Build gateway implementation object
		gateway := NewGatewayInstance(params)

		// start gateway network interactions
		for {
			err := gateway.InitNetwork()
			if err == nil {
				break
			}
			errMsg := err.Error()
			tic := strings.Contains(errMsg, "transport is closing")
			cde := strings.Contains(errMsg, "DeadlineExceeded")
			if tic || cde {
				if gateway.Comms != nil {
					gateway.Comms.Shutdown()
				}

				jww.ERROR.Printf("Cannot connect to node, "+
					"retrying in 10s: %+v", err)
				time.Sleep(10 * time.Second)
				continue
			}
			jww.FATAL.Panicf(err.Error())
		}

		jww.INFO.Printf("Starting xx network gateway v%s", SEMVER)

		// Begin gateway persistent components
		gateway.StartPeersThread()
		gateway.Start()

		// Wait forever
		select {}
	},
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
	listeningAddress := viper.GetString("listeningAddress")
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
	jww.INFO.Printf("Gateway listen IP address: %s", listeningAddress)
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

	p := Params{
		Port:                  gwPort,
		Address:               listeningAddress,
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
	}

	return p
}

// Execute adds all child commands to the root command and sets flags
// appropriately.  This is called by main.main(). It only needs to
// happen once to the RootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		jww.ERROR.Println(err)
		os.Exit(1)
	}
}

// init is the initialization function for Cobra which defines commands
// and flags.
func init() {
	// NOTE: The point of init() is to be declarative.
	// There is one init in each sub command. Do not put variable declarations
	// here, and ensure all the Flags are of the *P variety, unless there's a
	// very good reason not to have them as local Params to sub command."

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "",
		"Path to load the Gateway configuration file from. If not set, this "+
			"file must be named gateway.yaml and must be located in "+
			"~/.xxnetwork/, /opt/xxnetwork, or /etc/xxnetwork.")

	rootCmd.Flags().IntP("port", "p", -1,
		"Port for Gateway to listen on. Gateway must be the only listener "+
			"on this port. Required field.")
	err := viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	handleBindingError(err, "port")

	rootCmd.Flags().StringVar(&idfPath, "idfPath", "./gateway-logs/gatewayIDF.json",
		"Path to where the IDF is saved. This is used by the wrapper management script.")
	err = viper.BindPFlag("idfPath", rootCmd.Flags().Lookup("idfPath"))
	handleBindingError(err, "idfPath")

	rootCmd.Flags().UintVarP(&logLevel, "logLevel", "l", 0,
		"Level of debugging to print (0 = info, 1 = debug, >1 = trace).")
	err = viper.BindPFlag("logLevel", rootCmd.Flags().Lookup("logLevel"))
	handleBindingError(err, "logLevel")

	rootCmd.Flags().StringVar(&logPath, "log", "./gateway-logs/gateway.log",
		"Path where log file will be saved.")
	err = viper.BindPFlag("log", rootCmd.Flags().Lookup("log"))
	handleBindingError(err, "log")

	rootCmd.Flags().DurationVar(&messageTimeout, "messageTimeout", 60*time.Second,
		"Period in which the message cleanup function executes. All users"+
			" who message buffer have exceeded the maximum size will get their"+
			" messages deleted. Recommended period is on the order of a minute to an hour.")
	err = viper.BindPFlag("messageTimeout", rootCmd.Flags().Lookup("messageTimeout"))
	handleBindingError(err, "messageTimeout")

	rootCmd.Flags().String("listeningAddress", "0.0.0.0",
		"Local IP address of the Gateway used for internal listening.")
	err = viper.BindPFlag("listeningAddress", rootCmd.Flags().Lookup("listeningAddress"))
	handleBindingError(err, "listeningAddress")

	rootCmd.Flags().String("nodeAddress", "",
		"Public IP address of the Node associated with this Gateway. Required field.")
	err = viper.BindPFlag("nodeAddress", rootCmd.Flags().Lookup("nodeAddress"))
	handleBindingError(err, "nodeAddress")

	rootCmd.Flags().StringVar(&certPath, "certPath", "",
		"Path to the self-signed TLS certificate for Gateway. Expects PEM "+
			"format. Required field.")
	err = viper.BindPFlag("certPath", rootCmd.Flags().Lookup("certPath"))
	handleBindingError(err, "certPath")

	rootCmd.Flags().StringVar(&keyPath, "keyPath", "",
		"Path to the private key associated with the self-signed TLS "+
			"certificate. Required field.")
	err = viper.BindPFlag("keyPath", rootCmd.Flags().Lookup("keyPath"))
	handleBindingError(err, "keyPath")

	rootCmd.Flags().StringVar(&serverCertPath, "serverCertPath", "",
		"Path to the self-signed TLS certificate for Server. Expects PEM "+
			"format. Required field.")
	err = viper.BindPFlag("serverCertPath", rootCmd.Flags().Lookup("serverCertPath"))
	handleBindingError(err, "serverCertPath")

	rootCmd.Flags().StringVar(&permissioningCertPath, "permissioningCertPath", "",
		"Path to the self-signed TLS certificate for the Permissioning "+
			"server. Expects PEM format. Required field.")
	err = viper.BindPFlag("permissioningCertPath", rootCmd.Flags().Lookup("permissioningCertPath"))
	handleBindingError(err, "permissioningCertPath")

	// RATE LIMITING FLAGS
	rootCmd.Flags().Uint32Var(&capacity, "capacity", 20,
		"Amount of buckets to keep track of for rate limiting communications")
	err = viper.BindPFlag("capacity", rootCmd.Flags().Lookup("capacity"))
	handleBindingError(err, "Rate_Limiting_Capacity")

	rootCmd.Flags().Uint32Var(&leakedTokens, "leakedTokens", 3,
		"Used to calculate the leak rate")
	err = viper.BindPFlag("leakedTokens", rootCmd.Flags().Lookup("leakedTokens"))
	handleBindingError(err, "Rate_Limiting_LeakedTokens")

	rootCmd.Flags().DurationVar(&leakDuration, "leakDuration", 1*time.Millisecond,
		"Used to calculate the leak rate")
	err = viper.BindPFlag("leakDuration", rootCmd.Flags().Lookup("leakDuration"))
	handleBindingError(err, "Rate_Limiting_LeakDuration")

	rootCmd.Flags().DurationVar(&pollDuration, "pollDuration", 10*time.Second,
		"Duration between polls for stale buckets")
	err = viper.BindPFlag("pollDuration", rootCmd.Flags().Lookup("pollDuration"))
	handleBindingError(err, "Rate_Limiting_PollDuration")

	rootCmd.Flags().DurationVar(&bucketMaxAge, "bucketMaxAge", 10*time.Second,
		"Max time of inactivity before removal")
	err = viper.BindPFlag("bucketMaxAge", rootCmd.Flags().Lookup("bucketMaxAge"))
	handleBindingError(err, "Rate_Limiting_BucketMaxAge")

	// GOSSIP MANAGER FLAGS
	rootCmd.Flags().DurationVar(&bufferExpiration, "bufferExpiration", 300*time.Second,
		"How long a message record should last in the buffer")
	err = viper.BindPFlag("bufferExpiration", rootCmd.Flags().Lookup("bufferExpiration"))
	handleBindingError(err, "Rate_Limiting_BufferExpiration")

	rootCmd.Flags().DurationVar(&monitorThreadFrequency, "monitorThreadFrequency", 150*time.Second,
		"Frequency with which to check the gossip's buffer.")
	err = viper.BindPFlag("monitorThreadFrequency", rootCmd.Flags().Lookup("monitorThreadFrequency"))
	handleBindingError(err, "Rate_Limiting_MonitorThreadFrequency")

	rootCmd.Flags().IntVar(&kr, "kr", 1024, // fixme: probably should be orders of magnitudes bigger?
		"Amount of rounds to keep track of in kr")
	err = viper.BindPFlag("kr", rootCmd.Flags().Lookup("kr"))
	handleBindingError(err, "Known_Rounds")

}

// Handle flag binding errors
func handleBindingError(err error, flag string) {
	if err != nil {
		jww.FATAL.Panicf("Error on binding flag \"%s\":%+v", flag, err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	validConfig = true
	if cfgFile == "" {
		var err error
		cfgFile, err = utils.SearchDefaultLocations("gateway.yaml", "xxnetwork")
		if err != nil {
			validConfig = false
			jww.FATAL.Panicf("Failed to find config file: %+v", err)
		}
	}
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Unable to read config file (%s): %+v", cfgFile, err.Error())
		validConfig = false
	}

}

// initLog initializes logging thresholds and the log path.
func initLog() {
	vipLogLevel := viper.GetUint("logLevel")

	// Check the level of logs to display
	if vipLogLevel > 1 {
		// Set the GRPC log level
		err := os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
		if err != nil {
			jww.ERROR.Printf("Could not set GRPC_GO_LOG_SEVERITY_LEVEL: %+v", err)
		}

		err = os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
		if err != nil {
			jww.ERROR.Printf("Could not set GRPC_GO_LOG_VERBOSITY_LEVEL: %+v", err)
		}
		// Turn on trace logs
		jww.SetLogThreshold(jww.LevelTrace)
		jww.SetStdoutThreshold(jww.LevelTrace)
		mixmessages.TraceMode()
	} else if vipLogLevel == 1 {
		// Turn on debugging logs
		jww.SetLogThreshold(jww.LevelDebug)
		jww.SetStdoutThreshold(jww.LevelDebug)
		mixmessages.DebugMode()
	} else {
		// Turn on info logs
		jww.SetLogThreshold(jww.LevelInfo)
		jww.SetStdoutThreshold(jww.LevelInfo)
	}

	logPath = viper.GetString("log")

	logFile, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644)
	if err != nil {
		fmt.Printf("Could not open log file %s!\n", logPath)
	} else {
		jww.SetLogOutput(logFile)
	}
}
