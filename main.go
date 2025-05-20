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
	Messages []Message `json:"messages"`
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

		if len(req.Messages) == 0 {
			// Add default system + user message if not provided
			req.Messages = []Message{
				{Role: "system", Content: "You are a highly intelligent and creative AI assistant."},
				{Role: "user", Content: "Hello!"},
			}
		}

		openaiReq := OpenAIStreamRequest{
			Model:    "gpt-4",
			Messages: req.Messages,
			Stream:   true,
		}

		body, _ := json.Marshal(openaiReq)

		request, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", io.NopCloser(bufio.NewReader(bytes.NewReader(body))))
		request.Header.Set("Authorization", "Bearer "+apiKey)
		request.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenAI streaming request failed"})
			return
		}
		defer resp.Body.Close()

		// Set headers to enable streaming
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Flush()

		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()

			// Skip heartbeat or empty lines
			if line == "" || line == "data: [DONE]" {
				continue
			}

			// Extract content safely
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(line[6:]), &parsed); err == nil {
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
