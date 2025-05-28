package mastodon

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-mastodon"
	"golang.org/x/net/html"
)

type mastodonPost struct {
	raw       *mastodon.Status
	sourceUID string
}

func (p *mastodonPost) UID() string {
	return string(p.raw.ID)
}

func (p *mastodonPost) SourceUID() string {
	return p.sourceUID
}

func (p *mastodonPost) Title() string {
	if p.raw.Card != nil {
		return p.raw.Card.Title
	}

	return oneLineTitle(p.Body(), 50)
}

func (p *mastodonPost) Body() string {
	return extractTextFromHTML(p.raw.Content)
}

func (p *mastodonPost) URL() string {
	return p.raw.URL
}

func (p *mastodonPost) ImageURL() string {
	if len(p.raw.MediaAttachments) > 0 {
		return p.raw.MediaAttachments[0].URL
	}
	return ""
}

func (p *mastodonPost) CreatedAt() time.Time {
	return p.raw.CreatedAt
}

func extractTextFromHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return strings.TrimSpace(b.String())
}

func oneLineTitle(text string, maxLen int) string {
	re := regexp.MustCompile(`\s+`)
	t := re.ReplaceAllString(text, " ")
	t = strings.TrimSpace(t)
	if utf8.RuneCountInString(t) > maxLen {
		runes := []rune(t)
		return string(runes[:maxLen-1]) + "…"
	}
	return t
}
