package main

import (
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"main/src/config"
	"main/src/server"
	"main/src/worldgen"
)

var conf *config.Config = config.New()

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Read the configuration file.
	if err := conf.ReadFile("config.yml"); err != nil {
		panic(err)
	}

	log.Info("Successfully loaded configuration file")

	if conf.ViewDistance < 2 || conf.ViewDistance > 32 {
		panic(errors.New("view_distance must be between 2 and 32"))
	}

	if conf.SimulationDistance < 2 || conf.SimulationDistance > 32 {
		panic(errors.New("simulation_distance must be between 2 and 32"))
	}

	if conf.MaxPlayers < 1 {
		panic(errors.New("max_players must be at least 1"))
	}

	if conf.World.Generator != worldgen.NameTerrain && conf.World.Generator != worldgen.NameFlat {
		panic(errors.New("world.generator must be terrain or flat"))
	}

	// Open the world and prepare the spawn area.
	srv, err := server.New(conf, log)

	if err != nil {
		panic(err)
	}

	// Accept connections until the process is asked to stop.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error("Server stopped accepting connections", "error", err)
			os.Exit(1)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals

	log.Info("Shutting down")

	if err := srv.Shutdown(); err != nil {
		log.Error("Failed to save the world", "error", err)
		os.Exit(1)
	}
}
