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

	// API Routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/status", getStatus)
		r.Get("/config", getConfig)
		r.Post("/config", saveConfig)
		r.Get("/state", getState)
		r.Post("/trigger", triggerRoutine)
		r.Get("/history", getHistory)
		r.Get("/downloads", getDownloads)
	})

	// Setup and start scheduler
	sched := scheduler.NewScheduler()
	sched.ScheduleFridayNightJob(func() {
		triggerEngineLogic()
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
	json.NewEncoder(w).Encode(config.GetConfig())
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
	go triggerEngineLogic()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Job triggered"})
}

func triggerEngineLogic() {
	fmt.Println("Triggering Friday Night Movie Engine...")
	cfg := config.GetConfig()
	jClient := media.NewJellyfinClient(cfg.JellyfinURL, cfg.JellyfinKey)
	tClient := discovery.NewTMDBClient(cfg.TMDBKey)
	rClient := downloader.NewClient(cfg.RadarrURL, cfg.RadarrKey)
	
	movie, err := logic.RunFridayNightRoutine(jClient, tClient, rClient)
	if err != nil {
		fmt.Printf("Error running routine: %v\n", err)
	} else if movie != nil {
		fmt.Printf("Routine finished successfully. Selected: %s\n", movie.Title)
		config.SaveState(config.AppState{
			LastMovieTitle:      movie.Title,
			LastMoviePosterPath: movie.PosterPath,
			LastMovieOverview:   movie.Overview,
			LastMovieRating:     movie.VoteAverage,
		})
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
