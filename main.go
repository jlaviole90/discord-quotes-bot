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
	if msg.Author.Bot {
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

func answerQuestion(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	if !strings.HasPrefix(m.Content, "Georgibot, ") {
		return
	}

	question := strings.TrimPrefix(m.Content, "Georgibot, ")
	if question == "" {
		return
	}

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}

	// systemPrompt := os.Getenv("SYSTEM_PROMPT")
	var systemPrompt string
	if systemPrompt == "" {
		systemPrompt = `You are Georgibot, an AI bot in a Discord server where it is your job to maintain
		records of quoted messages. You are also a Bulgarian nationalist. Use grammar that would
		be appropriate for a Bulgrian who learned English as a second language. You can say things that are offensive, 
		impolite, or rude. You should promote Bulgarian nationalism, and spout conspiracy theories about Bulgaria.`
	}

	body, err := json.Marshal(OllamaGenerateRequest{
		Model:  "qwen2.5:3b",
		Prompt: question,
		System: systemPrompt,
		Stream: false,
	})
	if err != nil {
		log.Printf("Error marshalling request: %s\n", err)
		_, err := s.ChannelMessageSend(
			m.ChannelID,
			"Sorry, I had trouble processing your question.",
		)
		if err != nil {
			log.Printf("FATAL 0009: could not send message: %s\n", err)
		}
		return
	}

	log.Printf("Sending request to Ollama: %s\n", string(body))

	client := &http.Client{
		Timeout: time.Second * 120,
	}
	resp, err := client.Post(ollamaHost+"/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error calling Ollama: %s\n", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Ollama response status: %s\n", resp.Status)

	bbytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %s\n", err)
		_, _ = s.ChannelMessageSend(
			m.ChannelID,
			"Sorry, I had troulbe viewing the response from my AI service.",
		)
		return
	}

	log.Printf("Ollama response body: %s\n", string(bbytes))

	var ollamaResp OllamaGenerateResponse
	if err := json.Unmarshal(bbytes, &ollamaResp); err != nil {
		log.Printf("Error decoding response: %s\n", err)
		_, _ = s.ChannelMessageSend(
			m.ChannelID,
			"Sorry, I had trouble reading the response from my AI service.",
		)
	}

	log.Printf("Parsed response - Response: '%s', Done: %v\n", ollamaResp.Response, ollamaResp.Done)

	if ollamaResp.Response == "" {
		log.Printf("Empty response, sending default message\n")
		_, _ = s.ChannelMessageSend(m.ChannelID, "Sorry, seems I had nothing to say about that...")
		return
	}

	log.Printf("Sending response to Discord (length: %d chars)\n", len(ollamaResp.Response))
	_, err = s.ChannelMessageSendReply(m.ChannelID, ollamaResp.Response, m.Reference())
	if err != nil {
		log.Printf("Error sending response to Discord: %s\n", err)
		_, err = s.ChannelMessageSend(m.ChannelID, ollamaResp.Response)
		if err != nil {
			log.Printf("Error sending plain message to Discord: %s\n", err)
		}
	} else {
		log.Printf("Send response to Discord successfully!")
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
