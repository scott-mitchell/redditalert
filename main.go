package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/turnage/graw"
	"github.com/turnage/graw/reddit"
	"golang.org/x/oauth2"
)

var (
	configFile    = flag.String("config-file", "config.json", "The config file to load")
	agentFile     = flag.String("agent-file", "agent.yaml", "The agent file to load")
	pollingPeriod = flag.Duration("polling-period", 5*time.Minute, "The period to poll for new submissions and comments")
	delay429      = flag.Duration("delay-429", 5*time.Minute, "How long to wait on 429 oauth2 errors before exiting.")
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
	// Too make deployment for me simpler, wait in the app on 429s before exiting to avoid flooding reddit.
	var retErr *oauth2.RetrieveError
	if errors.As(err, &retErr) && retErr.Response.StatusCode == http.StatusTooManyRequests {
		glog.Errorf("Got 429 from oauth, waiting %v before exiting", *delay429)
		time.Sleep(*delay429)
		glog.Fatalf("Oauth error starting reddit scan: %v", err)
	}
	if err != nil {
		glog.Fatalf("Failed to start Reddit scan: %v", err)
	}
	defer stop()
	if err := wait(); err != nil {
		glog.Fatalf("Reddit Scan failed: %v", err)
	}

}
