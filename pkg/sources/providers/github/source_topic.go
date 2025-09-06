package github

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/glanceapp/glance/pkg/lib"
    activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
    "github.com/google/go-github/v72/github"
    "github.com/rs/zerolog"
)

const TypeGithubTopic = "githubtopic"

// SourceTopic fetches repositories for a single GitHub topic (tag)
// It can return either trending repositories (by stars) or newly created repositories.
type SourceTopic struct {
    Topic string `json:"topic" validate:"required"`
    // Mode controls how repositories are selected: "trending" or "new"
    // Defaults to "trending" when empty
    Mode  string `json:"mode" validate:"omitempty,oneof=new trending"`
    // MinStars is used only in trending mode to filter low-signal repositories
    MinStars int `json:"minStars"`
    Token    string `json:"token"`

    client *github.Client
    logger *zerolog.Logger
}

func NewSourceTopic() *SourceTopic {
    return &SourceTopic{
        Mode:     "trending",
        MinStars: 100,
    }
}

func (s *SourceTopic) UID() activitytypes.TypedUID {
    return lib.NewTypedUID(TypeGithubTopic, s.Topic, s.getMode())
}

func (s *SourceTopic) Name() string {
    return fmt.Sprintf("GitHub #%s repositories", s.Topic)
}

func (s *SourceTopic) Description() string {
    switch s.getMode() {
    case "new":
        return fmt.Sprintf("New repositories tagged #%s", s.Topic)
    default:
        return fmt.Sprintf("Trending repositories tagged #%s", s.Topic)
    }
}

func (s *SourceTopic) URL() string {
    return fmt.Sprintf("https://github.com/topics/%s", s.Topic)
}

func (s *SourceTopic) Icon() string {
    return "https://github.com/favicon.ico"
}

func (s *SourceTopic) Initialize(logger *zerolog.Logger) error {
    if err := lib.ValidateStruct(s); err != nil {
        return err
    }

    token := s.Token
    if token == "" {
        token = os.Getenv("GITHUB_TOKEN")
    }

    if token != "" {
        s.client = github.NewClient(nil).WithAuthToken(token)
    } else {
        s.client = github.NewClient(nil)
    }

    s.logger = logger
    return nil
}

func (s *SourceTopic) Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
    s.fetchTopicRepositories(ctx, since, feed, errs)
}

func (s *SourceTopic) fetchTopicRepositories(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, errs chan<- error) {
    mode := s.getMode()

    var sinceDate string
    if since != nil {
        sinceDate = since.CreatedAt().Format(time.DateOnly)
    } else {
        // Default lookback window
        switch mode {
        case "new":
            sinceDate = time.Now().AddDate(0, 0, -14).Format(time.DateOnly)
        default:
            sinceDate = time.Now().AddDate(0, -1, 0).Format(time.DateOnly)
        }
    }

    // Build search query
    var queryBuilder strings.Builder
    // topic filter
    queryBuilder.WriteString("topic:")
    queryBuilder.WriteString(s.Topic)
    // created filter
    if sinceDate != "" {
        queryBuilder.WriteString(" ")
        queryBuilder.WriteString("created:>")
        queryBuilder.WriteString(sinceDate)
    }
    // stars filter for trending
    if mode == "trending" && s.MinStars > 0 {
        queryBuilder.WriteString(" ")
        queryBuilder.WriteString("stars:>")
        queryBuilder.WriteString(fmt.Sprintf("%d", s.MinStars))
    }

    searchQuery := queryBuilder.String()

    opts := &github.SearchOptions{
        ListOptions: github.ListOptions{
            PerPage: 10,
        },
        Order: "desc",
    }
    switch mode {
    case "new":
        opts.Sort = "created"
    default:
        opts.Sort = "stars"
    }

    s.logger.Debug().
        Str("topic", s.Topic).
        Str("mode", mode).
        Str("query", searchQuery).
        Msg("Searching GitHub repositories by topic")

    page := 1
    for {
        opts.Page = page
        result, _, err := s.client.Search.Repositories(ctx, searchQuery, opts)
        if err != nil {
            errs <- fmt.Errorf("search repositories: %w", err)
            return
        }

        if len(result.Repositories) == 0 {
            break
        }

        s.logger.Debug().
            Int("count", len(result.Repositories)).
            Int("page", page).
            Msg("Fetched repositories for topic")

        for _, repo := range result.Repositories {
            // Safety checks
            if repo.Owner == nil || repo.Owner.Login == nil || repo.Name == nil {
                continue
            }
            activity := &RepositoryActivity{
                Repository: repo,
                SourceTyp:  TypeGithubTopic,
                SourceID:   s.UID(),
            }
            feed <- activity
        }

        page++
    }
}

func (s *SourceTopic) getMode() string {
    if s.Mode == "" {
        return "trending"
    }
    if s.Mode == "new" {
        return "new"
    }
    return "trending"
}

func (s *SourceTopic) MarshalJSON() ([]byte, error) {
    type Alias SourceTopic
    return json.Marshal(&struct {
        *Alias
        Type string `json:"type"`
    }{
        Alias: (*Alias)(s),
        Type:  TypeGithubTopic,
    })
}

func (s *SourceTopic) UnmarshalJSON(data []byte) error {
    type Alias SourceTopic
    aux := &struct {
        *Alias
        Type string `json:"type"`
    }{
        Alias: (*Alias)(s),
    }
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }
    return nil
}

// RepositoryActivity represents a repository result as an activity
type RepositoryActivity struct {
    Repository *github.Repository `json:"repository"`
    SourceID   activitytypes.TypedUID `json:"source_id"`
    SourceTyp  string `json:"source_type"`
}

func NewRepositoryActivity() *RepositoryActivity {
    return &RepositoryActivity{}
}

func (a *RepositoryActivity) SourceType() string {
    return a.SourceTyp
}

func (a *RepositoryActivity) MarshalJSON() ([]byte, error) {
    type Alias RepositoryActivity
    return json.Marshal(&struct {
        *Alias
    }{
        Alias: (*Alias)(a),
    })
}

func (a *RepositoryActivity) UnmarshalJSON(data []byte) error {
    type Alias RepositoryActivity
    aux := &struct {
        *Alias
        SourceID *lib.TypedUID `json:"source_id"`
    }{
        Alias: (*Alias)(a),
    }
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }
    if aux.SourceID == nil {
        return fmt.Errorf("source_id is required")
    }
    a.SourceID = aux.SourceID
    return nil
}

func (a *RepositoryActivity) UID() activitytypes.TypedUID {
    fullName := a.Repository.GetFullName()
    if fullName == "" {
        fullName = fmt.Sprintf("%s/%s", a.Repository.GetOwner().GetLogin(), a.Repository.GetName())
    }
    return lib.NewTypedUID(TypeGithubTopic, fullName)
}

func (a *RepositoryActivity) SourceUID() activitytypes.TypedUID {
    return a.SourceID
}

func (a *RepositoryActivity) Title() string {
    if a.Repository.FullName != nil {
        return *a.Repository.FullName
    }
    return a.Repository.GetName()
}

func (a *RepositoryActivity) Body() string {
    return a.Repository.GetDescription()
}

func (a *RepositoryActivity) URL() string {
    return a.Repository.GetHTMLURL()
}

func (a *RepositoryActivity) ImageURL() string {
    owner := a.Repository.GetOwner().GetLogin()
    repo := a.Repository.GetName()
    created := a.CreatedAt().Unix()
    return fmt.Sprintf("https://opengraph.githubassets.com/%d/%s/%s", created, owner, repo)
}

func (a *RepositoryActivity) CreatedAt() time.Time {
    if a.Repository.CreatedAt != nil {
        return a.Repository.GetCreatedAt().Time
    }
    if a.Repository.PushedAt != nil {
        return a.Repository.GetPushedAt().Time
    }
    if a.Repository.UpdatedAt != nil {
        return a.Repository.GetUpdatedAt().Time
    }
    return time.Now()
}

