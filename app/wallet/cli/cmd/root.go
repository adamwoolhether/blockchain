// Package cmd contains wallet app commands.
package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const keyExt = ".ecdsa"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "Simple wallet appl",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// rootCmd.PersistentFlags().StringP("account", "a", "private.ecdsa", "Path to the private key.")
	rootCmd.PersistentFlags().StringP("account-path", "p", "zblock/accounts/", "Path to the directory with private keys.")
	rootCmd.PersistentFlags().StringP("account", "a", "private.ecdsa", "The account to use.")
}

func keyPath(acctName, path string) string {
	if !strings.HasSuffix(acctName, keyExt) {
		acctName += keyExt
	}

	return filepath.Join(path, acctName)
}
