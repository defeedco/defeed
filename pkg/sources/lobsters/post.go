package lobsters

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-shiori/go-readability"
)

type Post struct {
	Post      *Story `json:"post"`
	SourceID  string `json:"source_id"`
	SourceTyp string `json:"source_type"`
}

func NewPost() *Post {
	return &Post{}
}

func (p *Post) SourceType() string {
	return p.SourceTyp
}

func (p *Post) MarshalJSON() ([]byte, error) {
	type Alias Post
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	})
}

func (p *Post) UnmarshalJSON(data []byte) error {
	type Alias Post
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	return json.Unmarshal(data, &aux)
}

func (p *Post) UID() string {
	return p.Post.ID
}

func (p *Post) SourceUID() string {
	return p.SourceID
}

func (p *Post) Title() string {
	return p.Post.Title
}

func (p *Post) Body() string {
	body := p.Post.Title
	if p.Post.URL != "" {
		article, err := readability.FromURL(p.Post.URL, 5*time.Second)
		if err == nil {
			body += "\n\nReferenced article: \n" + article.TextContent
		} else {
			slog.Error("Failed to fetch lobster article", "error", err, "url", p.Post.URL)
		}
	}
	return body
}

func (p *Post) URL() string {
	return p.Post.URL
}

func (p *Post) ImageURL() string {
	return ""
}

func (p *Post) CreatedAt() time.Time {
	return p.Post.ParsedTime
}
