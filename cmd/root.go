////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger.
package cmd

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"os"
	"time"
)

var cfgFile string
var logLevel uint // 0 = info, 1 = debug, >1 = trace
var gatewayNodeIdx int
var gwPort int
var logPath = "cmix-gateway.log"
var disablePermissioning bool
var noTLS bool

// For whitelist
var ipBucketCapacity, userBucketCapacity uint
var ipBucketLeakRate, userBucketLeakRate float64
var cleanPeriod, maxDuration string
var ipWhitelistFile, userWhitelistFile string

// RootCmd represents the base command when called without any sub-commands
var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Runs a cMix gateway",
	Long:  `The cMix gateways coordinate communications between servers and clients`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		params := InitParams(viper.GetViper())

		//Build gateway implementation object
		gateway := NewGatewayInstance(params)

		//start gateway network interactions
		err := gateway.InitNetwork()
		if err != nil {
			jww.FATAL.Panicf(err.Error())
		}

		//Begin gateway persistent components
		gateway.Start()

		// Wait forever
		select {}
	},
}

func InitParams(vip *viper.Viper) Params {
	jww.INFO.Printf("Params: \n %+v", vip.AllSettings())

	gwPort := vip.GetInt("Port")
	jww.INFO.Printf("Gateway Port: %d", gwPort)

	vip.SetDefault("Address", "0.0.0.0")
	gwListenIP := vip.GetString("Address")
	jww.INFO.Printf("Gateway Listen IP Address: %s", gwListenIP)

	cMixNodes := vip.GetStringSlice("CMixNodes")

	gatewayNodeIdx = viper.GetInt("Index")
	gatewayNode := cMixNodes[gatewayNodeIdx]
	jww.INFO.Printf("Gateway node %d: %s", gatewayNodeIdx, gatewayNode)

	certPath := vip.GetString("CertPath")

	keyPath := vip.GetString("KeyPath")

	serverCertPath := vip.GetString("ServerCertPath")

	permissioningCertPath := vip.GetString("PermissioningCertPath")

	cMixParams := vip.GetStringMapString("groups.cmix")

	firstNode := vip.GetBool("firstNode")
	lastNode := vip.GetBool("lastNode")

	cleanPeriodDur, err := time.ParseDuration(vip.GetString("Clean_Period"))
	if err != nil {
		jww.ERROR.Printf("Value for cleanPeriod incorrect %v: %v", cleanPeriod, err)
	}

	maxDurationDur, err := time.ParseDuration(vip.GetString("Max_Duration"))
	if err != nil {
		jww.ERROR.Printf("Value for IP address MaxDuration incorrect %v: %v", maxDuration, err)
	}

	ipBucketParams := rateLimiting.Params{
		Capacity:      vip.GetUint("IP_LeakyBucket_Capacity"),
		LeakRate:      vip.GetFloat64("IP_LeakyBucket_Rate"),
		CleanPeriod:   cleanPeriodDur,
		MaxDuration:   maxDurationDur,
		WhitelistFile: vip.GetString("IP_Whitelist_File"),
	}

	userBucketParams := rateLimiting.Params{
		Capacity:      vip.GetUint("User_LeakyBucket_Capacity"),
		LeakRate:      vip.GetFloat64("User_LeakyBucket_Rate"),
		CleanPeriod:   cleanPeriodDur,
		MaxDuration:   maxDurationDur,
		WhitelistFile: vip.GetString("User_Whitelist_File"),
	}

	p := Params{
		Port:                  gwPort,
		Address:               gwListenIP,
		CMixNodes:             cMixNodes,
		NodeAddress:           gatewayNode,
		CertPath:              certPath,
		KeyPath:               keyPath,
		ServerCertPath:        serverCertPath,
		PermissioningCertPath: permissioningCertPath,
		CmixGrp:               cMixParams,
		FirstNode:             firstNode,
		LastNode:              lastNode,
		IpBucket:              ipBucketParams,
		UserBucket:            userBucketParams,
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
	cobra.OnInitialize(initConfig, initLog)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "",
		"config file (default is $HOME/.elixxir/gateway.yaml)")
	rootCmd.Flags().UintVarP(&logLevel, "logLevel", "l", 0,
		"Level of debugging to display. 0 = info, 1 = debug, >1 = trace")
	rootCmd.Flags().IntVarP(&gatewayNodeIdx, "index", "i", -1,
		"Index of the node to connect to from the list of nodes.")
	rootCmd.Flags().IntVarP(&gwPort, "port", "p", -1,
		"Port for the gateway to listen on.")
	rootCmd.Flags().BoolVarP(&disablePermissioning, "disablePermissioning", "",
		false, "Disables interaction with the Permissioning Server")
	rootCmd.Flags().BoolVarP(&noTLS, "noTLS", "", false,
		"Set to ignore TLS")

	// Bind command line flags to config file parameters
	err := viper.BindPFlag("index", rootCmd.Flags().Lookup("index"))
	handleBindingError(err, "index")
	err = viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	handleBindingError(err, "index")

	// flags for leaky bucket
	rootCmd.Flags().Float64Var(&ipBucketLeakRate,
		"IP_LeakyBucket_Rate", 0.000005,
		"The leak rate for the IP address bucket in tokens/nanosecond.")
	rootCmd.Flags().Float64Var(&userBucketLeakRate,
		"User_LeakyBucket_Rate", 0.000005,
		"The leak rate for the user ID bucket in tokens/nanosecond.")
	rootCmd.Flags().UintVar(&ipBucketCapacity,
		"IP_LeakyBucket_Capacity", 4000,
		"The max capacity for the IP address bucket.")
	rootCmd.Flags().UintVar(&userBucketCapacity,
		"User_LeakyBucket_Capacity", 4000,
		"The max capacity for the user ID bucket.")
	rootCmd.Flags().StringVarP(&cleanPeriod,
		"Clean_Period", "", "30m",
		"The period at which stale buckets are removed")
	rootCmd.Flags().StringVarP(&maxDuration,
		"Max_Duration", "", "15m",
		"The max duration a bucket can persist before being removed.")
	rootCmd.Flags().StringVarP(&ipWhitelistFile,
		"IP_Whitelist_File", "", "",
		"List of whitelisted IP addresses.")
	rootCmd.Flags().StringVarP(&userWhitelistFile,
		"User_Whitelist_File", "", "",
		"List of whitelisted user IDs.")

	err = viper.BindPFlag("IP_LeakyBucket_Rate", rootCmd.Flags().Lookup("IP_LeakyBucket_Rate"))
	handleBindingError(err, "IP_LeakyBucket_Rate")
	err = viper.BindPFlag("User_LeakyBucket_Rate", rootCmd.Flags().Lookup("User_LeakyBucket_Rate"))
	handleBindingError(err, "User_LeakyBucket_Rate")
	err = viper.BindPFlag("IP_LeakyBucket_Capacity", rootCmd.Flags().Lookup("IP_LeakyBucket_Capacity"))
	handleBindingError(err, "IP_LeakyBucket_Capacity")
	err = viper.BindPFlag("User_LeakyBucket_Capacity", rootCmd.Flags().Lookup("User_LeakyBucket_Capacity"))
	handleBindingError(err, "User_LeakyBucket_Capacity")
	err = viper.BindPFlag("Clean_Period", rootCmd.Flags().Lookup("Clean_Period"))
	handleBindingError(err, "Clean_Period")
	err = viper.BindPFlag("Max_Duration", rootCmd.Flags().Lookup("Max_Duration"))
	handleBindingError(err, "Max_Duration")
	err = viper.BindPFlag("IP_Whitelist_File", rootCmd.Flags().Lookup("IP_Whitelist_File"))
	handleBindingError(err, "IP_Whitelist_File")
	err = viper.BindPFlag("User_Whitelist_File", rootCmd.Flags().Lookup("User_Whitelist_File"))
	handleBindingError(err, "User_Whitelist_File")

	// Set the default message timeout
	viper.SetDefault("MessageTimeout", 60)
}

// Handle flag binding errors
func handleBindingError(err error, flag string) {
	if err != nil {
		jww.FATAL.Panicf("Error on binding flag \"%s\":%+v", flag, err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile == "" {
		// Default search paths
		var searchDirs []string
		searchDirs = append(searchDirs, "./") // $PWD
		// $HOME
		home, _ := homedir.Dir()
		searchDirs = append(searchDirs, home+"/.elixxir/")
		// /etc/elixxir
		searchDirs = append(searchDirs, "/etc/.elixxir")
		jww.DEBUG.Printf("Configuration search directories: %v", searchDirs)

		for i := range searchDirs {
			cfgFile = searchDirs[i] + "/gateway.yaml"
			_, err := os.Stat(cfgFile)
			if !os.IsNotExist(err) {
				break
			}
		}
	}
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Unable to read config file (%s): %+v", cfgFile, err.Error())
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

	if viper.Get("log") != nil {
		// Create log file, overwrites if existing
		logPath = viper.GetString("log")
	} else {
		fmt.Printf("Invalid or missing log path %s, "+
			"default path used.\n", logPath)
	}
	logFile, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644)
	if err != nil {
		fmt.Printf("Could not open log file %s!\n", logPath)
	} else {
		jww.SetLogOutput(logFile)
	}
}
