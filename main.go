package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
)

var (
	configFile    = flag.String("config-file", "config.json", "The config file to load")
	agentFile     = flag.String("agent-file", "agent.yaml", "The agent file to load")
	pollingPeriod = flag.Duration("polling-period", 5*time.Minute, "The period to poll for new submissions and comments")
)

func main() {
	flag.Parse()
	glog.Infof("Starting...")

	cfg, err := LoadConfig(*configFile)
	if err != nil {
		glog.Fatalf("Failed to load config: %v", err)
	}
	glog.Infof("Using config %+v", cfg)

	uniqueSubreddits := make(map[string]bool)
	for _, filter := range cfg.Filters {
		for _, subreddit := range filter.Subreddits {
			uniqueSubreddits[strings.ToLower(subreddit)] = true
		}
	}
	subreddits := make([]string, 0, len(uniqueSubreddits))
	for subreddit := range uniqueSubreddits {
		subreddits = append(subreddits, subreddit)
	}
	alerter, err := New(cfg)
	if err != nil {
		glog.Fatalf("Failed to create alerter: %v", err)
	}

	bot, err := reddit.NewBotFromAgentFile(*agentFile, *pollingPeriod)
	if err != nil {
		glog.Fatalf("Failed to create Reddit script: %v", err)
	}

	glog.Infof("Starting scan...")
	stop, wait, err := graw.Run(alerter, bot, graw.Config{
		Subreddits:        subreddits,
		SubredditComments: subreddits,
		Logger:            log.New(os.Stderr, "[Graw]", log.LstdFlags),
	})
	if err != nil {
		glog.Fatalf("Failed to start Reddit scan: %v", err)
	}
	defer stop()
	if err := wait(); err != nil {
		glog.Fatalf("Reddit Scan failed: %v", err)
	}

}
