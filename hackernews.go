package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Story struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Upvotes int    `json:"upvotes"`
}

// GetFrontPageStories returns the stories from /front?day=YYYY-MM-DD.
func GetFrontPageStories(date string) ([]Story, error) {
	pageURL := "https://news.ycombinator.com/front?day=" + url.QueryEscape(date)

	response, err := http.Get(pageURL)
	if err != nil {
		return nil, fmt.Errorf("request Hacker News front page: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Hacker News front page returned %s", response.Status)
	}

	return parseFrontPageStories(pageURL, response.Body)
}

// ParseFrontPageStories parses the HTML returned by /front?day=YYYY-MM-DD.
func ParseFrontPageStories(date, html string) ([]Story, error) {
	pageURL := "https://news.ycombinator.com/front?day=" + url.QueryEscape(date)
	return parseFrontPageStories(pageURL, strings.NewReader(html))
}

func parseFrontPageStories(pageURL string, reader io.Reader) ([]Story, error) {
	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("parse Hacker News front page URL: %w", err)
	}

	document, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("decode Hacker News HTML: %w", err)
	}

	stories := make([]Story, 0, 30)
	document.Find("tr.athing").Each(func(_ int, row *goquery.Selection) {
		titleLink := row.Find("span.titleline > a").First()
		href, ok := titleLink.Attr("href")
		if !ok {
			return
		}

		storyURL, parseErr := url.Parse(href)
		if parseErr == nil {
			href = baseURL.ResolveReference(storyURL).String()
		}

		upvotes := 0
		if fields := strings.Fields(row.NextAll().Find("span.score").First().Text()); len(fields) > 0 {
			upvotes, _ = strconv.Atoi(fields[0])
		}

		stories = append(stories, Story{
			Title:   strings.TrimSpace(titleLink.Text()),
			URL:     href,
			Upvotes: upvotes,
		})
	})

	return stories, nil
}
