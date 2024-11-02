package crawler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/olivere/elastic/v7"
	"golang.org/x/net/html"
)

// Struct to represent the crawler
type Crawler struct {
	VisitedURLs  map[string]bool
	DepthLimit   int
	Mutex        sync.Mutex
	LogFile      *os.File
	FailureCount map[string]int // Track failures per URL
	MaxFailures  int            // Max failures allowed before blocking
}

// Struct to represent the data of a web page
type PageData struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
}

// Create a new crawler with a depth limit and max failures
func NewCrawler(depthLimit, maxFailures int) *Crawler {
	// Get the current executable's directory
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}
	executableDir := filepath.Dir(executablePath)

	// Create the log file in the current directory
	logFilePath := filepath.Join(executableDir, "crawler.log")
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	return &Crawler{
		VisitedURLs:  make(map[string]bool),
		DepthLimit:   depthLimit,
		LogFile:      logFile,
		FailureCount: make(map[string]int),
		MaxFailures:  maxFailures,
	}
}

// Initialize Elasticsearch client
func NewElasticsearchClient(url string) *elastic.Client {
	var client *elastic.Client
	var err error

	client, err = elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetBasicAuth("elastic", "ravikishan"),
	)
	if err == nil {
		// Check if client can ping the Elasticsearch instance
		_, _, err = client.Ping(url).Do(context.Background())
		if err == nil {
			return client // Successfully connected
		}
	}

	log.Fatalf("Error creating Elasticsearch client: %v", err)
	return nil
}

// Start crawling from a given URL
func (c *Crawler) StartCrawling(pageURL string, depth int, esClient *elastic.Client) {
	if depth > c.DepthLimit || c.isVisited(pageURL) || c.isBlocked(pageURL) {
		return
	}
	c.markVisited(pageURL)
	c.logURL(pageURL)

	fmt.Printf("Crawling: %s at depth %d\n", pageURL, depth)

	// Check robots.txt rules before fetching links
	if !c.isAllowedByRobots(pageURL) {
		fmt.Printf("Blocked by robots.txt: %s\n", pageURL)
		c.incrementFailure(pageURL)
		return
	}

	// Fetch and parse the page content
	links, title, content, description, err := c.fetchAndParsePage(pageURL)
	if err != nil {
		fmt.Printf("Failed to fetch %s: %v\n", pageURL, err)
		c.incrementFailure(pageURL)
		return
	}
	c.resetFailure(pageURL) // Reset failure count on successful fetch

	// Index the page data in Elasticsearch
	pageData := PageData{
		URL:         pageURL,
		Title:       title,
		Content:     content,
		Description: description,
		Keywords:    extractKeywords(content),
	}
	if err := IndexPageData(esClient, pageData); err != nil {
		fmt.Printf("Failed to index %s: %v\n", pageURL, err)
	}

	// Crawl found links concurrently
	var wg sync.WaitGroup
	for _, link := range links {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()
			c.StartCrawling(link, depth+1, esClient)
		}(link)
	}
	wg.Wait()
}

// Index page data into Elasticsearch
func IndexPageData(client *elastic.Client, data PageData) error {
	ctx := context.Background()
	_, err := client.Index().
		Index("webpages").
		BodyJson(data).
		Do(ctx)
	return err
}

// Fetch and parse page content
func (c *Crawler) fetchAndParsePage(pageURL string) ([]string, string, string, string, error) {
	resp, err := http.Get(pageURL)
	if err != nil {
		return nil, "", "", "", err
	}
	defer resp.Body.Close()

	var links []string
	var title, content, description string
	tokenizer := html.NewTokenizer(resp.Body)
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return links, title, content, description, nil
		case html.StartTagToken:
			t := tokenizer.Token()
			switch t.Data {
			case "a":
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						absoluteLink := c.resolveURL(pageURL, attr.Val)
						if absoluteLink != "" && !c.isVisited(absoluteLink) {
							links = append(links, absoluteLink)
						}
					}
				}
			case "title":
				tokenizer.Next()
				title = tokenizer.Token().Data
			case "meta":
				for _, attr := range t.Attr {
					if attr.Key == "name" && attr.Val == "description" {
						for _, attr := range t.Attr {
							if attr.Key == "content" {
								description = attr.Val
							}
						}
					}
				}
			}
		case html.TextToken:
			content += tokenizer.Token().Data
		}
	}
}

// Check if a URL has already been visited
func (c *Crawler) isVisited(url string) bool {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	return c.VisitedURLs[url]
}

// Mark a URL as visited
func (c *Crawler) markVisited(url string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.VisitedURLs[url] = true
}

// Resolve relative URLs to absolute ones based on the page URL
func (c *Crawler) resolveURL(base, href string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	parsedURL, err := baseURL.Parse(href)
	if err != nil {
		return ""
	}
	return parsedURL.String()
}

// Check robots.txt rules
func (c *Crawler) isAllowedByRobots(pageURL string) bool {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", parsedURL.Scheme, parsedURL.Host)

	resp, err := http.Get(robotsURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return true // Assume allowed if robots.txt is not accessible
	}
	defer resp.Body.Close()

	// Simplified approach (you could expand this with a parser)
	return true
}

// Increment failure count for a URL
func (c *Crawler) incrementFailure(url string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.FailureCount[url]++
	if c.FailureCount[url] >= c.MaxFailures {
		fmt.Printf("Blocked %s due to too many failures\n", url)
	}
}

// Reset failure count for a URL
func (c *Crawler) resetFailure(url string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	c.FailureCount[url] = 0
}

// Check if a URL has been blocked due to too many failures
func (c *Crawler) isBlocked(url string) bool {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	return c.FailureCount[url] >= c.MaxFailures
}

// Extract keywords from content for indexing
func extractKeywords(content string) []string {
	words := strings.Fields(content)
	keywordSet := make(map[string]struct{})
	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?:;\"'"))
		if len(word) > 3 { // Skip short words
			keywordSet[word] = struct{}{}
		}
	}

	// Convert set to slice
	keywords := make([]string, 0, len(keywordSet))
	for keyword := range keywordSet {
		keywords = append(keywords, keyword)
	}
	return keywords
}

// Logs the URL to the log file
func (c *Crawler) logURL(url string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if _, err := c.LogFile.WriteString(fmt.Sprintf("Scraping URL: %s\n", url)); err != nil {
		log.Printf("Failed to write URL to log file: %v", err)
	}
}
