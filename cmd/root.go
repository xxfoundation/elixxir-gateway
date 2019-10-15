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
	"log"
	"os"
)

var cfgFile string
var verbose bool
var showVer bool
var gatewayNodeIdx int
var gwListenIP string
var gwPort int
var logPath = "cmix-gateway.log"
var disablePermissioning bool
var noTLS bool

// RootCmd represents the base command when called without any sub-commands
var rootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Runs a cMix gateway",
	Long:  `The cMix gateways coordinate communications between servers and clients`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if showVer {
			printVersion()
			return
		}
		jww.INFO.Printf(getVersionInfo())
		if verbose {
			err := os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
			if err != nil {
				jww.ERROR.Printf("Could not set GRPC_GO_LOG_SEVERITY_LEVEL: %+v", err)
			}

			err = os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "2")
			if err != nil {
				jww.ERROR.Printf("Could not set GRPC_GO_LOG_VERBOSITY_LEVEL: %+v", err)
			}
		}

		params := InitParams(viper.GetViper())

		//Build gateway implementation object
		gateway := NewGatewayInstance(params)

		//start gateway network interactions
		gateway.InitNetwork()

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

	batchSize := uint64(vip.GetInt("BatchSize"))

	certPath := vip.GetString("CertPath")

	keyPath := vip.GetString("KeyPath")

	serverCertPath := vip.GetString("ServerCertPath")

	cMixParams := vip.GetStringMapString("groups.cmix")

	firstNode := vip.GetBool("firstNode")
	lastNode := vip.GetBool("lastNode")

	return Params{
		Port:           gwPort,
		Address:        gwListenIP,
		CMixNodes:      cMixNodes,
		GatewayNode:    connectionID(gatewayNode),
		BatchSize:      batchSize,
		CertPath:       certPath,
		KeyPath:        keyPath,
		ServerCertPath: serverCertPath,
		CmixGrp:        cMixParams,
		FirstNode:      firstNode,
		LastNode:       lastNode,
	}
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
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Verbose mode for debugging")
	rootCmd.Flags().BoolVarP(&showVer, "version", "V", false,
		"Show the gateway version information.")
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
	// If verbose flag set then log more info for debugging
	if verbose || viper.GetBool("verbose") {
		jww.SetLogThreshold(jww.LevelDebug)
		jww.SetStdoutThreshold(jww.LevelDebug)
		jww.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	} else {
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
