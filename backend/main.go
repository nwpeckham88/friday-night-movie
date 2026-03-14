package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	
	"github.com/user/friday-night-movie/pkg/config"
	"github.com/user/friday-night-movie/pkg/discovery"
	"github.com/user/friday-night-movie/pkg/downloader"
	"github.com/user/friday-night-movie/pkg/logic"
	"github.com/user/friday-night-movie/pkg/media"
	"github.com/user/friday-night-movie/pkg/scheduler"
)

func main() {
	if err := config.Load(); err != nil {
		fmt.Printf("Warning: Failed to load config: %v\n", err)
	}

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API Routes (moved after scheduler setup)

	// Setup and start scheduler
	sched := scheduler.NewScheduler()
	
	// API Routes - moved after scheduler init so it can use 'sched'
	r.Route("/api", func(r chi.Router) {
		r.Get("/status", getStatus)
		r.Get("/config", getConfig)
		r.Post("/config", saveConfig)
		r.Get("/state", getState)
		r.Post("/trigger", triggerRoutine)
		r.Post("/search", triggerSearch)
		r.Post("/add", addManual)
		r.Get("/schedule", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"nextRun": sched.NextRun()})
		})
		r.Get("/history", getHistory)
		r.Get("/downloads", getDownloads)
	})

	sched.ScheduleFridayNightJob(func() {
		triggerEngineLogic(false) // Auto-mode
	})
	sched.Start()
	defer sched.Stop()

	// Serve static files (Frontend)
	workDir, _ := os.Getwd()
	frontendDir := filepath.Join(filepath.Dir(workDir), "frontend")
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		// Fallback if running from root dir
		frontendDir = filepath.Join(workDir, "frontend")
	}

	FileServer(r, "/", http.Dir(frontendDir))

	port := 8080
	fmt.Printf("Starting Server on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), r))
}

func getStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "API is running"})
}

func getConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.GetFrontendConfig())
}

func saveConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.AppConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.SaveConfig(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Printf("Received and saved new config!\n")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func getState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config.GetState())
}

func triggerRoutine(w http.ResponseWriter, r *http.Request) {
	go triggerEngineLogic(false) // Lucky mode
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Lucky selection triggered"})
}

func triggerSearch(w http.ResponseWriter, r *http.Request) {
	go triggerEngineLogic(true) // Search mode
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Search triggered"})
}

func addManual(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TMDBID int `json:"tmdbId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go triggerAddLogic(body.TMDBID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Addition triggered"})
}

func triggerEngineLogic(searchOnly bool) {
	fmt.Println("Triggering Friday Night Movie Engine...")
	cfg := config.GetConfig()
	jClient := media.NewJellyfinClient(cfg.JellyfinURL, cfg.JellyfinKey)
	tClient := discovery.NewTMDBClient(cfg.TMDBKey)
	rClient := downloader.NewClient(cfg.RadarrURL, cfg.RadarrKey)

	updateStatus := func(msg string, running bool) {
		fmt.Printf("[Status] %s (Running: %v)\n", msg, running)
		state := config.GetState()
		state.Status = msg
		state.IsRunning = running
		// If starting, clear last movie info to show we are searching
		if running && msg == "Fetching existing library..." {
			state.LastMovieTitle = ""
			state.LastMoviePosterPath = ""
			state.LastMovieOverview = ""
			state.LastMovieRating = 0
			state.LastMovieID = 0
			state.IsSuggested = false
		}
		config.SaveState(state)
	}

	// Initialize Discovery Provider
	provider, err := discovery.GetProvider(cfg.GeminiKey, cfg.LLMProvider)
	if err != nil {
		fmt.Printf("Error initializing provider: %v\n", err)
		updateStatus(fmt.Sprintf("Provider Error: %v", err), false)
		return
	}

	if searchOnly {
		movie, err := logic.DiscoverNewMovie(cfg, jClient, rClient, tClient, provider, updateStatus, true)
		if err != nil {
			fmt.Printf("Error searching: %v\n", err)
			updateStatus(fmt.Sprintf("Error: %v", err), false)
		} else if movie != nil {
			updateStatus("Found a suggestion!", false)
			state := config.GetState()
			state.LastMovieTitle = movie.Title
			state.LastMoviePosterPath = movie.PosterPath
			state.LastMovieOverview = movie.Overview
			state.LastMovieRating = movie.VoteAverage
			state.LastMovieID = movie.ID
			state.Status = "Suggestion Found!"
			state.IsRunning = false
			state.IsSuggested = true
			config.SaveState(state)
		}
	} else {
		movie, err := logic.RunFridayNightRoutine(cfg, jClient, tClient, rClient, provider, updateStatus)
		if err != nil {
			fmt.Printf("Error running routine: %v\n", err)
			updateStatus(fmt.Sprintf("Error: %v", err), false)
		} else if movie != nil {
			fmt.Printf("Routine finished successfully. Selected: %s\n", movie.Title)
			state := config.GetState()
			state.LastMovieTitle = movie.Title
			state.LastMoviePosterPath = movie.PosterPath
			state.LastMovieOverview = movie.Overview
			state.LastMovieRating = movie.VoteAverage
			state.LastMovieID = movie.ID
			state.Status = "Movie Added!"
			state.IsRunning = false
			state.IsSuggested = false
			config.SaveState(state)
		}
	}
}

func triggerAddLogic(tmdbId int) {
	cfg := config.GetConfig()
	rClient := downloader.NewClient(cfg.RadarrURL, cfg.RadarrKey)

	updateStatus := func(msg string, running bool) {
		state := config.GetState()
		state.Status = msg
		state.IsRunning = running
		config.SaveState(state)
	}

	updateStatus("Fetching movie details from TMDB...", true)
	tClient := discovery.NewTMDBClient(cfg.TMDBKey)
	movie, err := tClient.GetMovie(tmdbId)
	if err != nil {
		fmt.Printf("Error fetching movie from TMDB: %v\n", err)
		updateStatus(fmt.Sprintf("TMDB Error: %v", err), false)
		return
	}

	if err := logic.AddMovieToRadarr(movie, rClient, updateStatus); err != nil {
		fmt.Printf("Error adding manually: %v\n", err)
		updateStatus(fmt.Sprintf("Error: %v", err), false)
	} else {
		state := config.GetState()
		state.Status = "Movie Added!"
		state.IsRunning = false
		state.IsSuggested = false
		config.SaveState(state)
	}
}


func getHistory(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	if cfg.JellyfinURL == "" || cfg.JellyfinKey == "" {
		http.Error(w, "Jellyfin not configured", http.StatusBadRequest)
		return
	}
	jClient := media.NewJellyfinClient(cfg.JellyfinURL, cfg.JellyfinKey)
	
	// Just fetch movies as pseudo-history for the dashboard
	movies, err := jClient.GetMovies("")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movies)
}

func getDownloads(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	if cfg.RadarrURL == "" || cfg.RadarrKey == "" {
		http.Error(w, "Radarr not configured", http.StatusBadRequest)
		return
	}
	rClient := downloader.NewClient(cfg.RadarrURL, cfg.RadarrKey)
	
	queue, err := rClient.GetQueue()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(queue)
}

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
