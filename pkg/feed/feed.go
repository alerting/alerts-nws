package feed

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/alerting/alerts/pkg/alerts"
	"github.com/alerting/alerts/pkg/cap"
	"github.com/golang/protobuf/ptypes"
	"github.com/lovoo/goka"

	"github.com/alerting/alerts-naads/pkg/codec"
	capxml "github.com/alerting/alerts/pkg/cap/xml"
)

// Config is the configuration for the feed processor.
type Config struct {
	// Kafka broker(s)
	Brokers []string

	// For alerts not in the system, place a fetch request into
	// this topic.
	FetchTopic string

	// Feed URL.
	FeedURL *url.URL

	// Update interval.
	UpdateInterval time.Duration

	// Alerts service.
	AlertsService alerts.AlertsServiceClient
}

// Structs to parse API
type graph struct {
	ID     string `json:"id"`
	Sender string `json:"sender"`
	Sent   string `json:"sent"`
}

type alertsResponse struct {
	Graph []*graph `json:"@graph"`
}

func GetAlertReferencesFromFeed(feed *url.URL) ([]*capxml.Reference, error) {
	req, err := http.NewRequest(http.MethodGet, feed.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ZacharySeguinAlerts/1.0 (https://alerts.zacharyseguin.ca; contact@zacharyseguin.ca)")
	req.Header.Set("Accept", "application/ld+json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var alerts alertsResponse
	err = json.Unmarshal(body, &alerts)
	if err != nil {
		return nil, err
	}

	var references = make([]*capxml.Reference, len(alerts.Graph))
	for i, alert := range alerts.Graph {
		sentTime, err := time.Parse(capxml.TimeFormat, alert.Sent)
		if err != nil {
			return nil, err
		}
		sent := capxml.Time{Time: sentTime}

		ref := &capxml.Reference{
			Identifier: alert.ID,
			Sender:     alert.Sender,
			Sent:       sent,
		}

		references[i] = ref
	}

	return references, nil
}

func loop(ctx context.Context, conf Config, emitter *goka.Emitter, fetchView *goka.View) error {
	for {
		log.Println("Fetching alert references from feed")

		// TODO: pagination?
		references, err := GetAlertReferencesFromFeed(conf.FeedURL)
		if err != nil {
			return err
		}

		// Check if the alert exists in the system, and if not,
		// schedule it for fetching.
		for _, xmlReference := range references {
			sent, _ := ptypes.TimestampProto(xmlReference.Sent.Time)
			ref := &cap.Reference{
				Sender:     xmlReference.Sender,
				Identifier: xmlReference.Identifier,
				Sent:       sent,
			}

			has, err := conf.AlertsService.Has(ctx, ref)
			if err != nil {
				return err
			}

			inTable, err := fetchView.Has(xmlReference.ID())
			if err != nil {
				return err
			}

			if !has.Result && !inTable {
				log.Printf("Requesting %v", ref)
				_, err := emitter.Emit(xmlReference.ID(), xmlReference)
				if err != nil {
					return err
				}
			}
		}

		log.Println("Sleeping for", conf.UpdateInterval)

		// Wait update interval (or until context is cancelled)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(conf.UpdateInterval):
		}
	}
}

// Run runs the feed processor, checking for new alerts and
// queing them for fetching.
func Run(ctx context.Context, conf Config) error {
	if conf.AlertsService == nil {
		return errors.New("No alerts service provided")
	}

	// Initialize goka
	emitter, err := goka.NewEmitter(conf.Brokers, goka.Stream(conf.FetchTopic), new(codec.Reference))
	if err != nil {
		return err
	}
	defer emitter.Finish()

	fetchView, err := goka.NewView(conf.Brokers, goka.Table(conf.FetchTopic), new(codec.Reference))
	if err != nil {
		return err
	}

	go func() {
		err = fetchView.Run(ctx)
		if err != nil {
			// TODO: not panic!
			panic(err)
		}
	}()

	return loop(ctx, conf, emitter, fetchView)
}
