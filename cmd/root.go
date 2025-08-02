package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "getgmail",
	Short: "A CLI tool for downloading Gmail emails to local folders",
	Long: `getgmail is a command-line interface tool written in Go that makes it 
possible to download Gmail emails to a local folder. Each email is saved 
in its own directory with metadata and body content.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}