package cmd

import (
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

// accountCmd represents the account command
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Print account for the specific wallet",
	RunE: func(cmd *cobra.Command, args []string) error {
		acctName, err := rootCmd.Flags().GetString("account")
		if err != nil {
			return err
		}

		path, err := rootCmd.Flags().GetString("account-path")
		if err != nil {
			return err
		}

		user := keyPath(acctName, path)

		return runAccount(user)
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)
}

func runAccount(user string) error {
	privateKey, err := crypto.LoadECDSA(user)
	if err != nil {
		return err
	}

	account := database.PublicKeyToAccountID(privateKey.PublicKey)
	fmt.Println(account)

	return nil
}
