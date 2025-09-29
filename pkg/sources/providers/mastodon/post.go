package mastodon

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/mattn/go-mastodon"
	"golang.org/x/net/html"
)

type Post struct {
	Status    *mastodon.Status `json:"status"`
	SourceID  types.TypedUID   `json:"source_id"`
	SourceTyp string           `json:"source_type"`
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

	if aux.SourceID == nil {
		return fmt.Errorf("source_id is required")
	}

	p.SourceID = aux.SourceID
	return nil
}

func (p *Post) UID() types.TypedUID {
	return lib.NewTypedUID(p.SourceTyp, string(p.Status.ID))
}

func (p *Post) SourceUID() types.TypedUID {
	return p.SourceID
}

func (p *Post) Title() string {
	if p.Status.Card != nil {
		return p.Status.Card.Title
	}

	return oneLineTitle(p.Body(), 50)
}

func (p *Post) Body() string {
	if p.Status.Content != "" {
		return extractTextFromHTML(p.Status.Content)
	}
	if p.Status.Reblog != nil && p.Status.Reblog.Content != "" {
		reblogAcct := p.Status.Reblog.Account.Acct
		body := extractTextFromHTML(p.Status.Reblog.Content)
		return "Reblogged " + reblogAcct + "'s post: " + body
	}
	return ""
}

func (p *Post) URL() string {
	if p.Status.Content != "" {
		return p.Status.URL
	}
	return p.Status.Reblog.URL
}

func (p *Post) ImageURL() string {
	if len(p.Status.MediaAttachments) > 0 {
		return p.Status.MediaAttachments[0].URL
	}
	return ""
}

func (p *Post) CreatedAt() time.Time {
	return p.Status.CreatedAt
}

func (p *Post) UpvotesCount() int {
	return int(p.Status.FavouritesCount)
}

func (p *Post) DownvotesCount() int {
	return -1
}

func (p *Post) CommentsCount() int {
	return int(p.Status.RepliesCount)
}

func (p *Post) AmplificationCount() int {
	return int(p.Status.ReblogsCount)
}

func (p *Post) SocialScore() float64 {
	favorites := float64(p.UpvotesCount())
	reblogs := float64(p.AmplificationCount())
	replies := float64(p.CommentsCount())

	favoritesWeight := 0.4
	reblogsWeight := 0.4
	repliesWeight := 0.2

	maxFavorites := 500.0
	maxReblogs := 100.0
	maxReplies := 50.0

	normalizedFavorites := math.Min(favorites/maxFavorites, 1.0)
	normalizedReblogs := math.Min(reblogs/maxReblogs, 1.0)
	normalizedReplies := math.Min(replies/maxReplies, 1.0)

	socialScore := (normalizedFavorites * favoritesWeight) + (normalizedReblogs * reblogsWeight) + (normalizedReplies * repliesWeight)

	return math.Min(socialScore, 1.0)
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
