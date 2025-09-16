package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/hacker65536/aft-pipeline-tool/internal/logger"
	"github.com/hacker65536/aft-pipeline-tool/internal/utils"
)

var (
	cfgFile string
	debug   bool
	noColor bool
	rootCmd = &cobra.Command{
		Use:   "aft-pipeline-tool",
		Short: "AFT CodePipeline Git Triggers management tool",
		Long:  `A CLI tool for managing Git triggers in AFT CodePipelines across multiple AWS accounts.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Set color output based on flag
			utils.SetColorOutput(!noColor)
			return logger.InitLogger(debug)
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.aft-pipeline-tool.yaml)")
	rootCmd.PersistentFlags().String("region", "", "AWS region")
	rootCmd.PersistentFlags().String("profile", "", "AWS profile")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// Bind flags to viper
	if err := viper.BindPFlag("aws.region", rootCmd.PersistentFlags().Lookup("region")); err != nil {
		fmt.Printf("Error binding region flag: %v\n", err)
	}
	if err := viper.BindPFlag("aws.profile", rootCmd.PersistentFlags().Lookup("profile")); err != nil {
		fmt.Printf("Error binding profile flag: %v\n", err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".aft-pipeline-tool" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".aft-pipeline-tool")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Printf("%s %s\n", utils.Info("Using config file:"), viper.ConfigFileUsed())
	}
}
