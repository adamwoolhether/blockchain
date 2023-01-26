package cmd

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

/*
	This gist shows how to create a wallet with PK's generated from a Mnemonic.
	https://gist.github.com/miguelmota/ee0fd9756e1651f38f4cd38c6e99b8bf
*/

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Args:  cobra.ExactArgs(1),
	Short: "Generate new key pair",
	RunE: func(cmd *cobra.Command, args []string) error {
		acctName, err := rootCmd.Flags().GetString("account")
		if err != nil {
			return err
		}

		path, err := rootCmd.Flags().GetString("account-path")
		if err != nil {
			return err
		}

		dest := keyPath(acctName, path)

		return runKeyGen(dest)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}

func runKeyGen(dest string) error {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return err
	}

	if err := crypto.SaveECDSA(dest, privateKey); err != nil {
		return err
	}

	return nil
}
