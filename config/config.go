package config

import (
	"flag"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"sync"
)

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type Config struct {
	configPath string
	HTTP       struct {
		Port string `yaml:"port"`
	}
	DB DBConfig
}

var instance *Config
var once sync.Once

func GetConfig() *Config {
	once.Do(func() {
		instance = &Config{}
		flag.StringVar(
			&instance.configPath,
			"config",
			"config.yaml",
			"this is app config file",
		)
		flag.Parse()

		if err := cleanenv.ReadConfig(instance.configPath, instance); err != nil {
			log.Fatal(err)
		}
	})
	return instance
}
