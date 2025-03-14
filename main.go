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
		"Bot " + "",
	)

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}

		data := i.ApplicationCommandData()
		if data.Name != "echo" {
			return
		}
	})

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
			if chn.Name == "quotes" {
				has = true

				wh, err := s.WebhookCreate(chn.ID, usr.Username, usr.AvatarURL(""))
				if err != nil {
					log.Fatalf("could not create webhook: %s", err)
				}

                p := discordgo.WebhookParams{
                    Content: msg.Content,
                    Username: fmt.Sprintf("%s || %s",usr.Username, usr.GlobalName),
                    AvatarURL: usr.AvatarURL(""),
                }
                _, err = s.WebhookExecute(wh.ID,wh.Token,false,&p)
                if err != nil {
                    log.Fatalf("could not send message: %s", err)
                }

                err = s.WebhookDelete(wh.ID)
                if err != nil {
                    log.Fatalf("could not delete webhook: %s", err)
                }
			}
			if err != nil {
				log.Fatalf("could not send message: %s", err)
			}
		}
		if !has {
			_, err = s.ChannelMessageSend(r.ChannelID, "Sorry, I couldn't find the quotes channel!")
            if err != nil {
                log.Fatalf("could not send message: %s", err)
            }
		}
	}
