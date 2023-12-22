package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/mykso/myks/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	cmd := cmd.NewMyksCmd(version, commit, date)
	err := cmd.Execute()
	if err != nil {
		log.Fatal().Err(err).Msg("Error executing myks")
	}
}
