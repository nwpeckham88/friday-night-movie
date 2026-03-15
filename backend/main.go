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
	"github.com/user/friday-night-movie/pkg/db"
	"github.com/user/friday-night-movie/pkg/notify"
)

func main() {
	if err := db.Init(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

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
		r.Get("/suggestions", getSuggestions)
		r.Get("/radarr/profiles", getRadarrProfiles)
		r.Get("/downloads", getDownloads)
		r.Post("/test-llm", testLLM)
		r.Post("/reject", rejectMovie)
		r.Post("/endorse", endorseMovie)
		r.Post("/clear-suggestion", clearSuggestion)
	})

	sched.ScheduleFridayNightJob(func() {
		triggerEngineLogic(false, true) // Auto-mode: Lucky + AutoAdd
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
	go triggerEngineLogic(false, false) // Lucky mode, NO AutoAdd
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Lucky selection triggered"})
}

func triggerSearch(w http.ResponseWriter, r *http.Request) {
	go triggerEngineLogic(true, false) // Search mode, NO AutoAdd
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
}

func rejectMovie(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TMDBID int `json:"tmdbId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	state := config.GetState()
	cfg := config.GetConfig()
	tClient := discovery.NewTMDBClient(cfg.TMDBKey)
	movie, err := tClient.GetMovie(body.TMDBID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add to rejected list
	state.RejectedMovies = append(state.RejectedMovies, fmt.Sprintf("%s (%d)", movie.Title, body.TMDBID))
	state.LastMovieTitle = "" // Clear suggestion
	state.IsSuggested = false
	config.SaveState(state)

	// Trigger profile update in background
	go func() {
		provider, _ := discovery.GetProvider(cfg.GeminiKey, cfg.LLMProvider)
		if provider != nil {
			newProfile, err := discovery.UpdateTasteProfile(provider, state.TasteProfile, []string{}, state.RejectedMovies, fmt.Sprintf("Rejected: %s", movie.Title))
			if err == nil {
				state := config.GetState()
				state.TasteProfile = newProfile
				config.SaveState(state)
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Movie rejected"})
}

func endorseMovie(w http.ResponseWriter, r *http.Request) {
	state := config.GetState()
	if state.LastMovieTitle == "" {
		http.Error(w, "No active suggestion to endorse", http.StatusBadRequest)
		return
	}

	provider, err := discovery.GetProvider(config.GetConfig().GeminiKey, config.GetConfig().LLMProvider)
	if err == nil {
		newProfile, err := discovery.UpdateTasteProfile(provider, state.TasteProfile, []string{state.LastMovieTitle}, state.RejectedMovies, fmt.Sprintf("User endorsed (Liked) but did not download: %s", state.LastMovieTitle))
		if err == nil {
			state.TasteProfile = newProfile
		}
	}

	// Move on (Clear suggestion)
	state.LastMovieTitle = ""
	state.LastMoviePosterPath = ""
	state.LastMovieOverview = ""
	state.LastMovieRating = 0
	state.LastMovieID = 0
	state.LastMovieTrailerKey = ""
	state.IsSuggested = false
	state.Status = "Movie Endorsed!"
	config.SaveState(state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Movie endorsed"})
}

func clearSuggestion(w http.ResponseWriter, r *http.Request) {
	state := config.GetState()
	state.LastMovieTitle = ""
	state.LastMoviePosterPath = ""
	state.LastMovieOverview = ""
	state.LastMovieRating = 0
	state.LastMovieID = 0
	state.LastMovieTrailerKey = ""
	state.IsSuggested = false
	config.SaveState(state)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Suggestion cleared"})
}

func triggerEngineLogic(searchOnly bool, autoAdd bool) {
	fmt.Println("Triggering Friday Night Movie Engine...")
	cfg := config.GetConfig()
	jClient := media.NewJellyfinClient(cfg.JellyfinURL, cfg.JellyfinKey)
	tClient := discovery.NewTMDBClient(cfg.TMDBKey)
	rClient := downloader.NewClient(cfg.RadarrURL, cfg.RadarrKey)

	// Set Notifier Instance
	if cfg.DiscordWebhookURL != "" {
		notify.SetNotifier(&notify.DiscordNotifier{WebhookURL: cfg.DiscordWebhookURL})
	} else {
		notify.SetNotifier(&notify.LogNotifier{})
	}

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
			state.LastMovieTrailerKey = ""
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

	if !autoAdd || searchOnly {
		// Discovery only mode (Manual roll or search)
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
			state.LastMovieTrailerKey = movie.TrailerKey
			state.Status = "Suggestion Found!"
			state.IsRunning = false
			state.IsSuggested = true
			config.SaveState(state)

			// Notify
			notify.Instance.Notify("🎲 FNM Suggested a Movie!", &discovery.TMDBMovie{
				ID:          movie.ID,
				Title:       movie.Title,
				Overview:    movie.Overview,
				PosterPath:  movie.PosterPath,
				VoteAverage: movie.VoteAverage,
				ReleaseDate: movie.ReleaseDate,
			})

			// Save to suggestions history
			var year int
			if len(movie.ReleaseDate) >= 4 {
				fmt.Sscanf(movie.ReleaseDate[:4], "%d", &year)
			}
			db.SaveSuggestion(db.DBSuggestion{
				TMDBID:     movie.ID,
				Title:      movie.Title,
				Year:       year,
				Overview:   movie.Overview,
				PosterPath: movie.PosterPath,
				Rating:     movie.VoteAverage,
				TrailerKey: movie.TrailerKey,
			})
		}
	} else {
		// Auto-add mode (Background routine)
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
			state.Status = "Movie Added to Radarr!"
			state.IsRunning = false
			state.IsSuggested = false
			config.SaveState(state)

			// Update Taste Profile on automatic pick
			go func() {
				newProfile, err := discovery.UpdateTasteProfile(provider, state.TasteProfile, []string{movie.Title}, state.RejectedMovies, fmt.Sprintf("Automatically selected and added: %s", movie.Title))
				if err == nil {
					s := config.GetState()
					s.TasteProfile = newProfile
					config.SaveState(s)
				}
			}()
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

		// Update Taste Profile
		go func() {
			provider, _ := discovery.GetProvider(cfg.GeminiKey, cfg.LLMProvider)
			if provider != nil {
				newProfile, err := discovery.UpdateTasteProfile(provider, state.TasteProfile, []string{movie.Title}, state.RejectedMovies, fmt.Sprintf("User accepted and added suggestion: %s", movie.Title))
				if err == nil {
					s := config.GetState()
					s.TasteProfile = newProfile
					config.SaveState(s)
				}
			}
		}()
	}
}


func getRadarrProfiles(w http.ResponseWriter, r *http.Request) {
	cfg := config.GetConfig()
	rClient := downloader.NewClient(cfg.RadarrURL, cfg.RadarrKey)
	profiles, err := rClient.GetQualityProfiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profiles)
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

func testLLM(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		APIKey   string `json:"apiKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	provider, err := discovery.GetProvider(body.APIKey, body.Provider)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
		return
	}

	// Simple test call with empty history/context
	suggestions, err := provider.DiscoverMovie([]string{}, "", []string{}, []string{}, func(msg string) {})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Successfully connected to %s!", body.Provider),
		"movie":   suggestions[0].Title,
	})
}

func getSuggestions(w http.ResponseWriter, r *http.Request) {
	suggestions, err := db.GetSuggestions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
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
