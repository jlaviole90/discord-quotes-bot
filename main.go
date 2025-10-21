package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

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

	// Execute the webhook mimicing a user
	_, err = s.WebhookExecute(wh.ID, wh.Token, false, &discordgo.WebhookParams{
		Content:   msg.Content,
		Username:  fmt.Sprintf("%s || %s", msg.Author.Username, msg.Author.GlobalName),
		AvatarURL: msg.Author.AvatarURL(""),
		Components: msg.Components,
		Embeds: msg.Embeds,
		Attachments: msg.Attachments,
	})
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
