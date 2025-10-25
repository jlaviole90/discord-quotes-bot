package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func Quote(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	// Guard clause against non-quote reactions
	if r.MessageReaction.Emoji.Name != "📸" &&
		r.MessageReaction.Emoji.Name != ":camera_with_flash:" {
		return
	}

	msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
	if err != nil {
		log.Printf("FATAL 0001: could not get message: %s\n", err)
		return
	}

	// Don't quote bots
	if msg.Author.Bot && msg.Author.ID != s.State.User.ID {
		_, _ = s.ChannelMessageSend(r.ChannelID, "Sorry, I don't quote application messages!")
		return
	}

	chns, err := s.GuildChannels(r.GuildID)
	if err != nil {
		log.Printf("FATAL 0003: could not get channels: %s\n", err)
		return
	}

	// Find the quotes channel
	// If we don't find a quotes channel, send a message so the user can know to create one
	qchn, err := getQuotesChannel(chns)
	if err != nil {
		_, _ = s.ChannelMessageSend(r.ChannelID, "Sorry, I couldn't find the quotes channel!")
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
		Username:  msg.Member.DisplayName(),
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
		log.Printf("WARNING: could not execute webhook: %s\n", err)

		_, _ = s.ChannelMessageSend(
			r.ChannelID,
			"Oops! Something went wrong while attempting to quote that message!",
		)
	}

	// Clean up after yourself
	err = s.WebhookDelete(wh.ID)
	if err != nil {
		_, _ = s.ChannelMessageSend(
			r.ChannelID,
			fmt.Sprintf(
				"Oops! Something went wrong while attempting to delete the webhook %s. You may want to manually delete it.",
				wh.Name,
			),
		)
		log.Printf("WARNING: could not delete webhook: %s\n", err)
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
