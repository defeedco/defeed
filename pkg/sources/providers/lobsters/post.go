package lobsters

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/providers"
)

type Post struct {
	Post            *Story           `json:"post"`
	SourceIDs       []types.TypedUID `json:"source_ids"`
	SourceTyp       string           `json:"source_type"`
	ExternalContent string           `json:"external_content"`
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
		SourceIDs []*lib.TypedUID `json:"source_ids"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if len(aux.SourceIDs) == 0 {
		return fmt.Errorf("source_ids is required")
	}

	p.SourceIDs = make([]types.TypedUID, len(aux.SourceIDs))
	for i, uid := range aux.SourceIDs {
		p.SourceIDs[i] = uid
	}

	return nil
}

func (p *Post) UID() types.TypedUID {
	return lib.NewTypedUID(p.SourceTyp, p.Post.ID)
}

func (p *Post) SourceUIDs() []types.TypedUID {
	return p.SourceIDs
}

func (p *Post) Title() string {
	return p.Post.Title
}

func (p *Post) Body() string {
	return fmt.Sprintf("%s\n\nExternal link content:\n%s", p.Post.Title, p.ExternalContent)
}

func (p *Post) URL() string {
	if p.Post.ShortIDURL == "" {
		// Old entries are missing ShortIDURL,
		// so we need to extract it from the CommentsURL.
		parts := strings.Split(p.Post.CommentsURL, "/")
		lastIndex := len(parts) - 1
		return strings.Join(parts[:lastIndex], "/")
	}
	return p.Post.ShortIDURL
}

func (p *Post) ImageURL() string {
	return ""
}

func (p *Post) CreatedAt() time.Time {
	return p.Post.CreatedAt
}

func (p *Post) UpvotesCount() int {
	return p.Post.Score
}

func (p *Post) DownvotesCount() int {
	return -1
}

func (p *Post) CommentsCount() int {
	return p.Post.CommentCount
}

func (p *Post) AmplificationCount() int {
	return -1
}

func (p *Post) SocialScore() float64 {
	upvotes := float64(p.UpvotesCount())
	comments := float64(p.CommentsCount())

	scoreWeight := 0.6
	commentsWeight := 0.4

	maxUpvotes := 500.0
	maxComments := 100.0

	return (providers.NormSocialScore(upvotes, maxUpvotes) * scoreWeight) +
		(providers.NormSocialScore(comments, maxComments) * commentsWeight)
}
