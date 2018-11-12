////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger.
package cmd

import (
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/privategrity/comms/connect"
	"gitlab.com/privategrity/comms/gateway"
	"log"
	"os"
	"strings"
)

var cfgFile string
var verbose bool
var showVer bool
var validConfig bool

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Runs a cMix gateway",
	Long:  `The cMix gateways coordinate communications between servers and clients`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if showVer {
			printVersion()
			return
		}
		if !validConfig {
			jww.WARN.Println("Invalid Config File")
		}

		address := viper.GetString("GatewayAddress")
		jww.INFO.Println("Gateway address: " + address)
		cmixNodes := viper.GetStringSlice("cMixNodes")
		gatewayNode := cmixNodes[viper.GetInt("GatewayNodeIndex")]
		jww.INFO.Println("Gateway node: " + gatewayNode)
		batchSize := uint64(viper.GetInt("batchSize"))

		gatewayImpl := NewGatewayImpl(batchSize, cmixNodes, gatewayNode)
		certPath := getFullPath(viper.GetString("certPath"))
		keyPath := getFullPath(viper.GetString("keyPath"))
		serverCertPath := getFullPath(viper.GetString("serverCertPath"))
		connect.ServerCertPath = serverCertPath
		gateway.StartGateway(address, gatewayImpl, certPath, keyPath)

		// Wait forever
		select {}
	},
}

// Execute adds all child commands to the root command and sets flags
// appropriately.  This is called by main.main(). It only needs to
// happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
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
	// very good reason not to have them as local params to sub command."
	cobra.OnInitialize(initConfig, initLog)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.Flags().StringVarP(&cfgFile, "config", "", "",
		"config file (default is $HOME/.privategrity/gateway.yaml)")
	RootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"Verbose mode for debugging")
	RootCmd.Flags().BoolVarP(&showVer, "version", "V", false,
		"Show the gateway version information.")

	// Set the default message timeout
	viper.SetDefault("MessageTimeout", 60)
}

// Given a path, replace a "~" character
// with the home directory to return a full file path
func getFullPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			jww.ERROR.Println(err)
			os.Exit(1)
		}
		// Append the home directory to the path
		return home + strings.TrimLeft(path, "~")
	}
	return path
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Default search paths
	var searchDirs []string
	searchDirs = append(searchDirs, "./") // $PWD
	// $HOME
	home, _ := homedir.Dir()
	searchDirs = append(searchDirs, home+"/.privategrity/")
	// /etc/privategrity
	searchDirs = append(searchDirs, "/etc/privategrity")
	jww.DEBUG.Printf("Configuration search directories: %v", searchDirs)

	validConfig = false
	for i := range searchDirs {
		if cfgFile == "" {
			cfgFile = searchDirs[i] + "gateway.yaml"
		} else {
			// Use config filename if we got one on the command line
			cfgFile = searchDirs[i] + cfgFile
		}
		_, err := os.Stat(cfgFile)
		if !os.IsNotExist(err) {
			validConfig = true
			viper.SetConfigFile(cfgFile)
			break
		}
	}
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		jww.WARN.Printf("Unable to read config file (%s): %s", cfgFile, err.Error())
		validConfig = false
	}

}

// initLog initializes logging thresholds and the log path.
func initLog() {
	if viper.Get("log") != nil {
		// If verbose flag set then log more info for debugging
		if verbose || viper.GetBool("verbose") {
			jww.SetLogThreshold(jww.LevelDebug)
			jww.SetStdoutThreshold(jww.LevelDebug)
			jww.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
		} else {
			jww.SetLogThreshold(jww.LevelInfo)
			jww.SetStdoutThreshold(jww.LevelInfo)
		}
		// Create log file, overwrites if existing
		logPath := viper.GetString("log")
		logFile, err := os.Create(logPath)
		if err != nil {
			jww.WARN.Println("Invalid or missing log path, default path used.")
		} else {
			jww.SetLogOutput(logFile)
		}
	}
}
