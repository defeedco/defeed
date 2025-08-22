package lobsters

import (
	"encoding/json"
	"fmt"
	"time"
)

type Post struct {
	Post            *Story `json:"post"`
	SourceID        string `json:"source_id"`
	SourceTyp       string `json:"source_type"`
	ExternalContent string `json:"external_content"`
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
	return fmt.Sprintf("%s:%s", p.SourceID, p.Post.ID)
}

func (p *Post) SourceUID() string {
	return p.SourceID
}

func (p *Post) Title() string {
	return p.Post.Title
}

func (p *Post) Body() string {
	return fmt.Sprintf("%s\n\nExternal link content:\n%s", p.Post.Title, p.ExternalContent)
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
