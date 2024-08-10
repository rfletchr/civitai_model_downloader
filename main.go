package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"golang.design/x/clipboard"
)

// watch_clipboard starts a goroutine to watch for changes in the clipboard and parse the text as an AirResource.
// It takes a channel of AirResources and a WaitGroup as input.
// The function uses the clipboard package to watch for changes in the clipboard's text format.
// When a change is detected, it attempts to parse the text as an AirResource using the parseAir function.
// If successful, the AirResource is sent through the channel. Any errors encountered during this process are logged with fmt.Println.
func watch_clipboard(resources chan *AirResource, wait_group *sync.WaitGroup) {

	slog.Info("Starting Clipboard Watcher")
	defer wait_group.Done()

	clip_chan := clipboard.Watch(context.TODO(), clipboard.FmtText)

	for text_as_bytes := range clip_chan {
		text_as_str := string(text_as_bytes)

		// the watcher expects a full AIR with the urn:air prefix
		if strings.ToLower(text_as_str[:7]) != "urn:air" {
			continue
		}

		res, err := parseAir(text_as_str)
		if err != nil {
			slog.Error(fmt.Sprintf("Invalid AIR: %s", text_as_str))
			continue
		}

		resources <- res

	}

}

// download_models starts a goroutine to download models based on resources received through the channel.
// It takes an API key, directory path, and a channel of AirResources as input.
// The function uses a CivitaiClient to get model version and model data for each resource.
// It then constructs a directory path using the model type, category, name, and version.
// If successful, it downloads the model version to the specified directory.
// Any errors encountered during this process are logged with slog.Error.
func download_models(api_key string, directory string, resources chan *AirResource, wait_group *sync.WaitGroup) {
	slog.Info("Waiting for models to download...")
	defer wait_group.Done()
	client := NewCivitaiClient(&api_key)

	for resource := range resources {
		slog.Info(fmt.Sprintf("Resource Found: %v", resource))
		version_data, err := client.GetModelVersion(resource.version_id)
		if err != nil {
			slog.Error(fmt.Sprintf("Error getting version data for %v: %s", resource, err))
			continue
		}

		model_data, err := client.GetModel(version_data.ModelId)
		if err != nil {
			slog.Error(fmt.Sprintf("Error getting model data for %v: Error: %s", resource, err))
			continue
		}

		type_str := strings.ReplaceAll(model_data.Type, " ", "_")
		m_name := strings.ReplaceAll(model_data.Name, " ", "_")
		v_name := strings.ReplaceAll(version_data.Name, " ", "_")
		category := strings.ReplaceAll(model_data.Category, " ", "_")

		model_directory := filepath.Join(directory, type_str, category, m_name, v_name)
		os.MkdirAll(model_directory, os.ModePerm)
		err = client.DownloadModelVersion(version_data, model_directory)

		if err != nil {
			slog.Error(fmt.Sprintf("Error downloading model '%s': %s", version_data.Name, err))
		} else {
			slog.Error("\nDownload Complete.\n")
		}
	}

}

func initConfig() {
	slog.Info("Setting Up Config.")
	config_path := os.ExpandEnv("$HOME/.config/civitai/model_downloader.yaml")
	viper.SetConfigFile(config_path)

	viper.SetDefault("api_key", "")
	viper.SetDefault("directory", os.ExpandEnv("$HOME/stable_diffusion/models"))

	if _, err := os.Stat(config_path); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(config_path), os.ModePerm)
		viper.WriteConfig()
	}

	slog.Info("Loading Config.")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func main() {
	slog.Info("Loading...")
	initConfig()

	api_key := viper.GetString("api_key")
	if api_key == "" {
		slog.Error(fmt.Sprintf("No API defined in config. Some models require you to login, these will fail"))
	}

	directory := os.ExpandEnv(viper.GetString("directory"))

	slog.Info(fmt.Sprintf("Using Directory: %s", directory))
	err := os.MkdirAll(directory, os.ModePerm)
	if err != nil {
		panic(err)
	}

	err = clipboard.Init()
	if err != nil {
		panic(err)
	}

	var wait_group sync.WaitGroup
	resources := make(chan *AirResource)

	wait_group.Add(1)
	go watch_clipboard(resources, &wait_group)

	wait_group.Add(1)
	go download_models(api_key, directory, resources, &wait_group)

	wait_group.Wait()

}
