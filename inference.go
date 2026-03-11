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

type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options,omitempty"`
}

type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// streamChunk is a union type that handles both /api/generate and /api/chat
// streaming responses. Fields not present in a given response decode as zero values.
type streamChunk struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Message  struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
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

func (c *streamChunk) text() string {
	if c.Response != "" {
		return c.Response
	}
	return c.Message.Content
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

const (
	maxContextTokens   = 4096
	maxDiscordMsgLen   = 2000
	streamEditInterval = 1500 * time.Millisecond
)

var (
	activeModel  string
	userContext   = make(map[string][]int)
	userActivity = make(map[string]time.Time)
	contextMutex = sync.RWMutex{}
	contextTimeout = 15 * time.Minute
	inferenceSem = make(chan struct{}, 1)

	ollamaHost   string
	prefix       string
	triggerWords []string

	ollamaClient = &http.Client{
		Timeout: 300 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        2,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}
)

func init() {
	ollamaHost = os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	prefix = os.Getenv("MENTION_PREFIX")
	if prefix == "" {
		prefix = "georgibot"
	}

	if words := os.Getenv("TRIGGER_WORDS"); words != "" {
		for _, w := range strings.Split(words, ",") {
			if trimmed := strings.TrimSpace(strings.ToLower(w)); trimmed != "" {
				triggerWords = append(triggerWords, trimmed)
			}
		}
	}
	if len(triggerWords) > 0 {
		log.Printf("Loaded %d trigger word(s): %v", len(triggerWords), triggerWords)
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

	var reqBody []byte
	var endpoint string
	var err error

	opts := map[string]any{
		"num_predict": 512,
		"num_ctx":     4096,
		"num_threads": 4,
	}

	if activeModel == "impersonate" {
		endpoint = "/api/chat"
		reqBody, err = json.Marshal(OllamaChatRequest{
			Model:    activeModel,
			Messages: []OllamaChatMessage{{Role: "user", Content: stripBotPrefix(prompt)}},
			Stream:   true,
			Options:  opts,
		})
	} else {
		endpoint = "/api/generate"
		ctx := getAndCleanContext(m.Author.ID)
		reqBody, err = json.Marshal(OllamaGenerateRequest{
			Model:   activeModel,
			Prompt:  enrichPrompt(prompt, s, m),
			System:  sysPrompt,
			Stream:  true,
			Context: ctx,
			Options: opts,
		})
	}
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
		ollamaHost+endpoint,
		"application/json",
		bytes.NewBuffer(reqBody),
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
	var finalChunk streamChunk
	var reply *discordgo.Message
	lastEdit := time.Now()

	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk streamChunk
		if err := decoder.Decode(&chunk); err != nil {
			if err != io.EOF {
				log.Printf("Error decoding stream chunk: %s", err)
			}
			break
		}

		fullResponse.WriteString(chunk.text())

		if chunk.Done {
			finalChunk = chunk
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

	if activeModel != "impersonate" {
		finalCtx := finalChunk.Context
		if len(finalCtx) > maxContextTokens {
			finalCtx = finalCtx[len(finalCtx)-maxContextTokens:]
		}
		contextMutex.Lock()
		userContext[m.Author.ID] = finalCtx
		userActivity[m.Author.ID] = time.Now()
		contextMutex.Unlock()
	}

	log.Printf("Inference complete: model=%s eval_count=%d total_duration_ns=%d",
		finalChunk.Model, finalChunk.EvalCount, finalChunk.TotalDuration)
}

func getAndCleanContext(authorID string) []int {
	contextMutex.RLock()
	last := userActivity[authorID]
	contextMutex.RUnlock()

	if time.Since(last) > contextTimeout {
		contextMutex.Lock()
		delete(userContext, authorID)
		delete(userActivity, authorID)
		contextMutex.Unlock()
	}

	contextMutex.RLock()
	ctx := userContext[authorID]
	contextMutex.RUnlock()
	return ctx
}

func isProperlyMentioned(content string) bool {
	str := strings.ToLower(content)

	if strings.HasPrefix(str, prefix) || strings.HasPrefix(str, "@"+prefix) {
		return true
	}

	for _, word := range triggerWords {
		if strings.Contains(str, word) {
			return true
		}
	}

	return false
}

func getOllamaRequestData(content, username string) (string, string) {
	systemPrompt := getSystemPrompt(username)

	systemPrompt = strings.ReplaceAll(systemPrompt, "<PREFIX>", prefix)
	systemPrompt = strings.ReplaceAll(systemPrompt, "\n", " ")
	systemPrompt = strings.ReplaceAll(systemPrompt, "\r", " ")
	systemPrompt = strings.ReplaceAll(systemPrompt, "\t", " ")

	prompt := strings.ReplaceAll(content, prefix+",", "")
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
			sysPrompt = `You are ` + prefix + `, an AI bot in a Discord server. You are friendly and helpful to all requests.`
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

func stripBotPrefix(prompt string) string {
	cleaned := strings.TrimSpace(prompt)
	lower := strings.ToLower(cleaned)
	pfx := strings.ToLower(prefix)

	for _, p := range []string{"@" + pfx, pfx} {
		if strings.HasPrefix(lower, p) {
			cleaned = strings.TrimSpace(cleaned[len(p):])
			cleaned = strings.TrimLeft(cleaned, ",. ")
			return strings.TrimSpace(cleaned)
		}
	}
	return cleaned
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
