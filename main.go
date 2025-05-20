package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	Prompt   string    `json:"prompt"`   // Optional simple input
	Messages []Message `json:"messages"` // Optional advanced input
	Stream   bool      `json:"stream"`
}

type OpenAIStreamRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

func main() {
	godotenv.Load()
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("Missing OPENAI_API_KEY in .env")
	}

	router := gin.Default()

	router.POST("/ai-agent", func(c *gin.Context) {
		var req AIRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Convert simple prompt to system+user messages if messages not provided
		var messages []Message
		if len(req.Messages) > 0 {
			messages = req.Messages
		} else if req.Prompt != "" {
			messages = []Message{
				{Role: "system", Content: "You are a helpful, creative and intelligent AI assistant."},
				{Role: "user", Content: req.Prompt},
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt or messages are required"})
			return
		}

		openaiReq := OpenAIStreamRequest{
			Model: "gpt-3.5-turbo",
			Messages: messages,
			Stream:   true,
		}

		body, _ := json.Marshal(openaiReq)

		request, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", io.NopCloser(bytes.NewReader(body)))
		request.Header.Set("Authorization", "Bearer "+apiKey)
		request.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenAI streaming request failed"})
			return
		}
		defer resp.Body.Close()

		// Enable streaming response to frontend
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || line == "data: [DONE]" {
				continue
			}

			// Remove "data: " prefix
			if len(line) > 6 && line[:6] == "data: " {
				line = line[6:]
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(line), &parsed); err == nil {
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

	router.Run(":8080")
}