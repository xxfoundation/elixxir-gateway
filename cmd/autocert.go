////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"
	"gitlab.com/elixxir/crypto/fastRNG"
	"gitlab.com/elixxir/gateway/autocert"
	"gitlab.com/xx_network/crypto/csprng"
	"gitlab.com/xx_network/primitives/utils"
)

var autocertCmd = &cobra.Command{
	Use:   "autocert",
	Short: "automatic cert request test command",
	Long:  `Attempt to request a cert for TLS, used for manual testing`,
	Run: func(cmd *cobra.Command, args []string) {
		initLog()

		eabKeyID := viper.GetString("eabKeyID")
		eabKey := viper.GetString("eabKey")
		domain := viper.GetString("domain")
		email := viper.GetString("email")

		if eabKey == "" || eabKeyID == "" || domain == "" {
			fmt.Printf("need eabKeyID, eabKey, and domain: "+
				"%s,%s,%s", eabKeyID, eabKey, domain)
			os.Exit(-1)
		}

		rng := fastRNG.NewStreamGenerator(10, 1, csprng.NewSystemRNG)

		certGetter := autocert.NewDNS()

		privKeyPEM, err := utils.ReadFile("certkey.pem")
		if os.IsNotExist(err) {
			certKey, err := autocert.GenerateCertKey(
				rng.GetStream())
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}

			err = utils.WriteFile("certkey.pem",
				certKey.MarshalPem(),
				0700, 0755)
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}

			err = certGetter.Register(certKey, eabKeyID, eabKey,
				email)
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}
		} else {
			certGetter, err = autocert.LoadDNS(privKeyPEM)
			if err != nil {
				jww.FATAL.Panicf("%+v", err)
			}
		}

		chalDomain, challenge, err := certGetter.Request(domain)
		if err != nil {
			jww.FATAL.Panicf("%+v", err)
		}

		fmt.Printf("ADD TXT RECORD: %s\t%s\n", chalDomain, challenge)

		csrPEM, csrDER, err := certGetter.CreateCSR(domain, email, "USA",
			"NodeID", rng.GetStream())
		if err != nil {
			jww.FATAL.Panicf("%+v", err)
			return
		}

		err = utils.WriteFile("cert-csr.pem", csrPEM, 0700, 0755)
		if err != nil {
			jww.FATAL.Panicf("%+v", err)
			return
		}

		cert, key, err := certGetter.Issue(csrDER, time.Hour)
		if err != nil {
			jww.FATAL.Panicf("%+v", err)
			return
		}

		err = utils.WriteFile("cert.pem", cert, 0700, 0755)

		if err != nil {
			jww.FATAL.Panicf("%+v", err)
			return
		}

		err = utils.WriteFile("certkey.pem", key, 0700, 0755)
		if err != nil {
			jww.FATAL.Panicf("%+v", err)
			return
		}

	},
}

func init() {
	rootCmd.AddCommand(autocertCmd)

	autocertCmd.Flags().StringP("eabKeyID", "i", "",
		"EAB Key ID (Required)")
	err := viper.BindPFlag("eabKeyID", autocertCmd.Flags().Lookup(
		"eabKeyID"))
	handleBindingError(err, "eabKeyID")

	autocertCmd.Flags().StringP("eabKey", "k", "",
		"EAB Key base64 format (Required)")
	err = viper.BindPFlag("eabKey", autocertCmd.Flags().Lookup(
		"eabKey"))
	handleBindingError(err, "eabKey")

	autocertCmd.Flags().StringP("domain", "d", "",
		"domain name to attempt to register")
	err = viper.BindPFlag("domain", autocertCmd.Flags().Lookup(
		"domain"))
	handleBindingError(err, "domain")

	autocertCmd.Flags().StringP("email", "e", "admins@elixxir.io",
		"email for registration, defaults to admins@elixxir.io")
	err = viper.BindPFlag("email", autocertCmd.Flags().Lookup(
		"email"))
	handleBindingError(err, "email")

}
