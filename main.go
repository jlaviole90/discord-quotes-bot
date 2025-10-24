package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

func main() {
	session, err := discordgo.New(
		"Bot " + os.Getenv("DISCORD_TOKEN"),
	)
	if err != nil {
		log.Fatalf("Could not create session. Check your token: %s", err)
	}

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as %s", r.User.String())
	})

	// quote.go
	session.AddHandler(Quote)

	// inference.go
	session.AddHandler(Inference)

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
