package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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

type OpenAIStreamRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Could not load .env file")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("Missing OPENAI_API_KEY in environment")
	}

	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to the AI Agent API. Use POST /ai-agent with a JSON body to interact with the AI.",
		})
	})

	router.POST("/ai-agent", func(c *gin.Context) {
		var req AIRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Either 'prompt' or 'messages' must be provided"})
			return
		}

		// Create OpenAI request
		openaiReq := OpenAIStreamRequest{
			Model:    "gpt-3.5-turbo",
			Messages: messages,
			Stream:   true,
		}

		reqBody, err := json.Marshal(openaiReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode request body"})
			return
		}

		// Create HTTP request to OpenAI
		httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(reqBody))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create OpenAI request"})
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to contact OpenAI"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":      "OpenAI API returned error",
				"statusCode": resp.StatusCode,
				"body":       string(body),
			})
			return
		}

		// Setup streaming response
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				break
			}

			text := string(line)
			if text == "" || text == "data: [DONE]\n" {
				continue
			}

			if len(text) > 6 && text[:6] == "data: " {
				text = text[6:]
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(text), &parsed); err == nil {
				if choices, ok := parsed["choices"].([]interface{}); ok {
					if delta, ok := choices[0].(map[string]interface{})["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok {
							c.Writer.Write([]byte(content))
							c.Writer.Flush()
						}
					}
				}
			}
		}
	})

	fmt.Println("Server running at http://localhost:8080")
	router.Run(":8080")
}