package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alerting/alerts-nws/pkg/fetch"
	"github.com/spf13/cobra"
)

var retryTopic string
var group string
var delay int
var topic string
var alertsTopic string
var fetchURLs []string

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch alerts.",
	Run: func(cmd *cobra.Command, args []string) {
		conf := fetch.Config{
			Brokers:     brokers,
			Topic:       topic,
			RetryTopic:  retryTopic,
			Group:       group,
			Delay:       delay,
			AlertsTopic: alertsTopic,
			FetchURLs:   fetchURLs,
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan bool)

		go func() {
			defer close(done)
			if err := fetch.Run(ctx, conf); err != nil {
				if err != context.Canceled {
					log.Fatal(err)
				}
			}
		}()

		wait := make(chan os.Signal, 1)
		signal.Notify(wait, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-wait: // Wait for SIGINT or SIGTERM
			log.Println("Signal received, terminating...")
			cancel() // Stop the processor
			<-done
		case <-done:
			cancel()
		}
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	fetchCmd.Flags().StringVarP(&group, "group", "g", "", "Group")
	fetchCmd.MarkFlagRequired("group")

	fetchCmd.Flags().StringVarP(&topic, "topic", "t", "", "Retry topic")
	fetchCmd.Flags().StringVarP(&retryTopic, "retry-topic", "r", "", "Retry topic")

	fetchCmd.Flags().IntVarP(&delay, "delay", "d", 0, "Delay, in seconds")

	fetchCmd.Flags().StringVarP(&alertsTopic, "alerts-topic", "a", "", "Alerts topic")
	fetchCmd.MarkFlagRequired("alerts-topic")

	fetchCmd.Flags().StringArrayVarP(&fetchURLs, "fetch-urls", "u", []string{}, "Fetch URLs")
	fetchCmd.MarkFlagRequired("fetch-urls")
}
