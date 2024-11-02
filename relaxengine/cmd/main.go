package main

import (
	"fmt"
	"time"

	gocron "github.com/go-co-op/gocron"

	config "github.com/ravikisha/relaxengine/config"
	crawler "github.com/ravikisha/relaxengine/crawler"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize the Elasticsearch client
	esClient := crawler.NewElasticsearchClient(cfg.ElasticsearchURL)

	// Initialize the crawler with the defined depth limit from config
	c := crawler.NewCrawler(cfg.DepthLimit, 5)
	seedURL := "https://vit.ac.in/" // Replace with the starting URL you want to crawl

	// Create a scheduler instance
	s := gocron.NewScheduler(time.UTC)

	// Define the crawling job to run every 30 minutes
	s.Every(30).Minutes().Do(func() {
		fmt.Printf("Starting crawl from: %s at %v\n", seedURL, time.Now())
		go func() {
			c.StartCrawling(seedURL, 0, esClient)
			fmt.Println("Crawling completed for this run.")
		}()
	})

	// Start the scheduler
	s.StartBlocking()
}
