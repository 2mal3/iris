package main

import (
	"log/slog"
	"os"

	"github.com/2mal3/iris/pkg"

	"gopkg.in/yaml.v3"
)

func main() {
	var config pkg.Config

	slog.Info("Loading config ...")
	if err := loadConfig(&config); err != nil {
		slog.Error(err.Error())
		return
	}

	pkg.Main(config)
}

func loadConfig(configVar *pkg.Config) error {
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlFile, configVar)
	if err != nil {
		return err
	}

	return nil
}
