package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Enhanced request structure with session management
type AIRequest struct {
	Prompt      string    `json:"prompt"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	SessionId   string    `json:"session_id,omitempty"`   // Optional session ID
	SystemRole  string    `json:"system_role,omitempty"`  // Custom system role
	MaxHistory  int       `json:"max_history,omitempty"`  // Max conversation history to keep
}

// Session management for conversation context
type ConversationSession struct {
	SessionId    string    `json:"session_id"`
	Messages     []Message `json:"messages"`
	SystemRole   string    `json:"system_role"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
}

// OpenAI API request structure
type OpenAIRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// OpenAI streaming response structure
type OpenAIStreamChoice struct {
	Delta struct {
		Content string `json:"content"`
		Role    string `json:"role,omitempty"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type OpenAIStreamResponse struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []OpenAIStreamChoice  `json:"choices"`
}

// Redis-based session manager
type RedisSessionManager struct {
	client         *redis.Client
	sessionTTL     time.Duration
	keyPrefix      string
	ctx            context.Context
}

func NewRedisSessionManager(redisURL, password string, db int, sessionTTL time.Duration) (*RedisSessionManager, error) {
	// Parse Redis URL or use individual components
	var rdb *redis.Client
	
	if redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Redis URL: %v", err)
		}
		rdb = redis.NewClient(opt)
	} else {
		// Default connection
		rdb = redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: password,
			DB:       db,
		})
	}

	ctx := context.Background()
	
	// Test connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &RedisSessionManager{
		client:     rdb,
		sessionTTL: sessionTTL,
		keyPrefix:  "ai_session:",
		ctx:        ctx,
	}, nil
}

func (rsm *RedisSessionManager) getSessionKey(sessionId string) string {
	return rsm.keyPrefix + sessionId
}

func (rsm *RedisSessionManager) GetOrCreateSession(sessionId, systemRole string) (*ConversationSession, error) {
	// Generate session ID if not provided
	if sessionId == "" {
		sessionId = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	sessionKey := rsm.getSessionKey(sessionId)
	
	// Try to get existing session
	sessionData, err := rsm.client.Get(rsm.ctx, sessionKey).Result()
	if err == redis.Nil {
		// Session doesn't exist, create new one
		if systemRole == "" {
			systemRole = "You are a helpful, creative, and intelligent AI assistant. You maintain context from our previous conversation and provide thoughtful, relevant responses."
		}

		session := &ConversationSession{
			SessionId:    sessionId,
			Messages:     []Message{{Role: "system", Content: systemRole}},
			SystemRole:   systemRole,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
		}

		// Save to Redis
		if err := rsm.saveSession(session); err != nil {
			return nil, err
		}

		return session, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get session from Redis: %v", err)
	}

	// Parse existing session
	var session ConversationSession
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %v", err)
	}

	// Update last activity
	session.LastActivity = time.Now()
	if err := rsm.saveSession(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (rsm *RedisSessionManager) saveSession(session *ConversationSession) error {
	sessionKey := rsm.getSessionKey(session.SessionId)
	
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	err = rsm.client.Set(rsm.ctx, sessionKey, sessionData, rsm.sessionTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to save session to Redis: %v", err)
	}

	return nil
}

func (rsm *RedisSessionManager) AddMessage(sessionId string, message Message) error {
	sessionKey := rsm.getSessionKey(sessionId)
	
	// Get current session
	sessionData, err := rsm.client.Get(rsm.ctx, sessionKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get session: %v", err)
	}

	var session ConversationSession
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return fmt.Errorf("failed to parse session: %v", err)
	}

	// Add message and update activity
	session.Messages = append(session.Messages, message)
	session.LastActivity = time.Now()

	// Save back to Redis
	return rsm.saveSession(&session)
}

func (rsm *RedisSessionManager) GetMessages(sessionId string, maxHistory int) ([]Message, error) {
	sessionKey := rsm.getSessionKey(sessionId)
	
	sessionData, err := rsm.client.Get(rsm.ctx, sessionKey).Result()
	if err == redis.Nil {
		return []Message{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get session: %v", err)
	}

	var session ConversationSession
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %v", err)
	}

	messages := session.Messages

	// Limit conversation history if specified
	if maxHistory > 0 && len(messages) > maxHistory+1 { // +1 for system message
		// Keep system message + recent messages
		systemMsg := messages[0]
		recentMessages := messages[len(messages)-maxHistory:]
		messages = append([]Message{systemMsg}, recentMessages...)
	}

	return messages, nil
}

func (rsm *RedisSessionManager) GetSessionInfo(sessionId string) (map[string]interface{}, error) {
	sessionKey := rsm.getSessionKey(sessionId)
	
	sessionData, err := rsm.client.Get(rsm.ctx, sessionKey).Result()
	if err == redis.Nil {
		return map[string]interface{}{"exists": false}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get session: %v", err)
	}

	var session ConversationSession
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %v", err)
	}

	return map[string]interface{}{
		"exists":        true,
		"session_id":    session.SessionId,
		"message_count": len(session.Messages) - 1, // Exclude system message
		"system_role":   session.SystemRole,
		"created_at":    session.CreatedAt,
		"last_activity": session.LastActivity,
	}, nil
}

func (rsm *RedisSessionManager) DeleteSession(sessionId string) error {
	sessionKey := rsm.getSessionKey(sessionId)
	return rsm.client.Del(rsm.ctx, sessionKey).Err()
}

func (rsm *RedisSessionManager) GetAllSessions() ([]string, error) {
	pattern := rsm.keyPrefix + "*"
	keys, err := rsm.client.Keys(rsm.ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	sessionIds := make([]string, len(keys))
	for i, key := range keys {
		sessionIds[i] = strings.TrimPrefix(key, rsm.keyPrefix)
	}

	return sessionIds, nil
}

func (rsm *RedisSessionManager) GetSessionStats() (map[string]interface{}, error) {
	// Get total sessions
	pattern := rsm.keyPrefix + "*"
	keys, err := rsm.client.Keys(rsm.ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	// Get Redis memory usage
	memInfo, err := rsm.client.Info(rsm.ctx, "memory").Result()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_sessions":    len(keys),
		"redis_memory_info": memInfo,
		"session_ttl":       rsm.sessionTTL.String(),
	}, nil
}

func (rsm *RedisSessionManager) Close() error {
	return rsm.client.Close()
}

var sessionManager *RedisSessionManager

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("⚠️  Warning: Could not load .env file")
	}

	// Redis configuration
	redisURL := os.Getenv("REDIS_URL") // For cloud deployments (Heroku, Railway, etc.)
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil {
			redisDB = db
		}
	}

	// Session TTL (default 24 hours)
	sessionTTL := 24 * time.Hour
	if ttlStr := os.Getenv("SESSION_TTL_HOURS"); ttlStr != "" {
		if hours, err := strconv.Atoi(ttlStr); err == nil {
			sessionTTL = time.Duration(hours) * time.Hour
		}
	}

	// Initialize Redis session manager
	sessionManager, err = NewRedisSessionManager(redisURL, redisPassword, redisDB, sessionTTL)
	if err != nil {
		fmt.Printf("❌ Failed to connect to Redis: %v\n", err)
		fmt.Println("💡 Make sure Redis is running and check your configuration")
		panic(err)
	}
	defer sessionManager.Close()

	fmt.Println("✅ Connected to Redis successfully")

	// OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("❌ Missing OPENAI_API_KEY in environment")
		fmt.Println("💡 Please create a .env file with: OPENAI_API_KEY=your-key-here")
		panic("Missing OPENAI_API_KEY in environment")
	}

	if !strings.HasPrefix(apiKey, "sk-") {
		fmt.Println("⚠️  Warning: API key should start with 'sk-'")
	}

	fmt.Printf("✅ API key loaded: %s...%s\n", apiKey[:7], apiKey[len(apiKey)-4:])

	// Set up Gin router
	router := gin.Default()

	// Add CORS middleware
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	// Health check endpoint
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "AI Gateway API with Redis is running",
			"version": "3.0.0",
			"endpoints": []string{
				"POST /ai-agent - Chat with AI (with Redis session management)",
				"GET /health - Health check",
				"GET /session/:id - Get session information",
				"DELETE /session/:id - Clear session",
				"GET /sessions - List all sessions",
				"GET /stats - Get system statistics",
			},
		})
	})

	router.GET("/health", func(c *gin.Context) {
		// Test Redis connection
		_, err := sessionManager.client.Ping(sessionManager.ctx).Result()
		redisHealthy := err == nil

		c.JSON(http.StatusOK, gin.H{
			"status":       "healthy",
			"redis_status": redisHealthy,
		})
	})

	// Get session information
	router.GET("/session/:id", func(c *gin.Context) {
		sessionId := c.Param("id")
		info, err := sessionManager.GetSessionInfo(sessionId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, info)
	})

	// Clear session
	router.DELETE("/session/:id", func(c *gin.Context) {
		sessionId := c.Param("id")
		err := sessionManager.DeleteSession(sessionId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Session cleared"})
	})

	// List all sessions
	router.GET("/sessions", func(c *gin.Context) {
		sessions, err := sessionManager.GetAllSessions()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"sessions": sessions,
			"count":    len(sessions),
		})
	})

	// Get system statistics
	router.GET("/stats", func(c *gin.Context) {
		stats, err := sessionManager.GetSessionStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, stats)
	})

	// Enhanced AI agent endpoint with Redis session management
	router.POST("/ai-agent", func(c *gin.Context) {
		var req AIRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		// Validate input
		if req.Prompt == "" && len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Either 'prompt' or 'messages' must be provided",
			})
			return
		}

		var messages []Message
		var sessionId string

		if len(req.Messages) > 0 {
			// Direct messages provided - use as is but still manage session
			messages = req.Messages
			sessionId = req.SessionId
		} else {
			// Simple prompt provided - use automatic role management
			var session *ConversationSession
			session, err = sessionManager.GetOrCreateSession(req.SessionId, req.SystemRole)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to manage session: " + err.Error(),
				})
				return
			}
			sessionId = session.SessionId

			// Add user message to session
			userMessage := Message{Role: "user", Content: req.Prompt}
			if err := sessionManager.AddMessage(sessionId, userMessage); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to add message to session: " + err.Error(),
				})
				return
			}

			// Get conversation history with limit
			maxHistory := req.MaxHistory
			if maxHistory == 0 {
				maxHistory = 10 // Default to last 10 exchanges
			}
			
			messages, err = sessionManager.GetMessages(sessionId, maxHistory)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to get conversation history: " + err.Error(),
				})
				return
			}
		}

		// Create OpenAI request
		openaiReq := OpenAIRequest{
			Model:       "gpt-3.5-turbo",
			Messages:    messages,
			Stream:      req.Stream,
			Temperature: 0.7,
			MaxTokens:   10,
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to encode request body",
			})
			return
		}

		// Create HTTP request to OpenAI
		httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBody))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create OpenAI request",
			})
			return
		}

		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to contact OpenAI API",
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.JSON(resp.StatusCode, gin.H{
				"error":      "OpenAI API returned error",
				"statusCode": resp.StatusCode,
				"body":       string(body),
			})
			return
		}

		// Handle streaming vs non-streaming responses
		if req.Stream {
			handleStreamingResponse(c, resp, sessionId)
		} else {
			handleNonStreamingResponse(c, resp, sessionId)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 AI Gateway Server running at http://localhost:%s\n", port)
	fmt.Println("📡 Ready to proxy requests to OpenAI API")
	fmt.Println("🧠 Redis session management enabled")
	fmt.Printf("⏰ Session TTL: %s\n", sessionTTL.String())
	router.Run(":" + port)
}

func handleStreamingResponse(c *gin.Context, resp *http.Response, sessionId string) {
	// Set up Server-Sent Events headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Flush()

	scanner := bufio.NewScanner(resp.Body)
	var assistantResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		if strings.TrimSpace(line) == "data: [DONE]" {
			fmt.Fprint(c.Writer, "data: [DONE]\n\n")
			c.Writer.Flush()
			break
		}

		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")

			var streamResp OpenAIStreamResponse
			if err := json.Unmarshal([]byte(jsonData), &streamResp); err == nil {
				if len(streamResp.Choices) > 0 {
					content := streamResp.Choices[0].Delta.Content
					if content != "" {
						assistantResponse.WriteString(content)
						fmt.Fprint(c.Writer, content)
						c.Writer.Flush()
					}
				}
			}
		}
	}

	// Save assistant response to Redis session
	if sessionId != "" && assistantResponse.Len() > 0 {
		assistantMessage := Message{
			Role:    "assistant",
			Content: assistantResponse.String(),
		}
		if err := sessionManager.AddMessage(sessionId, assistantMessage); err != nil {
			fmt.Printf("Failed to save assistant response to session: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading stream: %v\n", err)
	}
}

func handleNonStreamingResponse(c *gin.Context, resp *http.Response, sessionId string) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read OpenAI response",
		})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse OpenAI response",
		})
		return
	}

	// Extract the message content and save to Redis session
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					// Save assistant response to Redis session
					if sessionId != "" {
						assistantMessage := Message{
							Role:    "assistant",
							Content: content,
						}
						if err := sessionManager.AddMessage(sessionId, assistantMessage); err != nil {
							fmt.Printf("Failed to save assistant response to session: %v\n", err)
						}
					}

					c.JSON(http.StatusOK, gin.H{
						"response":   content,
						"session_id": sessionId,
						"raw":        result,
					})
					return
				}
			}
		}
	}

	// Fallback: return the raw response
	c.JSON(http.StatusOK, result)
}