package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/KaueAbade/melissa-bot/internal/app"
	"github.com/KaueAbade/melissa-bot/internal/commands"
	"github.com/KaueAbade/melissa-bot/internal/env"
	"github.com/bwmarrin/discordgo"
)

// loadConfigFromEnv retrieves the bot configuration from environment variables and validates them.
func loadConfigFromEnv() (*app.Config, error) {
	// Get the bot token from an environment variable
	discordToken := env.GetStr("DISCORD_BOT_TOKEN", "")
	if discordToken == "" {
		return nil, errors.New("No token provided\nIt is necessary to set the DISCORD_BOT_TOKEN environment variable")
	}

	return &app.Config{
		DiscordToken:  discordToken,
		CommandWipe:   env.GetBool("WIPE_COMMANDS_ON_EXIT", false),
		Debug:         env.GetBool("DEBUG", false),
		DesiredLocale: discordgo.Locale(env.GetStr("LOCALE", string(discordgo.EnglishUS))),
	}, nil
}

// init sets up the bot configuration, initializes the command registry, and prepares the runtime environment.
func init() {
	log.Println("Setting up Melissa Bot...")
	config, err := loadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	// Validate command consistency at startup
	registry := commands.GetRegistry()
	if err := registry.ValidateCommands(); err != nil {
		log.Fatalf("Command validation failed: %v", err)
	}

	app.SetupSession(config)
}

// main is the entrypoint for the bot application, starts the runtime and waits for termination signals
func main() {
	// Get the runtime instance and start it
	runtime := app.GetRuntime()
	if err := runtime.Run(); err != nil {
		log.Fatal(err)
	}

	// Wait here until a sigterm is received
	quitSignal := make(chan os.Signal, 1)
	signal.Notify(quitSignal, os.Interrupt, syscall.SIGTERM)
	<-quitSignal
	runtime.Shutdown()
}
