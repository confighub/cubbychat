package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Version information set at build time
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var (
	ollamaURL      string
	ollamaEnabled  bool                         // Whether Ollama is expected to be available
	ollamaModel    string                       // Will be dynamically retrieved
	modelReady     atomic.Bool                  // Thread-safe flag for model readiness
	modelStatus    string      = "initializing" // Current model status
	modelNeverReady atomic.Bool                 // Flag for when AI is permanently unavailable
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaStreamResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type ChatMessage struct {
	ID        int       `json:"id"`
	Sender    string    `json:"sender"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Ollama API response structures
type OllamaModel struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	Details    struct {
		Format            string   `json:"format"`
		Family            string   `json:"family"`
		Families          []string `json:"families"`
		ParameterSize     string   `json:"parameter_size"`
		QuantizationLevel string   `json:"quantization_level"`
	} `json:"details"`
}

type OllamaModelsResponse struct {
	Models []OllamaModel `json:"models"`
}

var db *pgxpool.Pool

// Connect to PostgreSQL
func initDB() {
	var err error
	db, err = pgxpool.New(context.Background(), "postgres://admin:password@postgres:5432/chatdb")
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	log.Println("Connected to PostgreSQL")
	// Create `chat_history` table if it doesn't exist
	createTable()
}

// Create `chat_history` table if it doesn't exist
func createTable() {
	query := `
		CREATE TABLE IF NOT EXISTS chat_history (
			id SERIAL PRIMARY KEY,
			sender TEXT NOT NULL,
			message TEXT NOT NULL,
			timestamp TIMESTAMPTZ DEFAULT NOW()
		);
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use Exec with pgxpool
	_, err := db.Exec(ctx, query)
	if err != nil {
		log.Fatal("❌ Failed to create table:", err)
	}
	log.Println("✅ Table chat_history is ready")
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// Handler to fetch chat history
func getChatHistory(w http.ResponseWriter, r *http.Request) {

	rows, err := db.Query(context.Background(),
		"SELECT id, sender, message, timestamp FROM chat_history ORDER BY timestamp ASC")
	if err != nil {
		http.Error(w, "Failed to fetch chat history", http.StatusInternalServerError)
		log.Println("Error fetching chat history:", err)
		return
	}
	defer rows.Close()

	var history []ChatMessage
	for rows.Next() {
		var msg ChatMessage
		if err := rows.Scan(&msg.ID, &msg.Sender, &msg.Message, &msg.Timestamp); err != nil {
			http.Error(w, "Error processing chat history", http.StatusInternalServerError)
			log.Println("Error scanning chat history:", err)
			return
		}
		history = append(history, msg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// Store message in database
func saveMessage(sender, message string) {
	log.Printf("saving message to database: %s", message)
	_, err := db.Exec(context.Background(),
		"INSERT INTO chat_history (sender, message) VALUES ($1, $2)", sender, message)
	if err != nil {
		log.Println("Error saving message:", err)
	}
}

// Stream response from Ollama
func streamOllamaResponse(conn *websocket.Conn, prompt string) {
	client := resty.New()
	ollamaGenerateURL := fmt.Sprintf("%s/api/generate", ollamaURL)

	request := OllamaRequest{
		Model:  ollamaModel, // Use the dynamically retrieved model
		Prompt: prompt,
		Stream: true,
	}

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(request).
		SetDoNotParseResponse(true).
		Post(ollamaGenerateURL)

	if err != nil {
		log.Println("Error connecting to Ollama:", err)
		conn.WriteMessage(websocket.TextMessage, []byte("Error processing request"))
		return
	}
	defer resp.RawBody().Close()

	scanner := bufio.NewScanner(resp.RawBody())
	var fullResponse string
	for scanner.Scan() {
		var result OllamaStreamResponse
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			log.Println("Error parsing Ollama response:", err)
			continue
		}

		// Send each token to WebSocket client
		if err := conn.WriteMessage(websocket.TextMessage, []byte(result.Response)); err != nil {
			log.Println("Error sending message:", err)
			break
		}

		fullResponse += result.Response

		if result.Done {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Println("Error reading Ollama stream:", err)
	}

	// Save AI response to database
	saveMessage("AI", fullResponse)
}

// Funny waiting messages for when model is loading
var waitingMessages = []string{
	"🦥 Hold on! The AI is still having its morning coffee...",
	"🚀 The model is warming up its neural networks...",
	"🧠 Please wait, the AI is doing some mental push-ups...",
	"🎮 The AI is still loading, maybe it's stuck on a loading screen?",
	"☕ Brewing intelligence... This may take a moment!",
	"🏃 The neurons are still jogging to their positions...",
	"📚 The AI is speed-reading the entire internet, be right with you!",
	"🧘 The model is meditating to achieve consciousness...",
	"🔌 Still downloading wisdom from the cloud...",
	"🎪 The AI circus is still setting up its tent!",
}

// Funny messages for when AI is not available at all
var noAIMessages = []string{
	"🤖 Sorry, our AI took the day off. It's probably at the beach somewhere...",
	"🎭 The AI is unavailable. It's currently pursuing its dream of becoming a Broadway star.",
	"🏖️ AI.exe not found. Did you check if it went on vacation?",
	"🎨 No AI here! The silicon brain decided to become an artist instead.",
	"🚫 AI is MIA. Last seen contemplating the meaning of consciousness.",
	"🎪 The AI has left the building. Elvis style.",
	"🌙 AI is offline. Probably dreaming of electric sheep.",
	"📵 No AI signal detected. Maybe it's in airplane mode?",
	"🎓 The AI is unavailable - it went back to school to study philosophy.",
	"🧳 AI is out of office. Return date: undefined.",
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade WebSocket connection:", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket connected")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		log.Printf("Received message: %s\n", msg)

		// Save user message to database
		saveMessage("User", string(msg))

		// Check if AI is permanently unavailable
		if modelNeverReady.Load() {
			// Send a funny "no AI" message
			noAIMsg := noAIMessages[rand.Intn(len(noAIMessages))]
			log.Printf("AI not available, sending no-AI message: %s", noAIMsg)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(noAIMsg)); err != nil {
				log.Println("Error sending no-AI message:", err)
			}
			// Save the message to database
			saveMessage("AI", noAIMsg)
			continue
		}

		// Check if model is still loading
		if !modelReady.Load() {
			// Send a funny waiting message
			waitMsg := waitingMessages[rand.Intn(len(waitingMessages))]
			log.Printf("Model loading, sending waiting message: %s", waitMsg)
			if err := conn.WriteMessage(websocket.TextMessage, []byte(waitMsg)); err != nil {
				log.Println("Error sending waiting message:", err)
			}
			// Save the waiting message to database
			saveMessage("AI", waitMsg)
			continue
		}

		// Stream AI response
		streamOllamaResponse(conn, string(msg))
	}

	log.Println("WebSocket connection closed")
}

// Config structure for environment variables
type Config struct {
	Title     string `json:"title"`
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	Model     string `json:"model"`
	Region    string `json:"region"`
	Role      string `json:"role"`
}

// Handler to return configuration as JSON
func getConfig(w http.ResponseWriter, r *http.Request) {
	// Get region and role with defaults
	region := os.Getenv("REGION")
	if region == "" {
		region = "unknown"
	}
	role := os.Getenv("ROLE")
	if role == "" {
		role = "unknown"
	}

	config := Config{
		Title:     os.Getenv("CHAT_TITLE"), // Read from env variable
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		Model:     ollamaModel, // Use the dynamically retrieved model
		Region:    region,
		Role:      role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// Model status response structure
type ModelStatusResponse struct {
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
	Model  string `json:"model"`
}

// Handler to return model status
func getModelStatus(w http.ResponseWriter, r *http.Request) {
	status := ModelStatusResponse{
		Ready:  modelReady.Load(),
		Status: modelStatus,
		Model:  ollamaModel,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// getAvailableModel retrieves the first available model from ollama
func getAvailableModel() (string, error) {
	client := resty.New()
	ollamaModelsURL := fmt.Sprintf("%s/api/tags", ollamaURL)

	log.Printf("🔍 Checking available models at: %s", ollamaModelsURL)
	modelStatus = "checking_models"
	resp, err := client.R().Get(ollamaModelsURL)
	if err != nil {
		modelStatus = "error_connecting"
		return "", fmt.Errorf("failed to connect to ollama: %v", err)
	}

	log.Printf("📡 Ollama API response status: %d", resp.StatusCode())
	log.Printf("📡 Ollama API response body: %s", resp.String())

	if resp.StatusCode() != 200 {
		modelStatus = "error_api"
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode())
	}

	var modelsResp OllamaModelsResponse
	if err := json.Unmarshal(resp.Body(), &modelsResp); err != nil {
		modelStatus = "error_parsing"
		return "", fmt.Errorf("failed to parse models response: %v", err)
	}

	if len(modelsResp.Models) == 0 {
		modelStatus = "no_models"
		return "", fmt.Errorf("no models available in ollama")
	}

	// Return the first available model
	modelName := modelsResp.Models[0].Name
	log.Printf("📋 Found available model: %s", modelName)
	modelStatus = "model_found"
	return modelName, nil
}

// testModelGeneration tests if the model can actually generate responses with short timeout and retries
func testModelGeneration(modelName string) error {
	log.Printf("🧪 Testing model generation with short timeout and retries...")
	modelStatus = "testing_generation"

	// Try multiple times with short timeouts
	maxTestRetries := 100
	testTimeout := 20 * time.Second // Short timeout

	for testAttempt := 1; testAttempt <= maxTestRetries; testAttempt++ {
		log.Printf("🧪 Test attempt %d/%d (timeout: %v)", testAttempt, maxTestRetries, testTimeout)

		client := resty.New()
		client.SetTimeout(testTimeout)

		ollamaGenerateURL := fmt.Sprintf("%s/api/generate", ollamaURL)
		request := map[string]interface{}{
			"model":  modelName,
			"prompt": "Hi", // Very simple prompt
			"stream": false,
		}

		resp, err := client.R().
			SetHeader("Content-Type", "application/json").
			SetBody(request).
			Post(ollamaGenerateURL)

		if err != nil {
			log.Printf("⚠️ Test attempt %d: Connection error: %v", testAttempt, err)
			if testAttempt < maxTestRetries {
				time.Sleep(2 * time.Second) // Short delay between retries
				continue
			}
			modelStatus = "error_generation"
			return fmt.Errorf("failed to connect to ollama after %d attempts: %v", maxTestRetries, err)
		}

		log.Printf("🧪 Test attempt %d status: %d", testAttempt, resp.StatusCode())
		if resp.StatusCode() != 200 {
			log.Printf("⚠️ Test attempt %d: HTTP error %d: %s", testAttempt, resp.StatusCode(), resp.String())
			if testAttempt < maxTestRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			modelStatus = "error_generation"
			return fmt.Errorf("model generation failed with status %d after %d attempts: %s", resp.StatusCode(), maxTestRetries, resp.String())
		}

		// Parse response to ensure we got actual content
		var response map[string]interface{}
		if err := json.Unmarshal(resp.Body(), &response); err != nil {
			log.Printf("⚠️ Test attempt %d: Parse error: %v", testAttempt, err)
			if testAttempt < maxTestRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			modelStatus = "error_parsing_response"
			return fmt.Errorf("failed to parse generation response after %d attempts: %v", maxTestRetries, err)
		}

		if response["response"] == nil {
			log.Printf("⚠️ Test attempt %d: No response content", testAttempt)
			if testAttempt < maxTestRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			modelStatus = "error_no_response"
			return fmt.Errorf("no response content from model after %d attempts", maxTestRetries)
		}

		// Success!
		log.Printf("✅ Model generation test successful on attempt %d!", testAttempt)
		return nil
	}

	modelStatus = "error_generation"
	return fmt.Errorf("model generation failed after %d attempts", maxTestRetries)
}

// checkModelReady checks if the ollama service is ready with the preloaded model
func checkModelReady() {
	// If Ollama is disabled, mark as permanently unavailable
	if !ollamaEnabled {
		log.Printf("🚫 Ollama is disabled. AI features unavailable.")
		modelStatus = "disabled"
		modelNeverReady.Store(true)
		return
	}

	log.Printf("🚀 Checking if ollama service is ready...")
	modelStatus = "starting"

	// Add retry logic
	maxRetries := 100              // More retries for model loading
	retryDelay := 10 * time.Second // Longer delay for model loading

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("Retry attempt %d/%d for model readiness check...", attempt, maxRetries)
			modelStatus = fmt.Sprintf("retry_%d", attempt)
			time.Sleep(retryDelay)
		}

		// Get the available model
		model, err := getAvailableModel()
		if err != nil {
			log.Printf("⚠️ Attempt %d: Failed to get available model: %v", attempt, err)
			continue
		}

		// Set the model name
		ollamaModel = model
		log.Printf("✅ Using model: %s", ollamaModel)

		// Test if the model can actually generate responses (this will load it into memory)
		if err := testModelGeneration(ollamaModel); err != nil {
			log.Printf("⚠️ Attempt %d: Model not ready for generation: %v", attempt, err)
			continue
		}

		// Success! Model is loaded and ready
		modelStatus = "ready"
		modelReady.Store(true)
		log.Printf("✅ Ollama service is ready with model: %s", ollamaModel)
		return
	}

	log.Printf("❌ Ollama service not ready after %d attempts. Users will see waiting messages.", maxRetries)
	modelStatus = "failed"
}

func main() {
	// Get environment variables
	ollamaURL = os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		log.Fatal("OLLAMA_URL environment variable not set")
	}

	// Check if Ollama is enabled (defaults to true for backwards compatibility)
	ollamaEnabledStr := os.Getenv("OLLAMA_ENABLED")
	ollamaEnabled = ollamaEnabledStr != "false"

	if ollamaEnabled {
		log.Printf("Using Ollama service with dynamic model detection")
	} else {
		log.Printf("Ollama disabled - AI features will be unavailable")
	}

	// Initialize database
	initDB()
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Set up HTTP routes with CORS
	http.HandleFunc("/api/ws", handleWebSocket)
	http.HandleFunc("/api/history", corsMiddleware(getChatHistory))
	http.HandleFunc("/api/config", corsMiddleware(getConfig))
	http.HandleFunc("/api/model-status", corsMiddleware(getModelStatus))
	http.HandleFunc("/api/ready", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ready": modelReady.Load()})
	}))

	// Start model readiness check in background
	go checkModelReady()

	log.Printf("🌐 WebSocket server started on port %s", port)
	log.Println("🔄 Checking ollama service readiness in background...")
	log.Println("⚠️  Note: Chat will respond with waiting messages until ollama service is ready")
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("Server error:", err)
	}
}
