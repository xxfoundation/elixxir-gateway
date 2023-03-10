////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// Package cmd initializes the CLI and config parsers as well as the logger.
package cmd

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/cmix"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/utils"
	"google.golang.org/grpc/grpclog"
)

// Flags to import from command line or config file
var (
	cfgFile, idfPath, logPath string
	certPath, keyPath, serverCertPath,
	permissioningCertPath string
	logLevel    uint // 0 = info, 1 = debug, >1 = trace
	gwPort      int
	validConfig bool

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
		initConfig()
		initLog()
		profileOut := viper.GetString("profile-cpu")
		if profileOut != "" {
			f, err := os.Create(profileOut)
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}
			pprof.StartCPUProfile(f)
		}

		params := InitParams(viper.GetViper())
		// Build gateway implementation object
		gateway := NewGatewayInstance(params)
		err := gateway.SetPeriod()
		if err != nil {
			jww.FATAL.Panicf("Unable to set gateway period: %+v", err)
		}

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

		if params.DevMode {
			jww.WARN.Printf("Starting in developer mode (devMode)" +
				" -- this will break on betanet or mainnet...")
			addPrecannedIDs(gateway)
		}

		jww.INFO.Printf("Starting xx network gateway v%s", SEMVER)

		// Begin gateway persistent components
		if !params.DisableGossip {
			jww.INFO.Println("Gossip is enabled")
			gateway.StartPeersThread()
		}

		gateway.Start()

		// Open Signal Handler for safe program exit
		stopCh := ReceiveExitSignal()

		// Block forever to prevent the program ending
		// Block until a signal is received, then call the function
		// provided
		select {
		case <-stopCh:
			jww.INFO.Printf(
				"Received Exit (SIGTERM or SIGINT) signal...\n")
			select {
			case gateway.ipAddrRateLimitQuit <- struct{}{}:
			case <-time.After(20 * time.Second):
				jww.ERROR.Println("Failed to stop ipAddrRateLimit")
			}

			select {
			case gateway.idRateLimitQuit <- struct{}{}:
			case <-time.After(20 * time.Second):
				jww.ERROR.Println("Failed to stop idRateLimitQuit")
			}

			select {
			case gateway.earliestRoundQuitChan <- struct{}{}:
			case <-time.After(20 * time.Second):
				jww.ERROR.Println("Failed to stop earliestRoundQuitChan")
			}

			select {
			case gateway.replaceCertificateQuit <- struct{}{}:
			case <-time.After(20 * time.Second):
				jww.ERROR.Println("Failed to stop replace certificate thread")
			}

			gateway.Comms.Shutdown()

			if profileOut != "" {
				pprof.StopCPUProfile()
			}
		}

	},
}

// ReceiveExitSignal signals a stop chan when it receives
// SIGTERM or SIGINT
func ReceiveExitSignal() chan os.Signal {
	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	return c
}

func addPrecannedIDs(gateway *Instance) {
	// add precannedIDs
	for i := uint64(0); i < 41; i++ {
		u := new(id.ID)
		binary.BigEndian.PutUint64(u[:], i)
		u.SetType(id.User)
		h := sha256.New()
		h.Reset()
		h.Write([]byte(strconv.Itoa(int(4000 + i))))
		baseKey := gateway.NetInf.GetCmixGroup().NewIntFromBytes(h.Sum(nil))
		jww.INFO.Printf("Added precan transmisssion key: %v",
			baseKey.Bytes())
		cgKey := cmix.GenerateClientGatewayKey(baseKey)
		// Insert client information to database
		newClient := &storage.Client{
			Id:  u.Marshal(),
			Key: cgKey,
		}

		err := gateway.storage.UpsertClient(newClient)
		if err != nil {
			jww.ERROR.Printf("Unable to insert precanned client: %+v", err)
		}
	}
	jww.INFO.Printf("Added precanned users")
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
		"Path to load the Gateway configuration file from. (Required)")

	rootCmd.Flags().IntP("port", "p", -1, "Port for Gateway to listen on."+
		"Gateway must be the only listener on this port. (Required)")
	err := viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	handleBindingError(err, "port")

	rootCmd.Flags().StringVar(&idfPath, "idfPath", "",
		"Path to where the identity file (IDF) is saved. The IDF stores the "+
			"Gateway's Node's network identity. This is used by the wrapper "+
			"management script. (Required)")
	err = viper.BindPFlag("idfPath", rootCmd.Flags().Lookup("idfPath"))
	handleBindingError(err, "idfPath")

	rootCmd.Flags().UintVarP(&logLevel, "logLevel", "l", 0,
		"Level of debugging to print (0 = info, 1 = debug, >1 = trace).")
	err = viper.BindPFlag("logLevel", rootCmd.Flags().Lookup("logLevel"))
	handleBindingError(err, "logLevel")

	rootCmd.Flags().StringVar(&logPath, "log", "log/gateway.log",
		"Path where log file will be saved.")
	err = viper.BindPFlag("log", rootCmd.Flags().Lookup("log"))
	handleBindingError(err, "log")

	rootCmd.Flags().String("cmixAddress", "",
		"The IP address of the machine running cMix that the Gateway "+
			"communicates with. Expects an IPv4 address with a port. (Required)")
	err = viper.BindPFlag("cmixAddress", rootCmd.Flags().Lookup("cmixAddress"))
	handleBindingError(err, "cmixAddress")

	rootCmd.Flags().StringVar(&certPath, "certPath", "",
		"Path to the self-signed TLS certificate for Gateway. Expects PEM "+
			"format. (Required)")
	err = viper.BindPFlag("certPath", rootCmd.Flags().Lookup("certPath"))
	handleBindingError(err, "certPath")

	rootCmd.Flags().StringVar(&keyPath, "keyPath", "",
		"Path to the private key associated with the self-signed TLS "+
			"certificate. (Required)")
	err = viper.BindPFlag("keyPath", rootCmd.Flags().Lookup("keyPath"))
	handleBindingError(err, "keyPath")

	rootCmd.Flags().StringVar(&serverCertPath, "cmixCertPath", "",
		"Path to the self-signed TLS certificate for cMix. Expects PEM "+
			"format. (Required)")
	err = viper.BindPFlag("cmixCertPath", rootCmd.Flags().Lookup("cmixCertPath"))
	handleBindingError(err, "cmixCertPath")

	rootCmd.Flags().StringVar(&permissioningCertPath, "schedulingCertPath", "",
		"Path to the self-signed TLS certificate for the Scheduling server. "+
			"Expects PEM format. (Required)")
	err = viper.BindPFlag("schedulingCertPath", rootCmd.Flags().Lookup("schedulingCertPath"))
	handleBindingError(err, "schedulingCertPath")

	// RATE LIMITING FLAGS
	rootCmd.Flags().Uint32Var(&capacity, "capacity", 20,
		"The capacity of rate-limiting buckets in the map.")
	err = viper.BindPFlag("capacity", rootCmd.Flags().Lookup("capacity"))
	handleBindingError(err, "Rate_Limiting_Capacity")

	rootCmd.Flags().Uint32Var(&leakedTokens, "leakedTokens", 3,
		"The rate that the rate limiting bucket leaks tokens at [tokens/ns].")
	err = viper.BindPFlag("leakedTokens", rootCmd.Flags().Lookup("leakedTokens"))
	handleBindingError(err, "Rate_Limiting_LeakedTokens")

	rootCmd.Flags().DurationVar(&leakDuration, "leakDuration", 1*time.Millisecond,
		"How often the number of leaked tokens is leaked from the bucket.")
	err = viper.BindPFlag("leakDuration", rootCmd.Flags().Lookup("leakDuration"))
	handleBindingError(err, "Rate_Limiting_LeakDuration")

	rootCmd.Flags().DurationVar(&pollDuration, "pollDuration", 10*time.Second,
		"How often inactive buckets are removed.")
	err = viper.BindPFlag("pollDuration", rootCmd.Flags().Lookup("pollDuration"))
	handleBindingError(err, "Rate_Limiting_PollDuration")

	rootCmd.Flags().DurationVar(&bucketMaxAge, "bucketMaxAge", 10*time.Second,
		"The max age of a bucket without activity before it is removed.")
	err = viper.BindPFlag("bucketMaxAge", rootCmd.Flags().Lookup("bucketMaxAge"))
	handleBindingError(err, "Rate_Limiting_BucketMaxAge")

	// GOSSIP MANAGER FLAGS
	rootCmd.Flags().BoolP("enableGossip", "", false,
		"Feature flag for in progress gossip functionality")
	err = viper.BindPFlag("enableGossip", rootCmd.Flags().Lookup("enableGossip"))
	handleBindingError(err, "Enable_Gossip")

	rootCmd.Flags().DurationVar(&bufferExpiration, "bufferExpiration", 300*time.Second,
		"How long a message record should last in the gossip buffer if it "+
			"arrives before the Gateway starts handling the gossip.")
	err = viper.BindPFlag("bufferExpiration", rootCmd.Flags().Lookup("bufferExpiration"))
	handleBindingError(err, "Rate_Limiting_BufferExpiration")

	rootCmd.Flags().DurationVar(&monitorThreadFrequency, "monitorThreadFrequency", 150*time.Second,
		"Frequency with which to check the gossip buffer.")
	err = viper.BindPFlag("monitorThreadFrequency", rootCmd.Flags().Lookup("monitorThreadFrequency"))
	handleBindingError(err, "Rate_Limiting_MonitorThreadFrequency")

	rootCmd.Flags().IntVar(&kr, "kr", 1024, // fixme: probably should be orders of magnitudes bigger?
		"Amount of rounds to keep track of in kr")
	err = viper.BindPFlag("kr", rootCmd.Flags().Lookup("kr"))
	handleBindingError(err, "Known_Rounds")

	// DevMode enables developer mode, which allows you to run without
	// a database and with unsafe "precanned" users
	rootCmd.Flags().BoolP("devMode", "", false,
		"Run in development/testing mode. Do not use on beta or main "+
			"nets")
	err = viper.BindPFlag("devMode", rootCmd.Flags().Lookup("devMode"))
	handleBindingError(err, "Rate_Limiting_MonitorThreadFrequency")
	_ = rootCmd.Flags().MarkHidden("devMode")

	rootCmd.Flags().String("profile-cpu", "",
		"Enable cpu profiling to this file")
	viper.BindPFlag("profile-cpu", rootCmd.Flags().Lookup("profile-cpu"))

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
		jww.FATAL.Panicf("No config file provided.")
	}

	cfgFile, _ = utils.ExpandPath(cfgFile)
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
	// Set log file
	logPath = viper.GetString("log")
	logFile, err := os.OpenFile(logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644)
	if err != nil {
		fmt.Printf("Could not open log file %s!\n", logPath)
		jww.SetLogOutput(os.Stderr)
	} else {
		jww.SetLogOutput(logFile)
	}

	// Check the level of logs to display
	vipLogLevel := viper.GetUint("logLevel")
	if vipLogLevel > 1 {
		// Set GRPC trace logging
		grpcLogger := grpclog.NewLoggerV2WithVerbosity(
			logFile, logFile, logFile, 99)
		grpclog.SetLoggerV2(grpcLogger)

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
}
