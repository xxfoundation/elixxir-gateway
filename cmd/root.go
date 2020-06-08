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
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/primitives/rateLimiting"
	"os"
	"time"
)

var cfgFile string
var idfPath string
var logLevel uint // 0 = info, 1 = debug, >1 = trace
var gwPort int
var logPath string
var messageTimeout time.Duration
var listeningAddress, nodeAddress, certPath, keyPath, serverCertPath, permissioningCertPath string

// For whitelist
var ipBucketCapacity, userBucketCapacity uint
var ipBucketLeakRate, userBucketLeakRate float64
var cleanPeriod, maxDuration string
var ipWhitelistFile, userWhitelistFile string

var requiredFlags []*pflag.Flag

func setFlagAsRequired(f string) {
	requiredFlags = append(requiredFlags, rootCmd.Flags().Lookup(f))
}

// RootCmd represents the base command when called without any sub-commands
var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Runs a cMix gateway",
	Long:  `The cMix gateways coordinate communications between servers and clients`,
	Args:  cobra.NoArgs,
	PreRun: func(cmd *cobra.Command, args []string) {
		// If not all required flags are present, then panic
		missingFlags := false
		for _, f := range requiredFlags {
			if !f.Changed {
				if f.Shorthand != "" {
					jww.ERROR.Printf("Flag --%s (-%s) is manditory", f.Name, f.Shorthand)
				} else {
					jww.ERROR.Printf("Flag --%s is manditory", f.Name)
				}
				missingFlags = true
			}
		}
		if missingFlags {
			jww.FATAL.Print("Flag(s) are missing")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		params := InitParams(viper.GetViper())

		// Build gateway implementation object
		gateway := NewGatewayInstance(params)

		// start gateway network interactions
		err := gateway.InitNetwork()
		if err != nil {
			jww.FATAL.Panicf(err.Error())
		}

		// Begin gateway persistent components
		gateway.Start()

		// Wait forever
		select {}
	},
}

func InitParams(vip *viper.Viper) Params {
	jww.INFO.Printf("Params: \n %+v", vip.AllSettings())

	jww.INFO.Printf("Gateway port: %d", gwPort)
	jww.INFO.Printf("Gateway listen IP address: %s", listeningAddress)
	jww.INFO.Printf("Gateway node: %s", nodeAddress)

	idfPath = vip.GetString("idfPath")

	cleanPeriodDur, err := time.ParseDuration(cleanPeriod)
	if err != nil {
		jww.ERROR.Printf("Value for cleanPeriod incorrect %v: %v", cleanPeriod, err)
	}

	maxDurationDur, err := time.ParseDuration(maxDuration)
	if err != nil {
		jww.ERROR.Printf("Value for IP address MaxDuration incorrect %v: %v", maxDuration, err)
	}

	ipBucketParams := rateLimiting.Params{
		Capacity:      ipBucketCapacity,
		LeakRate:      ipBucketLeakRate,
		CleanPeriod:   cleanPeriodDur,
		MaxDuration:   maxDurationDur,
		WhitelistFile: ipWhitelistFile,
	}

	userBucketParams := rateLimiting.Params{
		Capacity:      userBucketCapacity,
		LeakRate:      userBucketLeakRate,
		CleanPeriod:   cleanPeriodDur,
		MaxDuration:   maxDurationDur,
		WhitelistFile: userWhitelistFile,
	}

	p := Params{
		Port:                  gwPort,
		Address:               listeningAddress,
		NodeAddress:           nodeAddress,
		CertPath:              certPath,
		KeyPath:               keyPath,
		ServerCertPath:        serverCertPath,
		IDFPath:               idfPath,
		PermissioningCertPath: permissioningCertPath,
		IpBucket:              ipBucketParams,
		UserBucket:            userBucketParams,
		MessageTimeout:        messageTimeout,
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
	home, _ := homedir.Dir()
	rootCmd.Flags().StringVarP(&cfgFile, "config", "c",
		home+"/.xxnetwork/gateway.yaml",
		"The location of the Gateway configuration file.")

	rootCmd.Flags().IntVarP(&gwPort, "port", "p", -1,
		"Port for the Gateway to listen on. Gateway must be the only listener "+
			"on this port. Required field.")
	setFlagAsRequired("config")
	err := viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	handleBindingError(err, "port")

	rootCmd.Flags().StringVar(&idfPath, "idfPath",
		home+"/.xxnetwork/idf.json",
		"Path to where the IDF is saved. This is used by the wrapper "+
			"management script.")
	err = viper.BindPFlag("idfPath", rootCmd.Flags().Lookup("idfPath"))
	handleBindingError(err, "idfPath")

	rootCmd.Flags().UintVarP(&logLevel, "logLevel", "l", 0,
		"Level of debugging to print. 0 = info, 1 = debug, >1 = trace")
	err = viper.BindPFlag("logLevel", rootCmd.Flags().Lookup("logLevel"))
	handleBindingError(err, "logLevel")

	rootCmd.Flags().StringVar(&logPath, "log",
		home+"/.xxnetwork/cmix-gateway.log",
		"Path where logs will be printed.")
	err = viper.BindPFlag("log", rootCmd.Flags().Lookup("log"))
	handleBindingError(err, "log")

	rootCmd.Flags().DurationVar(&messageTimeout, "messageTimeout", 60*time.Second,
		"Period in which the message cleanup function executes. Recommended "+
			"period is on the order of a minute.")
	err = viper.BindPFlag("messageTimeout", rootCmd.Flags().Lookup("messageTimeout"))
	handleBindingError(err, "messageTimeout")

	rootCmd.Flags().StringVar(&listeningAddress, "listeningAddress", "0.0.0.0",
		"The local IP address of the Gateway used for internal listening.")
	err = viper.BindPFlag("listeningAddress", rootCmd.Flags().Lookup("listeningAddress"))
	handleBindingError(err, "listeningAddress")
	err = rootCmd.Flags().MarkHidden("listeningAddress")
	handleBindingError(err, "listeningAddress")

	rootCmd.Flags().StringVar(&nodeAddress, "nodeAddress", "",
		"The public IP address of the Node associated with this Gateway. "+
			"Required field.")
	err = viper.BindPFlag("nodeAddress", rootCmd.Flags().Lookup("nodeAddress"))
	handleBindingError(err, "nodeAddress")
	setFlagAsRequired("nodeAddress")

	rootCmd.Flags().StringVar(&certPath, "certPath", "",
		"The path to the self-signed TLS certificate for Gateway. Expects PEM "+
			"format. Required field.")
	err = viper.BindPFlag("certPath", rootCmd.Flags().Lookup("certPath"))
	handleBindingError(err, "certPath")
	setFlagAsRequired("certPath")

	rootCmd.Flags().StringVar(&keyPath, "keyPath", "",
		"The path to the private key associated with the self-signed TLS "+
			"certificate. Required field.")
	err = viper.BindPFlag("keyPath", rootCmd.Flags().Lookup("keyPath"))
	handleBindingError(err, "keyPath")
	setFlagAsRequired("keyPath")

	rootCmd.Flags().StringVar(&serverCertPath, "serverCertPath", "",
		"The path to the self-signed TLS certificate for Server. Expects PEM "+
			"format. Required field.")
	err = viper.BindPFlag("serverCertPath", rootCmd.Flags().Lookup("serverCertPath"))
	handleBindingError(err, "serverCertPath")
	setFlagAsRequired("serverCertPath")

	rootCmd.Flags().StringVar(&permissioningCertPath, "permissioningCertPath", "",
		"The path to the self-signed TLS certificate for the Permissioning "+
			"server. Expects PEM format. Required field.")
	err = viper.BindPFlag("permissioningCertPath", rootCmd.Flags().Lookup("permissioningCertPath"))
	handleBindingError(err, "permissioningCertPath")
	setFlagAsRequired("permissioningCertPath")

	// DEPRECIATED - Flags for leaky bucket
	rootCmd.Flags().Float64Var(&ipBucketLeakRate,
		"IP_LeakyBucket_Rate", 0.000005,
		"The leak rate for the IP address bucket in tokens/nanosecond.")
	err = viper.BindPFlag("IP_LeakyBucket_Rate", rootCmd.Flags().Lookup("IP_LeakyBucket_Rate"))
	handleBindingError(err, "IP_LeakyBucket_Rate")
	err = rootCmd.Flags().MarkHidden("IP_LeakyBucket_Rate")
	handleBindingError(err, "IP_LeakyBucket_Rate")

	rootCmd.Flags().Float64Var(&userBucketLeakRate,
		"User_LeakyBucket_Rate", 0.000005,
		"The leak rate for the user ID bucket in tokens/nanosecond.")
	err = viper.BindPFlag("User_LeakyBucket_Rate", rootCmd.Flags().Lookup("User_LeakyBucket_Rate"))
	handleBindingError(err, "User_LeakyBucket_Rate")
	err = rootCmd.Flags().MarkHidden("User_LeakyBucket_Rate")
	handleBindingError(err, "User_LeakyBucket_Rate")

	rootCmd.Flags().UintVar(&ipBucketCapacity,
		"IP_LeakyBucket_Capacity", 4000,
		"The max capacity for the IP address bucket.")
	err = viper.BindPFlag("IP_LeakyBucket_Capacity", rootCmd.Flags().Lookup("IP_LeakyBucket_Capacity"))
	handleBindingError(err, "IP_LeakyBucket_Capacity")
	err = rootCmd.Flags().MarkHidden("IP_LeakyBucket_Capacity")
	handleBindingError(err, "IP_LeakyBucket_Capacity")

	rootCmd.Flags().UintVar(&userBucketCapacity,
		"User_LeakyBucket_Capacity", 4000,
		"The max capacity for the user ID bucket.")
	err = viper.BindPFlag("User_LeakyBucket_Capacity", rootCmd.Flags().Lookup("User_LeakyBucket_Capacity"))
	handleBindingError(err, "User_LeakyBucket_Capacity")
	err = rootCmd.Flags().MarkHidden("User_LeakyBucket_Capacity")
	handleBindingError(err, "User_LeakyBucket_Capacity")

	rootCmd.Flags().StringVarP(&cleanPeriod,
		"Clean_Period", "", "30m",
		"The period at which stale buckets are removed")
	err = viper.BindPFlag("Clean_Period", rootCmd.Flags().Lookup("Clean_Period"))
	handleBindingError(err, "Clean_Period")
	err = rootCmd.Flags().MarkHidden("Clean_Period")
	handleBindingError(err, "Clean_Period")

	rootCmd.Flags().StringVarP(&maxDuration,
		"Max_Duration", "", "15m",
		"DEPRECIATED. The max duration a bucket can persist before being removed.")
	err = viper.BindPFlag("Max_Duration", rootCmd.Flags().Lookup("Max_Duration"))
	handleBindingError(err, "Max_Duration")
	err = rootCmd.Flags().MarkHidden("Max_Duration")
	handleBindingError(err, "Max_Duration")

	rootCmd.Flags().StringVarP(&ipWhitelistFile,
		"IP_Whitelist_File", "", "",
		"List of whitelisted IP addresses.")
	err = viper.BindPFlag("IP_Whitelist_File", rootCmd.Flags().Lookup("IP_Whitelist_File"))
	handleBindingError(err, "IP_Whitelist_File")
	err = rootCmd.Flags().MarkHidden("IP_Whitelist_File")
	handleBindingError(err, "IP_Whitelist_File")

	rootCmd.Flags().StringVarP(&userWhitelistFile,
		"User_Whitelist_File", "", "",
		"List of whitelisted user IDs.")
	err = viper.BindPFlag("User_Whitelist_File", rootCmd.Flags().Lookup("User_Whitelist_File"))
	handleBindingError(err, "User_Whitelist_File")
	err = rootCmd.Flags().MarkHidden("User_Whitelist_File")
	handleBindingError(err, "User_Whitelist_File")
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
		searchDirs = append(searchDirs, home+"/.xxnetwork/")
		// /etc/elixxir
		searchDirs = append(searchDirs, "/etc/.xxnetwork")
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

	if !rootCmd.Flags().Lookup("log").Changed {
		fmt.Printf("Invalid or missing log path, default path %s used.\n",
			logPath)
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
