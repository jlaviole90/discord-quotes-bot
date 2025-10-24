package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func main() {
	session, _ := discordgo.New(
		"Bot " + os.Getenv("DISCORD_TOKEN"),
	)

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as %s", r.User.String())
	})

	session.AddHandler(handleQuote)
	session.AddHandler(answerQuestion)

	err := session.Open()
	if err != nil {
		log.Fatalf("could not open session: %s", err)
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch

	err = session.Close()
	if err != nil {
		log.Printf("could not close session gracefully: %s", err)
	}
}

func handleQuote(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// Guard clause against non-quote reactions
	if r.MessageReaction.Emoji.Name != "📸" &&
		r.MessageReaction.Emoji.Name != ":camera_with_flash:" {
		return
	}

	msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		log.Fatalf("FATAL 0001: could not get message: %s\n", err)
	}

	// Don't quote bots
	if msg.Author.Bot && msg.Author.ID != s.State.User.ID {
		_, err = s.ChannelMessageSend(r.ChannelID, "Sorry, I don't quote application messages!")
		if err != nil {
			log.Fatalf("FATAL 0002: could not send message: %s\n", err)
		}
		return
	}

	chns, err := s.GuildChannels(r.GuildID)
	if err != nil {
		log.Fatalf("FATAL 0003: could not get channels: %s\n", err)
	}

	// Find the quotes channel
	// If we don't find a quotes channel, send a message so the user can know to create one
	qchn, err := getQuotesChannel(chns)
	if err != nil {
		_, err = s.ChannelMessageSend(r.ChannelID, "Sorry, I couldn't find the quotes channel!")
		if err != nil {
			log.Fatalf("FATAL 0004: could not send message: %s\n", err)
		}
		return
	}

	enableChannelCache(s, qchn)

	// Don't send the same message twice
	for _, m := range qchn.Messages {
		if m.Content == msg.Content {
			return
		}
	}

	// Create a new webhook to mimic the user in question
	wh, err := s.WebhookCreate(qchn.ID, msg.Author.Username, msg.Author.AvatarURL(""))
	if err != nil {
		log.Panicf("FATAL 0005: could not create webhook: %s\n", err)
	}

	params := &discordgo.WebhookParams{
		Content:   msg.Content,
		Username:  fmt.Sprintf("%s || %s", msg.Author.Username, msg.Author.GlobalName),
		AvatarURL: msg.Author.AvatarURL(""),
	}
	if len(msg.Attachments) > 0 {
		params.Embeds = []*discordgo.MessageEmbed{
			{
				Title: msg.Attachments[0].Filename,
				Image: &discordgo.MessageEmbedImage{
					URL: msg.Attachments[0].URL,
				},
			},
		}
	}

	// Execute the webhook mimicing a user
	_, err = s.WebhookExecute(wh.ID, wh.Token, false, params)
	if err != nil {
		log.Printf("WARNING: could not send message: %s\n", err)
		log.Printf("WARNING: attempting to delete webhook %s\n", wh.ID)

		_, err = s.ChannelMessageSend(
			r.ChannelID,
			"Oops! Something went wrong while attempting to quote that message!",
		)
		if err != nil {
			log.Fatalf("FATAL 0006: could not send message: %s\n", err)
		}
	}

	// Clean up after yourself
	err = s.WebhookDelete(wh.ID)
	if err != nil {
		_, err = s.ChannelMessageSend(
			r.ChannelID,
			fmt.Sprintf(
				"Oops! Something went wrong while attempting to delete the webhook %s. You may way to manually delete it.",
				wh.Name,
			),
		)
		log.Printf("WARNING: could not delete webhook: %s\n", err)
		if err != nil {
			log.Fatalf("FATAL 0007: could not send message: %s\n", err)
		}
		log.Fatalf("FATAL 0008: could not delete webhook: %s", err)
	}
}

type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
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
		sysPrompt = `You are Georgibot, an AI bot in a Discord server where it is your job to maintain records of quoted messages.
You love Bulgaria and it's vibrant history, and love talking about it any chance you get. You are friendly and helpful to all requests.`
	}
	return sysPrompt
}

func getOllamaRequestData(content string) (string, string) {
	systemPrompt := getSystemPrompt()
	prefix := getPrefix()

	prompt := strings.ReplaceAll(content, prefix+",", "")
	prompt = strings.ReplaceAll(prompt, prefix+",", "")

	sysPrompt := strings.ReplaceAll(systemPrompt, "\n", " ")
	prompt = strings.ReplaceAll(prompt, "\n", " ")

	sysPrompt = strings.ReplaceAll(sysPrompt, "\r", " ")
	prompt = strings.ReplaceAll(prompt, "\r", " ")

	sysPrompt = strings.ReplaceAll(sysPrompt, "\t", " ")
	prompt = strings.ReplaceAll(prompt, "\t", " ")

	return prompt, sysPrompt
}

func answerQuestion(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || !isProperlyMentioned(m.Content) {
		return
	}

	prompt, sysPrompt := getOllamaRequestData(m.Content)

	body, err := json.Marshal(OllamaGenerateRequest{
		Model:  "qwen2.5:3b",
		Prompt: prompt,
		System: sysPrompt,
		Stream: false,
	})
	if err != nil {
		log.Printf("Error marshalling request: %s\n", err)
		return
	}

	client := &http.Client{
		Timeout: time.Second * 600,
	}

	resp, err := client.Post(
		getOllamaHost()+"/api/generate",
		"application/json",
		bytes.NewBuffer(body),
	)
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

	_, err = s.ChannelMessageSendReply(m.ChannelID, ollamaResp.Response, m.Reference())
	if err != nil {
		log.Printf("Error sending response to Discord: %s\n", err)
		_, _ = s.ChannelMessageSend(m.ChannelID, ollamaResp.Response)
	}
}

func getQuotesChannel(chns []*discordgo.Channel) (*discordgo.Channel, error) {
	for _, chn := range chns {
		if strings.ToLower(chn.Name) == "quotes" {
			return chn, nil
		}
	}
	return nil, fmt.Errorf("no quotes channel present")
}

func enableChannelCache(s *discordgo.Session, c *discordgo.Channel) {
	s.StateEnabled = true
	s.State.MaxMessageCount = 1000
	err := s.State.ChannelAdd(c)
	if err != nil {
		log.Printf("WARNING: could not add channel to state: %s\n", err)
	}
}
