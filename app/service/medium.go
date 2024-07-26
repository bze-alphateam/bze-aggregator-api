package service

import (
	"encoding/json"
	"errors"
	"github.com/bze-alphateam/bze-aggregator-api/app/dto"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

const (
	FeedURL = "https://medium.com/feed/bzedge-community"

	numOfArticles       = 6
	articleContentLimit = 150
)

type Medium struct {
	logger logrus.FieldLogger
	cache  Cache

	htmlPolicy *bluemonday.Policy
}

func NewMediumService(logger logrus.FieldLogger, cache Cache) (*Medium, error) {
	if logger == nil || cache == nil {
		return nil, errors.New("invalid dependencies provided to medium service")
	}

	policy := bluemonday.StrictPolicy()

	return &Medium{logger: logger, cache: cache, htmlPolicy: policy}, nil
}

func (m *Medium) GetLatestArticles() []dto.Article {
	cacheValue, err := m.cache.Get(FeedURL)
	if err != nil {
		m.logger.Errorf("failed to articles from cache: %v", err)
	}

	if cacheValue != nil {
		var articles []dto.Article
		err = json.Unmarshal(cacheValue, &articles)
		if err != nil {
			m.logger.Errorf("failed to unmarshal articles from cache: %v", err)
		} else {
			return articles
		}
	}

	articles := m.fetchLatestArticles()
	encoded, err := json.Marshal(articles)
	if err != nil {
		m.logger.Errorf("failed to marshal articles in order to cache them: %v", err)

		return articles
	}

	err = m.cache.Set(FeedURL, encoded, time.Duration(cacheExpireSeconds)*time.Second)
	if err != nil {
		m.logger.Errorf("failed to cache articles: %v", err)
	}

	return articles
}

func (m *Medium) fetchLatestArticles() []dto.Article {
	// Fetch the RSS feed
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(FeedURL)
	if err != nil {
		m.logger.Errorf("failed to fetch latest articles from %s: %v", FeedURL, err)

		return []dto.Article{}
	}

	var articles []dto.Article
	for i, item := range feed.Items {
		if i >= numOfArticles {
			break
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		description := m.extractShortDescription(content, articleContentLimit)

		authorName := ""
		if len(item.Authors) > 0 {
			authorName = item.Authors[0].Name
		}

		articles = append(articles, dto.Article{
			Title:       item.Title,
			URL:         item.Link,
			Description: description,
			PublishDate: *item.PublishedParsed,
			AuthorName:  authorName,
			PictureURL:  m.extractFirstImageUrl(content),
		})
	}

	return articles
}

func (m *Medium) extractShortDescription(content string, limit int) string {
	stripped := m.htmlPolicy.Sanitize(content)
	if len(stripped) > limit {
		return stripped[:limit] + "..."
	}

	return stripped
}

func (m *Medium) extractFirstImageUrl(content string) string {
	// Define a regular expression to match <img> tags and capture the src attribute
	imgRegex := regexp.MustCompile(`<img[^>]+src="([^">]+)"`)

	// Find the first match
	var foundUrl string
	matches := imgRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		// The second element in the matches slice is the captured src URL
		foundUrl = matches[1]
	}

	if strings.Contains(foundUrl, "https://cdn") {
		return foundUrl
	}

	return ""
}
