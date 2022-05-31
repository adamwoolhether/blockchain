package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"github.com/adamwoolhether/blockchain/foundation/blockchain/database"
)

var (
	url   string
	nonce uint64
	to    string
	value uint64
	tip   uint64
	data  []byte
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send transaction",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		acctName := args[0]

		path, err := rootCmd.Flags().GetString("path")
		if err != nil {
			return err
		}

		user := keyPath(acctName, path)

		return runSend(user)
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVarP(&url, "url", "u", "http://localhost:8080", "Url of the node.")
	sendCmd.Flags().Uint64VarP(&nonce, "nonce", "n", 0, "id for the transaction.")
	sendCmd.Flags().StringVarP(&to, "to", "t", "", "Url of the node.")
	sendCmd.Flags().Uint64VarP(&value, "value", "v", 0, "Value to send.")
	sendCmd.Flags().Uint64VarP(&tip, "tip", "c", 0, "Tip to send.")
	sendCmd.Flags().BytesHexVarP(&data, "data", "d", nil, "Data to send.")
}

func runSend(user string) error {
	privateKey, err := crypto.LoadECDSA(user)
	if err != nil {
		return err
	}

	toAccount, err := database.ToAccountID(to)
	if err != nil {
		return err
	}

	const chainID = 1
	tx, err := database.NewTx(chainID, nonce, toAccount, value, tip, data)
	if err != nil {
		return err
	}

	walletTx, err := tx.Sign(privateKey)
	if err != nil {
		return err
	}

	data, err := json.Marshal(walletTx)
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("%s/v1/tx/submit", url), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
