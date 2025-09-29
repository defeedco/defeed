package lobsters

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
)

type Post struct {
	Post            *Story         `json:"post"`
	SourceID        types.TypedUID `json:"source_id"`
	SourceTyp       string         `json:"source_type"`
	ExternalContent string         `json:"external_content"`
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
		SourceID *lib.TypedUID `json:"source_id"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.SourceID != nil {
		p.SourceID = aux.SourceID
	}
	return nil
}

func (p *Post) UID() types.TypedUID {
	return lib.NewTypedUID(p.SourceTyp, p.Post.ID)
}

func (p *Post) SourceUID() types.TypedUID {
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
	score := float64(p.UpvotesCount())
	comments := float64(p.CommentsCount())

	scoreWeight := 0.6
	commentsWeight := 0.4

	maxScore := 500.0
	maxComments := 100.0

	normalizedScore := math.Min(score/maxScore, 1.0)
	normalizedComments := math.Min(comments/maxComments, 1.0)

	socialScore := (normalizedScore * scoreWeight) + (normalizedComments * commentsWeight)

	return math.Min(socialScore, 1.0)
}
