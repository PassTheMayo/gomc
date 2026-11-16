package config

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func New() *Config {
	return &Config{}
}

type Config struct {
	Host                 string       `yaml:"host"`
	Port                 uint16       `yaml:"port"`
	Motd                 string       `yaml:"motd"`
	MaxPlayers           int          `yaml:"max_players"`
	OnlineMode           bool         `yaml:"online_mode"`
	CompressionThreshold int          `yaml:"compression_threshold"`
	ViewDistance         int          `yaml:"view_distance"`
	SimulationDistance   int          `yaml:"simulation_distance"`
	GameMode             string       `yaml:"game_mode"`
	Operators            []string     `yaml:"operators"`
	ServerLinks          []ServerLink `yaml:"server_links"`
	World                struct {
		Directory string `yaml:"directory"`
		Seed      int64  `yaml:"seed"`
		Generator string `yaml:"generator"`
	} `yaml:"world"`
}

// ServerLink is a link shown in the pause menu. Label is a built-in link type,
// such as website or report_bug, or text shown as it is.
type ServerLink struct {
	Label string `yaml:"label"`
	URL   string `yaml:"url"`
}

func (c *Config) ReadFile(file string) error {
	data, err := os.ReadFile(file)

	if err != nil {
		return err
	}

	if err = yaml.Unmarshal(data, c); err != nil {
		return err
	}

	c.readEnvVars()

	return nil
}

func (c *Config) readEnvVars() {
	if v := os.Getenv("HOST"); len(v) > 0 {
		c.Host = v
	}

	if v := os.Getenv("PORT"); len(v) > 0 {
		c.Port = parsePort(v)
	}

	if v := os.Getenv("MOTD"); len(v) > 0 {
		c.Motd = v
	}

	if v := os.Getenv("MAX_PLAYERS"); len(v) > 0 {
		c.MaxPlayers = int(parseInt(v, 32))
	}

	if v := os.Getenv("ONLINE_MODE"); len(v) > 0 {
		c.OnlineMode = parseBool(v)
	}

	if v := os.Getenv("COMPRESSION_THRESHOLD"); len(v) > 0 {
		c.CompressionThreshold = int(parseInt(v, 32))
	}

	if v := os.Getenv("VIEW_DISTANCE"); len(v) > 0 {
		c.ViewDistance = int(parseInt(v, 32))
	}

	if v := os.Getenv("SIMULATION_DISTANCE"); len(v) > 0 {
		c.SimulationDistance = int(parseInt(v, 32))
	}

	if v := os.Getenv("GAME_MODE"); len(v) > 0 {
		c.GameMode = v
	}

	if v := os.Getenv("OPERATORS"); len(v) > 0 {
		c.Operators = strings.Split(v, ",")
	}

	if v := os.Getenv("SERVER_LINKS"); len(v) > 0 {
		c.ServerLinks = parseServerLinks(v)
	}

	if v := os.Getenv("WORLD_DIRECTORY"); len(v) > 0 {
		c.World.Directory = v
	}

	if v := os.Getenv("WORLD_SEED"); len(v) > 0 {
		c.World.Seed = parseInt(v, 64)
	}

	if v := os.Getenv("WORLD_GENERATOR"); len(v) > 0 {
		c.World.Generator = v
	}
}

// parseServerLinks reads comma separated label=url pairs.
func parseServerLinks(v string) []ServerLink {
	var links []ServerLink

	for _, pair := range strings.Split(v, ",") {
		label, url, ok := strings.Cut(pair, "=")

		if !ok {
			panic("config: SERVER_LINKS entries must be label=url")
		}

		links = append(links, ServerLink{Label: strings.TrimSpace(label), URL: strings.TrimSpace(url)})
	}

	return links
}

func parsePort(v string) uint16 {
	value, err := strconv.ParseUint(v, 10, 16)

	if err != nil {
		panic(err)
	}

	return uint16(value)
}

func parseInt(v string, bitSize int) int64 {
	value, err := strconv.ParseInt(v, 10, bitSize)

	if err != nil {
		panic(err)
	}

	return value
}

func parseBool(v string) bool {
	value, err := strconv.ParseBool(v)

	if err != nil {
		panic(err)
	}

	return value
}
