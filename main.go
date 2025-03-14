package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

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
			log.Fatalf("could not get message: %s", err)
		}

		usr := msg.Author

		chns, err := s.GuildChannels(r.GuildID)
		if err != nil {
			log.Fatalf("could not get channels: %s", err)
		}

		has := false
		for _, chn := range chns {
            // Only post in the quotes channel
			if chn.Name == "quotes" {
				has = true

                // Create a new webhook
				wh, err := s.WebhookCreate(chn.ID, usr.Username, usr.AvatarURL(""))
				if err != nil {
					log.Fatalf("could not create webhook: %s", err)
				}

                // Execute the webhook mimicing a user
                _, err = s.WebhookExecute(wh.ID,wh.Token,false,&discordgo.WebhookParams{
                    Content: msg.Content,
                    Username: fmt.Sprintf("%s || %s",usr.Username, usr.GlobalName),
                    AvatarURL: usr.AvatarURL(""),
                })
                if err != nil {
                    log.Fatalf("could not send message: %s", err)
                }

                // Clean up after yourself
                err = s.WebhookDelete(wh.ID)
                if err != nil {
                    log.Fatalf("could not delete webhook: %s", err)
                }
			}
		}
        // If we didn't find a quotes channel, send a message so the user can know to create one
		if !has {
			_, err = s.ChannelMessageSend(r.ChannelID, "Sorry, I couldn't find the quotes channel!")
            if err != nil {
                log.Fatalf("could not send message: %s", err)
            }
		}
	}
