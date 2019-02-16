// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/alerting/alerts-nws/pkg/consume"
	capxml "github.com/alerting/alerts/pkg/cap/xml"
	shp "github.com/jonas-p/go-shp"
	"github.com/spf13/cobra"
)

var polygonsUGCC string
var polygonsUGCZ string

var formatCounty = "C"
var formatPublicZone = "Z"

var system string

func getPolygons(filename, format string) (map[string]*capxml.Polygon, error) {
	// Load shapefiles
	reader, err := shp.OpenZip(filename)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	polygons := make(map[string]*capxml.Polygon)
	for reader.Next() {
		_, shape := reader.Shape()
		pshape := shape.(*shp.Polygon)

		polygon := &capxml.Polygon{
			Type:        "Polygon",
			Coordinates: make([][][]float64, 1),
		}
		polygon.Coordinates[0] = make([][]float64, pshape.NumPoints)

		for i, coord := range pshape.Points {
			polygon.Coordinates[0][pshape.NumPoints-int32(i)-1] = []float64{coord.X, coord.Y}
		}

		// STATE, TYPE, ZONE
		// https://www.weather.gov/media/alert/CAP_v12_guide_05-16-2017.pdf
		// https://www.nws.noaa.gov/emwin/winugc.htm
		polygons[reader.Attribute(0)+format+reader.Attribute(4)] = polygon
	}

	return polygons, nil
}

// consumeCmd represents the consume command
var consumeCmd = &cobra.Command{
	Use:   "consume",
	Short: "Consume alerts",
	Run: func(cmd *cobra.Command, args []string) {
		polygons := make(map[string]*capxml.Polygon)

		if polygonsUGCC != "" {
			log.Println("Loading UGC-C polygons...")
			countyPolygons, err := getPolygons(polygonsUGCC, formatCounty)
			if err != nil {
				log.Fatal(err)
			}

			// Merge into 1
			for k, polygon := range countyPolygons {
				polygons[k] = polygon
			}
		}

		if polygonsUGCZ != "" {
			log.Println("Loading UGC-Z polygons...")
			publicZonePolygons, err := getPolygons(polygonsUGCZ, formatPublicZone)
			if err != nil {
				log.Fatal(err)
			}

			// Merge into 1
			for k, polygon := range publicZonePolygons {
				polygons[k] = polygon
			}
		}

		log.Println("Done loading polygons")

		// Generate config.
		conf := consume.Config{
			Brokers:       brokers,
			Topic:         topic,
			RetryTopic:    retryTopic,
			Group:         group,
			Delay:         delay,
			AlertsService: alertsService,
			FetchTopic:    fetchTopic,
			Polygons:      polygons,
			System:        system,
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan bool)
		go func() {
			defer close(done)
			if err := consume.Run(ctx, conf); err != nil {
				if err != context.Canceled {
					log.Fatal(err)
				}
			}
		}()

		wait := make(chan os.Signal, 1)
		signal.Notify(wait, syscall.SIGINT, syscall.SIGTERM)
		<-wait // Wait for SIGINT or SIGTERM
		log.Println("Signal received, terminating...")
		cancel() // Stop the processor
		<-done
	},
}

func init() {
	rootCmd.AddCommand(consumeCmd)

	consumeCmd.Flags().StringVarP(&group, "group", "g", "", "Group")
	consumeCmd.MarkFlagRequired("group")

	consumeCmd.Flags().StringVarP(&topic, "topic", "t", "", "Retry topic")
	consumeCmd.MarkFlagRequired("topic")

	consumeCmd.Flags().StringVarP(&retryTopic, "retry-topic", "r", "", "Retry topic")

	consumeCmd.Flags().IntVarP(&delay, "delay", "d", 0, "Delay, in seconds")

	consumeCmd.Flags().StringVarP(&fetchTopic, "fetch-topic", "f", "", "Alerts topic")
	consumeCmd.MarkFlagRequired("fetch-topic")

	consumeCmd.Flags().StringVar(&polygonsUGCC, "ugc-c", "polygons/ugc-c.zip", "UGC-C polygons")
	consumeCmd.Flags().StringVar(&polygonsUGCZ, "ugc-z", "polygons/ugc-z.zip", "UGC-Z polygons")

	// We need the alerts service
	consumeCmd.MarkFlagRequired("alerts-service")

	consumeCmd.Flags().StringVarP(&system, "system", "s", "nws", "System name")
}
