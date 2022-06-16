package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/api"
	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/webhook"
	"github.com/golang/glog"
	"github.com/turnage/graw/reddit"
)

const (
	eventTimeout        = time.Minute
	redditOrange        = 0xFF4500
	redditLogo          = "https://www.redditinc.com/assets/images/site/reddit-logo.png"
	maxEmbedTitle       = 256
	maxEmbedDescription = 2048
)

type filter struct {
	name         string
	subreddits   map[string]bool
	textFilter   *regexp.Regexp
	authorFilter *regexp.Regexp
}

type Alerter struct {
	webhookID    discord.WebhookID
	webhookToken string
	filters      []filter
}

func New(config *Config) (*Alerter, error) {
	webhookID, err := discord.ParseSnowflake(config.WebhookID)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook id %q: %v", config.WebhookID, err)
	}

	filters := make([]filter, 0, len(config.Filters))
	for i, f := range config.Filters {
		var textRegex, authorRegex *regexp.Regexp
		var err error
		if f.TextRegex != "" {
			textRegex, err = regexp.Compile(f.TextRegex)
			if err != nil {
				return nil, fmt.Errorf("error regex parsing filter[%d].textRegex(%q): %v", i, f.TextRegex, err)
			}
		}

		if f.AuthorRegex != "" {
			authorRegex, err = regexp.Compile(f.AuthorRegex)
			if err != nil {
				return nil, fmt.Errorf("error regex parsing filter[%d].authorRegex(%q): %v", i, f.AuthorRegex, err)
			}
		}

		subreddits := make(map[string]bool, len(f.Subreddits))
		for _, subreddit := range f.Subreddits {
			subreddits[strings.ToLower(subreddit)] = true
		}

		filters = append(filters, filter{
			name:         f.Name,
			subreddits:   subreddits,
			textFilter:   textRegex,
			authorFilter: authorRegex,
		})
	}

	return &Alerter{
		webhookID:    discord.WebhookID(webhookID),
		webhookToken: config.WebhookToken,
		filters:      filters,
	}, nil
}

type redditEvent struct {
	author         string
	filterableText string
	subreddit      string
	createdUTC     uint64
	body           string
	title          string
	permalink      string
}

func (a *Alerter) Post(post *reddit.Post) error {
	glog.V(2).Infof("Post in r/%s by u/%s: %s", post.Subreddit, post.Author, post.Title)
	return a.handleEvent(context.TODO(), redditEvent{
		author:         post.Author,
		filterableText: post.Title + post.SelfText,
		subreddit:      post.Subreddit,
		createdUTC:     post.CreatedUTC,
		body:           post.SelfText,
		title:          post.Title,
		permalink:      post.Permalink,
	})
}

func (a *Alerter) Comment(comment *reddit.Comment) error {
	glog.V(2).Infof("Comment in r/%s by u/%s: %s", comment.Subreddit, comment.Author, comment.Body)
	return a.handleEvent(context.TODO(), redditEvent{
		author:         comment.Author,
		filterableText: comment.Body,
		subreddit:      comment.Subreddit,
		createdUTC:     comment.CreatedUTC,
		body:           comment.Body,
		title:          comment.LinkTitle,
		permalink:      comment.Permalink,
	})
}

func truncate(s string, length int) string {
	runes := []rune(s)
	if len(runes) <= length {
		return s
	}
	return string(runes[:length])
}
func (a *Alerter) handleEvent(ctx context.Context, event redditEvent) error {
	ctx, cancel := context.WithTimeout(ctx, eventTimeout)
	defer cancel()

	matchingFilter := ""
	for _, f := range a.filters {
		if !f.subreddits[strings.ToLower(event.subreddit)] {
			continue
		}
		if f.textFilter != nil && !f.textFilter.MatchString(event.filterableText) {
			continue
		}
		if f.authorFilter != nil && !f.authorFilter.MatchString(event.author) {
			continue
		}
		matchingFilter = f.name
	}
	if matchingFilter == "" {
		return nil
	}

	url := fmt.Sprintf("https://reddit.com%s", event.permalink)
	glog.Infof("Matched filter %q: %s", matchingFilter, url)

	client := webhook.NewCustomClient(a.webhookID, a.webhookToken, webhook.DefaultHTTPClient.WithContext(ctx))
	_, err := client.ExecuteAndWait(api.ExecuteWebhookData{
		Embeds: []discord.Embed{
			{
				Title:       truncate(event.title, maxEmbedTitle),
				Description: truncate(event.body, maxEmbedDescription),
				URL:         url,
				Color:       discord.Color(redditOrange),
				Author: &discord.EmbedAuthor{
					Name: event.author,
					URL:  fmt.Sprintf("https://reddit.com/u/%s", event.author),
				},
				Timestamp: discord.NewTimestamp(time.Unix(int64(event.createdUTC), 0)),
				Thumbnail: &discord.EmbedThumbnail{
					URL: redditLogo,
				},
				Fields: []discord.EmbedField{
					{
						Name:  "Filter",
						Value: matchingFilter,
					},
					{
						Name:  "Link",
						Value: url,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error posting to discord webhook: %v", err)
	}
	return nil
}
