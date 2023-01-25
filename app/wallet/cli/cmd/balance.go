package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

type balance struct {
	Account string `json:"account"`
	Balance uint   `json:"balance"`
}

type balances struct {
	LastestBlock string    `json:"lastest_block"`
	Uncommitted  int       `json:"uncommitted"`
	Balances     []balance `json:"balances"`
}

// balanceCmd represents the balance command
var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "A brief description of your command",
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

		return runBalance(user)
	},
}

func init() {
	rootCmd.AddCommand(balanceCmd)
	balanceCmd.Flags().StringVarP(&url, "url", "u", "http://localhost:8080", "Url of the node.")
}

func runBalance(user string) error {
	privateKey, err := crypto.LoadECDSA(user)
	if err != nil {
		return err
	}

	accountID := database.PublicKeyToAccountID(privateKey.PublicKey)
	fmt.Println("For Account:", accountID)

	resp, err := http.Get(fmt.Sprintf("%s/v1/accounts/list/%s", url, accountID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var balances balances
	if err := decoder.Decode(&balances); err != nil {
		return err
	}

	if len(balances.Balances) > 0 {
		fmt.Println(balances.Balances[0].Balance)
	}

	return nil
}
