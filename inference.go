package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
	Context []int `json:"context"`
}

type OllamaGenerateResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	DoneReason         string `json:"done_reason"`
	Context            []int  `json:"context"`
	TotalDuration      int    `json:"total_duration"`
	LoadDuration       int    `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int    `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int    `json:"eval_duration"`
}

var (
	channelContext = make(map[string][]int)
	channelActivity = make(map[string]time.Time)
	contextMutex = sync.RWMutex{}
	contextTimeout = time.Minute * 30 
)

func enrichPrompt(prompt, user string) string {
	return `This message was sent by ` + user +
		`Message Content: ` + prompt
}

func Inference(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || !isProperlyMentioned(m.Content) {
		return
	}

	prompt, sysPrompt := getOllamaRequestData(m.Content)

	contextMutex.RLock()
	last := channelActivity[m.ChannelID]
	contextMutex.RUnlock()

	if time.Since(last) > contextTimeout {
		contextMutex.Lock()
		delete(channelContext, m.ChannelID)
		delete(channelActivity, m.ChannelID)
		contextMutex.Unlock()
		log.Printf("Cleared stale channel context for channel %s\n", m.ChannelID)
	}

	contextMutex.RLock()
	ctx := channelContext[m.ChannelID]
	contextMutex.RUnlock()


	body, err := json.Marshal(OllamaGenerateRequest{
		Model:  "qwen2.5:3b",
		Prompt: enrichPrompt(prompt, m.Member.DisplayName()),
		System: sysPrompt,
		Stream: false,
		Context: ctx,
	})
	if err != nil {
		log.Printf("Error marshalling request: %s\n", err)
		return
	}

	if len(prompt) > 1000 {
		log.Printf("Prompt exceeds 1000 characters. Aborting.")
		_, _ = s.ChannelMessageSendReply(
			m.ChannelID,
			"Yeah, not reading all that. 1000 characters or less please.",
			m.Reference(),
		)
	}

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = s.ChannelTyping(m.ChannelID)
			}
		}
	}()

	client := &http.Client{
		Timeout: time.Second * 600,
	}
	resp, err := client.Post(
		getOllamaHost()+"/api/generate",
		"application/json",
		bytes.NewBuffer(body),
	)

	close(done)

	if err != nil {
		log.Printf("Error calling Ollama: %s\n", err)
		return
	}

	defer resp.Body.Close()

	bbytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %s\n", err)
		return
	}

	log.Printf("Ollama response body: %s\n", string(bbytes))

	var ollamaResp OllamaGenerateResponse
	if err := json.Unmarshal(bbytes, &ollamaResp); err != nil {
		log.Printf("Error decoding response: %s\n", err)
		return
	}

	if ollamaResp.Response == "" {
		log.Printf("Empty response, sending default message\n")
		return
	}

	contextMutex.Lock()
	channelContext[m.ChannelID] = ollamaResp.Context
	channelActivity[m.ChannelID] = time.Now()
	contextMutex.Unlock()

	_, err = s.ChannelMessageSendReply(m.ChannelID, ollamaResp.Response, m.Reference())
	if err != nil {
		log.Printf("Error sending response to Discord: %s\n", err)
		_, _ = s.ChannelMessageSend(m.ChannelID, ollamaResp.Response)
	}
}

func isProperlyMentioned(content string) bool {
	prefix := getPrefix()
	str := strings.ToLower(content)

	if !strings.HasPrefix(str, prefix+",") &&
		!strings.HasPrefix(str, "@"+prefix+",") &&
		!strings.Contains(str, "bulgaria") {
		return false
	}

	return true
}

func getPrefix() string {
	prefix := os.Getenv("MENTION_PREFIX")
	if prefix == "" {
		prefix = "georgibot"
	}
	return prefix
}

func getOllamaHost() string {
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	return ollamaHost
}

func getSystemPrompt() string {
	sysPrompt := os.Getenv("SYSTEM_PROMPT")
	if sysPrompt == "" {
		sysPrompt = `You are ` + getPrefix() + `, an AI bot in a Discord server where it is your job to maintain records of quoted messages.
You love Bulgaria and it's vibrant history, and love talking about it any chance you get. You are friendly and helpful to all requests.`
	}
	return sysPrompt
}

func getOllamaRequestData(content string) (string, string) {
	systemPrompt := getSystemPrompt()
	prefix := getPrefix()

	systemPrompt = strings.ReplaceAll(systemPrompt, "${PREFIX}", prefix)
	systemPrompt = strings.ReplaceAll(systemPrompt, "\n", " ")
	systemPrompt = strings.ReplaceAll(systemPrompt, "\r", " ")
	systemPrompt = strings.ReplaceAll(systemPrompt, "\t", " ")

	prompt := strings.ReplaceAll(content, prefix+",", "")
	prompt = strings.ReplaceAll(prompt, prefix+",", "")
	prompt = strings.ReplaceAll(prompt, "\n", " ")
	prompt = strings.ReplaceAll(prompt, "\r", " ")
	prompt = strings.ReplaceAll(prompt, "\t", " ")

	return prompt, systemPrompt
}
