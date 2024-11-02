package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SearchResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	// Content            string   `json:"content"`
	HighlightedContent []string `json:"highlightedContent,omitempty"`
}

type ElasticsearchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source    SearchResult        `json:"_source"`
			Highlight map[string][]string `json:"highlight"`
		} `json:"hits"`
	} `json:"hits"`
}

// SearchByKeyword performs a full-text search with pagination, date filtering, fuzzy matching, and content highlighting.
func SearchByKeyword(elasticsearchURL, keyword string, from, size int, dateRangeStart, dateRangeEnd string) ([]SearchResult, error) {
	// Define the search query for keyword matching and additional indexing on "content" field for better relevance
	query := map[string]interface{}{
		"from": from,
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"multi_match": map[string]interface{}{
							"query":     keyword,
							"fields":    []string{"title^3", "content^2"}, // Adding weights to title and content
							"fuzziness": "AUTO",                           // Fuzzy matching to handle minor typos in search
						},
					},
					// {
					// 	"range": map[string]interface{}{
					// 		"timestamp": map[string]interface{}{
					// 			"gte":    dateRangeStart,
					// 			"lte":    dateRangeEnd,
					// 			"format": "strict_date_optional_time||epoch_millis",
					// 		},
					// 	},
					// },
				},
			},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"content": map[string]interface{}{
					"fragment_size":       150, // Limit fragment size for readability in highlighted snippets
					"number_of_fragments": 3,
				},
			},
		},
	}

	// Convert query to JSON
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, fmt.Errorf("failed to encode query: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/webpages/_search", elasticsearchURL), &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("elastic", "ravikishan")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Elasticsearch returned error status: %d", resp.StatusCode)
	}

	// Decode the response
	var esResp ElasticsearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract results
	results := make([]SearchResult, len(esResp.Hits.Hits))
	for i, hit := range esResp.Hits.Hits {
		results[i] = hit.Source
		if highlight, exists := hit.Highlight["content"]; exists {
			results[i].HighlightedContent = highlight
		}
	}

	return results, nil
}

// Helper function to format time for Elasticsearch date query
func FormatDateForElasticsearch(date time.Time) string {
	return date.Format("2006-01-02T15:04:05Z")
}
