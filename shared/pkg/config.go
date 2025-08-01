package pkg

import (
	"log/slog"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	UserServer      UserServer      `yaml:"user-server"`
	PostServer      PostServer      `yaml:"post-server"`
	Database        Database        `yaml:"database"`
	ProcessorServer ProcessorServer `yaml:"processor-server"`
	Queue           Queue           `yaml:"queue"`
}

type PostServer struct {
	Port int `yaml:"port"`
}

type ProcessorServer struct {
	Port int `yaml:"port"`
}

type UserServer struct {
	Port int `yaml:"port"`
}

type Queue struct {
	Brokers  []string `yaml:"brokers"`
	Topic    string   `yaml:"topic"`
	GroupID  string   `yaml:"group_id"`
	Username string   `yaml:"username"`
}

type Database struct {
	Username string `yaml:"username"`
	// Token           string        `yaml:"token"` -- use .env
	Path     string `yaml:"path"`
	Keyspace string `yaml:"keyspace"`
}

func (c *Config) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read config file", "error", err, "path", path)
		return err
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		slog.Error("failed to unmarshal config file", "error", err, "path", path)
		return err
	}

	return nil

}
