package cmd

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alerting/alerts-nws/pkg/feed"
	"github.com/spf13/cobra"
)

var feedURL string
var updateInterval time.Duration

// feedCmd represents the feed command
var feedCmd = &cobra.Command{
	Use:   "feed",
	Short: "Check NWS feed for alerts.",
	Run: func(cmd *cobra.Command, args []string) {
		url, err := url.Parse(feedURL)
		if err != nil {
			log.Fatal(err)
		}

		conf := feed.Config{
			Brokers:        brokers,
			FetchTopic:     fetchTopic,
			FeedURL:        url,
			UpdateInterval: updateInterval,
			AlertsService:  alertsService,
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan bool)

		go func() {
			defer close(done)
			if err := feed.Run(ctx, conf); err != nil {
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
	rootCmd.AddCommand(feedCmd)

	feedCmd.Flags().StringVarP(&feedURL, "feed-url", "u", "", "Feed URL")
	feedCmd.MarkFlagRequired("feed-url")

	feedCmd.Flags().DurationVarP(&updateInterval, "update-interval", "i", 5*time.Minute, "Duration between checks of the feed")

	// We need the alerts service
	feedCmd.MarkFlagRequired("alerts-service")
}
