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
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system"`
	Stream  bool           `json:"stream"`
	Context []int          `json:"context"`
	Options map[string]any `json:"options,omitempty"`
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

const (
	maxContextTokens   = 4096
	maxDiscordMsgLen   = 2000
	streamEditInterval = 1500 * time.Millisecond
)

var (
	activeModel    string
	userContext    = make(map[string][]int)
	userActivity   = make(map[string]time.Time)
	contextMutex   = sync.RWMutex{}
	contextTimeout = 15 * time.Minute
	inferenceSem   = make(chan struct{}, 1)

	ollamaHost string
	prefix     string

	ollamaClient = &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        2,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}
)

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func init() {
	ollamaHost = os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	prefix = os.Getenv("MENTION_PREFIX")
	if prefix == "" {
		prefix = "georgibot"
	}

	activeModel = "hermes"
}

func detectModel() {
	resp, err := ollamaClient.Get(ollamaHost + "/api/tags")
	if err != nil {
		log.Printf("Could not query Ollama models, using fallback '%s': %v", activeModel, err)
		return
	}
	defer resp.Body.Close()

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		log.Printf("Could not parse Ollama tags, using fallback '%s': %v", activeModel, err)
		return
	}

	for _, m := range tags.Models {
		if strings.HasPrefix(m.Name, "impersonate") {
			activeModel = "impersonate"
			log.Printf("Detected fine-tuned model '%s', using it for inference", m.Name)
			return
		}
	}

	log.Printf("No 'impersonate' model found, using fallback '%s'", activeModel)
}

func Inference(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || !isProperlyMentioned(m.Content) {
		return
	}

	select {
	case inferenceSem <- struct{}{}:
		defer func() { <-inferenceSem }()
	default:
		_, _ = s.ChannelMessageSendReply(
			m.ChannelID,
			"I'm busy thinking about something else, try again in a moment.",
			m.Reference(),
		)
		return
	}

	prompt, sysPrompt := getOllamaRequestData(m.Content, m.Author.Username)

	if len(prompt) > 1000 {
		log.Printf("Prompt exceeds 1000 characters. Aborting.")
		_, _ = s.ChannelMessageSendReply(
			m.ChannelID,
			"Yeah, not reading all that. 1000 characters or less please.",
			m.Reference(),
		)
		return
	}

	contextMutex.RLock()
	last := userActivity[m.Author.ID]
	contextMutex.RUnlock()

	if time.Since(last) > contextTimeout {
		contextMutex.Lock()
		delete(userContext, m.Author.ID)
		delete(userActivity, m.Author.ID)
		contextMutex.Unlock()
	}

	contextMutex.RLock()
	ctx := userContext[m.Author.ID]
	contextMutex.RUnlock()

	body, err := json.Marshal(OllamaGenerateRequest{
		Model:   activeModel,
		Prompt:  enrichPrompt(prompt, s, m),
		System:  sysPrompt,
		Stream:  true,
		Context: ctx,
		Options: map[string]any{
			"num_predict": 512,
			"num_ctx":     4096,
			"num_threads": 4,
		},
	})
	if err != nil {
		log.Printf("Error marshalling request: %s", err)
		return
	}

	typingDone := make(chan struct{})
	var typingOnce sync.Once
	stopTyping := func() { typingOnce.Do(func() { close(typingDone) }) }
	defer stopTyping()

	go func() {
		_ = s.ChannelTyping(m.ChannelID)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-typingDone:
				return
			case <-ticker.C:
				_ = s.ChannelTyping(m.ChannelID)
			}
		}
	}()

	resp, err := ollamaClient.Post(
		ollamaHost+"/api/generate",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		log.Printf("Error calling Ollama: %s", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("Ollama returned status %d: %s", resp.StatusCode, string(errBody))
		return
	}

	var fullResponse strings.Builder
	var finalResp OllamaGenerateResponse
	var reply *discordgo.Message
	lastEdit := time.Now()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk OllamaGenerateResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err != io.EOF {
				log.Printf("Error decoding stream chunk: %s", err)
			}
			break
		}

		fullResponse.WriteString(chunk.Response)

		if chunk.Done {
			finalResp = chunk
			break
		}

		text := truncateForDiscord(escapeMarkdown(fullResponse.String()))
		if text == "" {
			continue
		}

		if reply == nil {
			stopTyping()
			reply, err = s.ChannelMessageSendReply(m.ChannelID, text, m.Reference())
			if err != nil {
				log.Printf("Error sending initial reply: %s", err)
			}
			lastEdit = time.Now()
		} else if time.Since(lastEdit) > streamEditInterval {
			_, _ = s.ChannelMessageEdit(m.ChannelID, reply.ID, text)
			lastEdit = time.Now()
		}
	}

	finalText := truncateForDiscord(escapeMarkdown(fullResponse.String()))
	if finalText == "" {
		log.Printf("Empty response from Ollama")
		return
	}

	if reply == nil {
		stopTyping()
		_, err = s.ChannelMessageSendReply(m.ChannelID, finalText, m.Reference())
		if err != nil {
			log.Printf("Error sending response to Discord: %s", err)
			_, _ = s.ChannelMessageSend(m.ChannelID, finalText)
		}
	} else {
		_, _ = s.ChannelMessageEdit(m.ChannelID, reply.ID, finalText)
	}

	finalCtx := finalResp.Context
	if len(finalCtx) > maxContextTokens {
		finalCtx = finalCtx[len(finalCtx)-maxContextTokens:]
	}

	contextMutex.Lock()
	userContext[m.Author.ID] = finalCtx
	userActivity[m.Author.ID] = time.Now()
	contextMutex.Unlock()

	log.Printf("Inference complete: model=%s eval_count=%d total_duration_ns=%d",
		finalResp.Model, finalResp.EvalCount, finalResp.TotalDuration)
}

func truncateForDiscord(s string) string {
	runes := []rune(s)
	if len(runes) > maxDiscordMsgLen {
		return string(runes[:maxDiscordMsgLen-3]) + "..."
	}
	return s
}

func escapeMarkdown(s string) string {
	return strings.ReplaceAll(s, "*", "\\*")
}

func isProperlyMentioned(content string) bool {
	str := strings.ToLower(content)

	if !strings.HasPrefix(str, prefix) &&
		!strings.HasPrefix(str, "@"+prefix) &&
		!strings.Contains(str, "bulgaria") {
		return false
	}

	return true
}

func getOllamaRequestData(content, username string) (string, string) {
	systemPrompt := getSystemPrompt(username)

	systemPrompt = strings.ReplaceAll(systemPrompt, "<PREFIX>", prefix)
	systemPrompt = strings.ReplaceAll(systemPrompt, "\n", " ")
	systemPrompt = strings.ReplaceAll(systemPrompt, "\r", " ")
	systemPrompt = strings.ReplaceAll(systemPrompt, "\t", " ")

	prompt := strings.ReplaceAll(content, prefix+",", "")
	prompt = strings.ReplaceAll(prompt, prefix+",", "")
	prompt = strings.ReplaceAll(prompt, "@"+prefix+",", "")
	prompt = strings.ReplaceAll(prompt, "\n", " ")
	prompt = strings.ReplaceAll(prompt, "\r", " ")
	prompt = strings.ReplaceAll(prompt, "\t", " ")

	return prompt, systemPrompt
}

func getSystemPrompt(username string) string {
	sysPrompt := os.Getenv("SYSTEM_PROMPT_" + strings.ToUpper(username))
	if sysPrompt == "" {
		sysPrompt = os.Getenv("SYSTEM_PROMPT")
		if sysPrompt == "" {
			sysPrompt = `You are ` + prefix + `, an AI bot in a Discord server where it is your job to maintain records of quoted messages.`
		}
	}
	return sysPrompt
}

func enrichPrompt(prompt string, s *discordgo.Session, m *discordgo.MessageCreate) string {
	msg := "This message was sent by: " + m.Author.Username +
		". Message Content: " + prompt

	if m.Type == discordgo.MessageTypeReply &&
		m.ReferencedMessage != nil &&
		m.ReferencedMessage.Content != "" {

		msg = msg + ". This message was a reply to: " + m.ReferencedMessage.Content +
			". The reply was sent by: "

		if m.ReferencedMessage.Author.ID == s.State.User.ID {
			return msg + "You, the bot named " + prefix
		}

		return msg + m.ReferencedMessage.Author.Username
	}

	return msg
}
