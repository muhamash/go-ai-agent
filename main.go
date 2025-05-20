package main

import (
	"bytes"
	"encoding/json"

	// "fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type AIRequest struct {
	Message string `json:"message"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model    string         `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message OpenAIMessage `json:"message"`
	} `json:"choices"`
}

func main() {
	godotenv.Load() // Load API key from .env
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

		openaiReq := OpenAIRequest{
			Model: "gpt-4-turbo",
			Messages: []OpenAIMessage{
				{Role: "system", Content: "You are a helpful, intelligent, and highly engaging AI assistant."},
				{Role: "user", Content: req.Message},
			},
		}

		body, _ := json.Marshal(openaiReq)
		request, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
		request.Header.Set("Authorization", "Bearer "+apiKey)
		request.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(request)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenAI API call failed"})
			return
		}
		defer resp.Body.Close()

		respBody, _ := ioutil.ReadAll(resp.Body)
		var openaiResp OpenAIResponse
		json.Unmarshal(respBody, &openaiResp)

		if len(openaiResp.Choices) > 0 {
			c.JSON(http.StatusOK, gin.H{"reply": openaiResp.Choices[0].Message.Content})
		} else {
			c.JSON(http.StatusOK, gin.H{"reply": "Sorry, I couldn't process that."})
		}
	})

	router.Run(":8080")
}