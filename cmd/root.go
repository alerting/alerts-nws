package cmd

import (
	"fmt"
	"os"

	"github.com/alerting/alerts/pkg/alerts"
	"google.golang.org/grpc"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var brokers []string
var fetchTopic string

var alertsAddress string
var alertsService alerts.AlertsServiceClient

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "alerts-nws",
	Short: "Fetches alerts issued from the National Weather Service (NWS).",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize the alerts service
		if alertsAddress != "" {
			grpcConn, err := grpc.Dial(
				alertsAddress,
				grpc.WithMaxMsgSize(1024*1024*1024),
				grpc.WithInsecure(),
			)
			if err != nil {
				return err
			}

			alertsService = alerts.NewAlertsServiceClient(grpcConn)
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.alerts-nws.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().StringArrayVarP(&brokers, "brokers", "b", []string{}, "Brokers")
	rootCmd.PersistentFlags().StringVarP(&fetchTopic, "fetch-topic", "f", "nws-fetch", "Fetch topic")

	// Alert service
	rootCmd.PersistentFlags().StringVar(&alertsAddress, "alerts-service", "", "Address of alerts service")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".alerts-nws" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".alerts-nws")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
