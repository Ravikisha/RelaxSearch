package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux" // Gorilla Mux for routing

	"github.com/ravikisha/relaxweb/config"
	"github.com/ravikisha/relaxweb/search"
)

func main() {
	// Load configuration for Elasticsearch URL
	cfg := config.LoadConfig()

	// Set up the router and define the search endpoint
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to RelaxWeb! Please use the /search endpoint to search for articles.")
	}).Methods("GET")

	r.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		handleSearch(w, r, cfg.ElasticsearchURL)
	}).Methods("GET")

	// Start the server
	port := ":7000"
	fmt.Printf("Starting search API server on port %s\n", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleSearch handles the search functionality as an HTTP GET request
func handleSearch(w http.ResponseWriter, r *http.Request, elasticsearchURL string) {
	// Parse query parameters
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		http.Error(w, "Missing 'keyword' parameter", http.StatusBadRequest)
		return
	}

	// Pagination parameters (default: from = 0, size = 10)
	from, _ := strconv.Atoi(r.URL.Query().Get("from"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size == 0 {
		size = 10
	}

	// Date range parameters (optional)
	dateRangeStart := r.URL.Query().Get("start")
	dateRangeEnd := r.URL.Query().Get("end")
	if dateRangeStart == "" {
		dateRangeStart = search.FormatDateForElasticsearch(time.Now().AddDate(0, -1, 0)) // Default: 1 month ago
	}
	if dateRangeEnd == "" {
		dateRangeEnd = search.FormatDateForElasticsearch(time.Now())
	}

	// Call the search function
	results, err := search.SearchByKeyword(elasticsearchURL, keyword, from, size, dateRangeStart, dateRangeEnd)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error executing search: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers and return JSON results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
	}
}
