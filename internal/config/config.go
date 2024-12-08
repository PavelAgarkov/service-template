package config

import (
	"flag"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"sync"
)

type Config struct {
	HTTP struct {
		Port string `yaml:"port"`
	}
	DB struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	}
}

var configPath string
var instance *Config
var once sync.Once

func GetConfig() *Config {
	once.Do(func() {
		flag.StringVar(
			&configPath,
			"config",
			"config.yaml",
			"this is app config file",
		)
		flag.Parse()

		instance = &Config{}
		if err := cleanenv.ReadConfig(configPath, instance); err != nil {
			log.Fatal(err)
		}
	})
	return instance
}
