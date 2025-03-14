package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type optionMap = map[string]*discordgo.ApplicationCommandInteractionDataOption

func parseOptions(options []*discordgo.ApplicationCommandInteractionDataOption) (om optionMap) {
	om = make(optionMap)
	for _, opt := range options {
		om[opt.Name] = opt
	}
	return
}

func interationAuthor(i *discordgo.Interaction) *discordgo.User {
	if i.Member != nil {
		return i.Member.User
	}
	return i.User
}

func handleEcho(s *discordgo.Session, i *discordgo.InteractionCreate, opts optionMap) {
	builder := new(strings.Builder)
	if v, ok := opts["author"]; ok && v.BoolValue() {
		author := interactionAuthor(i.Interaction)
		builder.WriteString("**" + author + "** says: ")
	}

	builder.WriteString(opts["message"].StringValue())

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: builder.String(),
		},
	})
	if err != nil {
		log.Panicf("could not respond to interaction: %s", err)
	}
}

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "echo",
		Description: "Say something through a bot",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "message",
				Description: "Contents of the message",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
			{
				Name:        "author",
				Description: "Whether to prepend message's author",
				Type:        discordgo.ApplicationCommandOptionBoolean,
			},
		},
	},
}

var (
	Token = flag.String("token", "", "Bot authentication token")
	App   = flag.String("app", "", "Application ID")
	Guild = flag.String("guild", "", "Guild ID")
)

func main() {
	flag.Parse()
	if *App == "" {
		log.Fatal("application id is not set")
	}

	os.Getenv("DISCORD_TOKEN")

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

		handleEcho(s, i, parseOptions(data.Options))
	})

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as %s", r.User.String())
	})

	session.AddHandler(func(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
		if r.MessageReaction.Emoji.Name != "📸" ||
			r.MessageReaction.Emoji.Name != ":camera_with_flash:" {
			return
		}

		usr, err := s.User(r.UserID)
		if err != nil {
			log.Fatalf("could not get user: %s", err)
		}

		msg, err := s.ChannelMessage(r.ChannelID, r.MessageID)
		if err != nil {
			log.Fatalf("could not get message: %s", err)
		}

		chns, err := s.GuildChannels(r.GuildID)
		if err != nil {
			log.Fatalf("could not get channels: %s", err)
		}
		for _, chn := range chns {
			if chn.Name == "quotes" {
				s.ChannelMessageSend(chn.ID, fmt.Sprintf("%s: %s", usr.Username, msg.Content))
			} else {
				s.ChannelMessageSend(r.ChannelID, fmt.Sprint("Cannot find quotes channel!"))
			}
		}
	})

	_, err := session.ApplicationCommandBulkOverwrite(*App, *Guild, commands)
	if err != nil {
		log.Fatalf("could not register commands: %s", err)
	}

	err = session.Open()
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
