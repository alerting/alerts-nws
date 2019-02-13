package fetch

import (
	"context"
	"encoding/xml"
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	capxml "github.com/alerting/alerts/pkg/cap/xml"

	"github.com/alerting/alerts-naads/pkg/codec"
	"github.com/lovoo/goka"
	"github.com/lovoo/goka/kafka"
)

var notFoundError = errors.New("Not found")

type Config struct {
	Brokers    []string
	Topic      string
	RetryTopic string
	Group      string
	Delay      int

	AlertsTopic string
	FetchURLs   []string
}

func fetch(ctx context.Context, conf *Config, ref *capxml.Reference) (*capxml.Alert, error) {
	// Generate the URL
	resourceURL, err := url.Parse(ref.Identifier)

	if err != nil {
		return nil, err
	}

	for i, fetchURL := range conf.FetchURLs {
		baseURL, err := url.Parse(fetchURL)
		if err != nil {
			return nil, err
		}
		u := baseURL.ResolveReference(resourceURL)

		log.Printf("Fetching %s", u.String())
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", "ZacharySeguinAlerts/1.0 (https://alerts.zacharyseguin.ca; contact@zacharyseguin.ca)")
		req.Header.Set("Accept", "application/cap+xml")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		log.Printf("Got status code %d (%s)", res.StatusCode, ref.ID())
		if res.StatusCode != 200 {
			// If it's the last alert, and we have a 404
			if res.StatusCode == 404 && i == len(conf.FetchURLs)-1 {
				return nil, notFoundError
			}
			continue
		}

		// Parse the alert
		var alert capxml.Alert
		decoder := xml.NewDecoder(res.Body)
		err = decoder.Decode(&alert)
		if err != nil {
			return nil, err
		}
		return &alert, nil
	}
	return nil, errors.New("Unable to fetch alert")
}

func collect(ctx context.Context, conf *Config) func(ctx goka.Context, msg interface{}) {
	return func(gctx goka.Context, msg interface{}) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(conf.Delay) * time.Second):
		}

		ref := msg.(capxml.Reference)

		log.Printf("Received: %v, %v => %v", gctx.Topic(), gctx.Key(), ref)

		alert, err := fetch(ctx, conf, &ref)
		if err != nil {
			if err == notFoundError {
				log.Printf("Alert not found: %v", ref)
				return
			} else {
				time.Sleep(15 * time.Second)
				log.Println(err)

				alert, err = fetch(ctx, conf, &ref)
				if err != nil {
					log.Println(err)
					return
				}
				// TODO: Handle
			}
		}

		gctx.Emit(goka.Stream(conf.AlertsTopic), alert.ID(), alert)
	}
}

func Run(ctx context.Context, conf Config) error {
	edges := []goka.Edge{
		goka.Input(goka.Stream(conf.Topic), new(codec.Reference), collect(ctx, &conf)),
		goka.Output(goka.Stream(conf.RetryTopic), new(codec.Reference)),
		goka.Output(goka.Stream(conf.AlertsTopic), new(codec.Alert)),
	}

	kconf := kafka.NewConfig()
	// 5 MB
	kconf.Producer.MaxMessageBytes = 1024 * 1024 * 5

	g := goka.DefineGroup(goka.Group(conf.Group), edges...)
	p, err := goka.NewProcessor(conf.Brokers, g,
		goka.WithConsumerBuilder(kafka.ConsumerBuilderWithConfig(kconf)),
		goka.WithProducerBuilder(kafka.ProducerBuilderWithConfig(kconf)))
	if err != nil {
		return err
	}

	return p.Run(ctx)
}
