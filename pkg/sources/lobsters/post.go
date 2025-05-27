package lobsters

import (
	"log/slog"
	"time"

	"github.com/go-shiori/go-readability"
)

type lobstersPost struct {
	raw       *Story
	sourceUID string
}

func (p *lobstersPost) UID() string {
	return p.raw.ID
}

func (p *lobstersPost) SourceUID() string {
	return p.sourceUID
}

func (p *lobstersPost) Title() string {
	return p.raw.Title
}

func (p *lobstersPost) Body() string {
	body := p.raw.Title
	if p.raw.URL != "" {
		article, err := readability.FromURL(p.raw.URL, 5*time.Second)
		if err == nil {
			body += "\n\nReferenced article: \n" + article.TextContent
		} else {
			slog.Error("Failed to fetch lobster article", "error", err, "url", p.raw.URL)
		}
	}
	return body
}

func (p *lobstersPost) URL() string {
	return p.raw.URL
}

func (p *lobstersPost) ImageURL() string {
	return ""
}

func (p *lobstersPost) CreatedAt() time.Time {
	return p.raw.ParsedTime
}
