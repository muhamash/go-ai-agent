package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIRequest struct {
	Prompt   string    `json:"prompt"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
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

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("⚠️  Warning: Could not load .env file")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("❌ Missing OPENAI_API_KEY in environment")
		fmt.Println("💡 Please create a .env file with: OPENAI_API_KEY=your-key-here")
		panic("Missing OPENAI_API_KEY in environment")
	}

	// Validate API key format
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
			"message": "AI Gateway API is running",
			"version": "1.0.0",
			"endpoints": []string{
				"POST /ai-agent - Chat with AI",
				"GET /health - Health check",
			},
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	router.POST("/ai-agent", func(c *gin.Context) {
		var req AIRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		// Prepare messages
		var messages []Message
		if len(req.Messages) > 0 {
			messages = req.Messages
		} else if req.Prompt != "" {
			messages = []Message{
				{Role: "system", Content: "You are a helpful, creative, and intelligent AI assistant."},
				{Role: "user", Content: req.Prompt},
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Either 'prompt' or 'messages' must be provided",
			})
			return
		}

		// Create OpenAI request
		openaiReq := OpenAIRequest{
			Model:       "gpt-3.5-turbo", // Using correct model name
			Messages:    messages,
			Stream:      req.Stream,
			Temperature: 0.7,
			MaxTokens:   2000,
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to encode request body",
			})
			return
		}

		// Create HTTP request to OpenAI (using correct endpoint)
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
			handleStreamingResponse(c, resp)
		} else {
			handleNonStreamingResponse(c, resp)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 AI Gateway Server running at http://localhost:%s\n", port)
	fmt.Println("📡 Ready to proxy requests to OpenAI API")
	router.Run(":" + port)
}

func handleStreamingResponse(c *gin.Context, resp *http.Response) {
	// Set up Server-Sent Events headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Flush()

	scanner := bufio.NewScanner(resp.Body)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Skip empty lines
		if line == "" {
			continue
		}

		// Handle the [DONE] marker
		if strings.TrimSpace(line) == "data: [DONE]" {
			fmt.Fprint(c.Writer, "data: [DONE]\n\n")
			c.Writer.Flush()
			break
		}

		// Process data lines
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			
			// Parse the JSON to extract content
			var streamResp OpenAIStreamResponse
			if err := json.Unmarshal([]byte(jsonData), &streamResp); err == nil {
				if len(streamResp.Choices) > 0 {
					content := streamResp.Choices[0].Delta.Content
					if content != "" {
						// Send the content directly (or as JSON if you prefer)
						fmt.Fprint(c.Writer, content)
						c.Writer.Flush()
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading stream: %v\n", err)
	}
}

func handleNonStreamingResponse(c *gin.Context, resp *http.Response) {
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

	// Extract the message content for easier consumption
	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					c.JSON(http.StatusOK, gin.H{
						"response": content,
						"raw":      result,
					})
					return
				}
			}
		}
	}

	// Fallback: return the raw response
	c.JSON(http.StatusOK, result)
}