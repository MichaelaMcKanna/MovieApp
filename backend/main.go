package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

type Movie struct {
	ID              string          `json:"id"`
	TitleText       string          `json:"titleText"`
	TitleType       string          `json:"titleType"`
	ReleaseYear     int             `json:"releaseYear"`
	ReleaseDate     string          `json:"releaseDate"`
	Genres          []string        `json:"genres"`
	PrimaryImage    *PrimaryImage   `json:"primaryImage,omitempty"`
	RatingsSummary  *RatingsSummary `json:"ratingsSummary,omitempty"`
	MainActors      []Actor         `json:"mainActors"`
	StreamingOptions []StreamingOption `json:"streamingOptions,omitempty"` 
}

type PrimaryImage struct {
	URL string `json:"url"`
}

type RatingsSummary struct {
	AggregateRating float64 `json:"aggregateRating"`
	VoteCount       int     `json:"voteCount"`
}

type Actor struct {
	Name string `json:"name"`
}

type MovieAPIResponse struct {
	Results []Movie `json:"results"`
}

type StreamingOption struct {
	Service string `json:"service"`
	URL     string `json:"url"`
	Price   string `json:"price,omitempty"`
	Quality string `json:"quality,omitempty"`
}

var movieAPIBaseURL string
var streamingAPIBaseURL string
var movieAPIHost string
var streamingAPIHost string
var rapidAPIKey string

// Simple in-memory cache
var movieCache = make(map[string]Movie)
var cacheMutex sync.RWMutex

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	movieAPIBaseURL = os.Getenv("MOVIE_API_URL")
	streamingAPIBaseURL = os.Getenv("STREAMING_API_URL")
	movieAPIHost = os.Getenv("MOVIE_API_HOST")
	streamingAPIHost = os.Getenv("STREAMING_API_HOST")
	rapidAPIKey = os.Getenv("RAPID_API_KEY")
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to the Movie API. Use /movies to get a list of movies or /movies/{id} to get details of a specific movie."))
	}).Methods("GET")

	r.HandleFunc("/movies", getMovies).Methods("GET")
	r.HandleFunc("/movies/{id}", getMovie).Methods("GET")

	r.Use(loggingMiddleware)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func getMovies(w http.ResponseWriter, r *http.Request) {
	// Assume we're receiving a list of movie IDs via a query parameter
	movieIDs := r.URL.Query()["id"]

	var movies []Movie
	for _, id := range movieIDs {
		// Check cache first
		cacheMutex.RLock()
		movie, found := movieCache[id]
		cacheMutex.RUnlock()

		if !found {
			// Fetch movie data
			movie = fetchMovieData(id)
			// Fetch streaming options
			movie.StreamingOptions = fetchStreamingOptions(id)
			// Cache the result
			cacheMutex.Lock()
			movieCache[id] = movie
			cacheMutex.Unlock()
		}

		movies = append(movies, movie)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movies)
}

func getMovie(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Check cache first
	cacheMutex.RLock()
	movie, found := movieCache[id]
	cacheMutex.RUnlock()

	if !found {
		// Fetch movie data
		movie = fetchMovieData(id)
		// Fetch streaming options
		movie.StreamingOptions = fetchStreamingOptions(id)
		// Cache the result
		cacheMutex.Lock()
		movieCache[id] = movie
		cacheMutex.Unlock()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movie)
}

func fetchMovieData(id string) Movie {
    url := fmt.Sprintf("%s/titles/%s?info=base_info", movieAPIBaseURL, id)
    
    log.Printf("Fetching movie data from URL: %s", url)
    
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Add("x-rapidapi-host", movieAPIHost)
    req.Header.Add("x-rapidapi-key", rapidAPIKey)
    
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("Error fetching movie data for %s: %v", id, err)
        return Movie{}
    }
    defer res.Body.Close()
    
    body, _ := io.ReadAll(res.Body)
    log.Printf("Received movie data response: %s", string(body))
    
    var movieResp struct {
        Results Movie `json:"results"`
    }
    err = json.Unmarshal(body, &movieResp)
    if err != nil {
        log.Printf("Error parsing movie data for %s: %v", id, err)
        return Movie{}
    }
    
    movie := movieResp.Results
    movie.MainActors = getMainActors(id)
    return movie
}

func fetchStreamingOptions(id string) []StreamingOption {
    url := fmt.Sprintf("%s/shows/%s", streamingAPIBaseURL, id)
    
    log.Printf("Fetching streaming options from URL: %s", url)
    
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Add("x-rapidapi-host", streamingAPIHost)
    req.Header.Add("x-rapidapi-key", rapidAPIKey)
    
    res, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("Error fetching streaming options for %s: %v", id, err)
        return nil
    }
    defer res.Body.Close()
    
    body, _ := io.ReadAll(res.Body)
    log.Printf("Received streaming options response: %s", string(body))
    
    var streamingResp struct {
        Results []StreamingOption `json:"results"`
    }
    err = json.Unmarshal(body, &streamingResp)
    if err != nil {
        log.Printf("Error parsing streaming options for %s: %v", id, err)
        return nil
    }
    
    return streamingResp.Results
}


func getMainActors(id string) []Actor {
	url := fmt.Sprintf("%s/titles/%s/main_actors", movieAPIBaseURL, id)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("x-rapidapi-host", movieAPIHost)
	req.Header.Add("x-rapidapi-key", rapidAPIKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error fetching main actors for %s: %v", id, err)
		return nil
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	var actorsResp struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	err = json.Unmarshal(body, &actorsResp)
	if err != nil {
		log.Printf("Error parsing main actors for %s: %v", id, err)
		return nil
	}

	var actors []Actor
	for _, result := range actorsResp.Results {
		actors = append(actors, Actor{Name: result.Name})
	}

	return actors
}
