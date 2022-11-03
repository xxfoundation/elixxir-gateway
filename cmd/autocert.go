////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/crypto/fastRNG"
	"gitlab.com/elixxir/gateway/autocert"
	"gitlab.com/xx_network/crypto/csprng"
)

var autocertCmd = &cobra.Command{
	Use:   "autocert",
	Short: "automatic cert request test command",
	Long:  `Attempt to request a cert for TLS, used for manual testing`,
	Run: func(cmd *cobra.Command, args []string) {
		initLog()
		rng := fastRNG.NewStreamGenerator(10, 1, csprng.NewSystemRNG)
		certKey, err := autocert.GenerateCertKey(rng.GetStream())
		if err != nil {
			jww.ERROR.Printf("%+v", err)
			return
		}

		client, err := autocert.NewRequest(certKey, "rick1.xxn2.work",
			"",
			"")

		if err != nil {
			jww.ERROR.Printf("%+v", err)
			return
		}

		jww.INFO.Printf("client %+v", client)
	},
}

func init() {
	rootCmd.AddCommand(autocertCmd)

	// rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "",
	// 	"Path to load the Gateway configuration file from. (Required)")

	// rootCmd.Flags().IntP("port", "p", -1, "Port for Gateway to listen on."+
	// 	"Gateway must be the only listener on this port. (Required)")
	// err := viper.BindPFlag("port", rootCmd.Flags().Lookup("port"))
	// handleBindingError(err, "port")

}
