package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/golang/glog"
)

type Filter struct {
	Name        string   `json:"name"`
	Subreddits  []string `json:"subreddits"`
	TextRegex   string   `json:"textRegex"`
	AuthorRegex string   `json:"authorRegex"`
}

type Config struct {
	WebhookID       string   `json:"webhookID"`
	WebhookToken    string   `json:"webhookToken"`
	RedditUserAgent string   `json:"redditUserAgent"`
	Filters         []Filter `json:"filters"`
}

func LoadConfig(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("error opening config file %q: %v", file, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			glog.Warningf("Failed to close config file %q: %v", file, err)
		}
	}()
	config := new(Config)
	if err := json.NewDecoder(f).Decode(config); err != nil {
		return nil, fmt.Errorf("error parsing config file %q: %v", file, err)
	}
	return config, nil
}
