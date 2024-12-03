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
