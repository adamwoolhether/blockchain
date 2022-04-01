package cmd

import (
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Args:  cobra.ExactArgs(1),
	Short: "Generate new key pair",
	RunE: func(cmd *cobra.Command, args []string) error {
		acctName := args[0]
		
		path, err := rootCmd.Flags().GetString("path")
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
