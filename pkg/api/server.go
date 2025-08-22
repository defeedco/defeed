package api

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/presets"
	"github.com/glanceapp/glance/pkg/sources/providers/changedetection"
	"github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	"github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	"github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"
	"html/template"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/sources/nlp"
	httpswagger "github.com/swaggo/http-swagger"

	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/glanceapp/glance/pkg/sources"
	"github.com/rs/zerolog"
)

//go:embed openapi.yaml
var openapiSpecYaml string

type Server struct {
	executor       *sources.Executor
	presetRegistry *presets.Registry
	logger         *zerolog.Logger
	http           http.Server
}

var _ ServerInterface = (*Server)(nil)

func NewServer(logger *zerolog.Logger, cfg *Config, db *postgres.DB) (*Server, error) {
	summarizerModel, err := openai.New(
		openai.WithModel("gpt-5-nano-2025-08-07"),
	)
	if err != nil {
		return nil, fmt.Errorf("create summarizer model: %w", err)
	}

	embedderModel, err := openai.New(
		openai.WithEmbeddingModel("text-embedding-3-large"),
	)
	if err != nil {
		return nil, fmt.Errorf("create embedder model: %w", err)
	}

	executor := sources.NewExecutor(
		logger,
		nlp.NewSummarizer(summarizerModel),
		nlp.NewEmbedder(embedderModel),
		postgres.NewActivityRepository(db),
		postgres.NewSourceRepository(db),
	)
	if err := executor.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize executor: %w", err)
	}

	presetRegistry := presets.NewRegistry(logger)
	if err := presetRegistry.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize preset registry: %w", err)
	}

	mux := http.NewServeMux()

	server := &Server{
		logger:         logger,
		presetRegistry: presetRegistry,
		executor:       executor,
		http: http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler: corsMiddleware(mux, cfg.CORSOrigin),
		},
	}

	HandlerFromMux(server, mux)
	server.registerApiDocsHandlers(mux)

	return server, nil
}

func corsMiddleware(next http.Handler, originConfig string) http.Handler {
	origins := strings.Split(originConfig, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestOrigin := r.Header.Get("Origin")

		if len(origins) == 1 && origins[0] == "*" {
			// Allow all origins
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if requestOrigin != "" && slices.Contains(origins, requestOrigin) {
			// CORS doesn't support multiple origins,
			// so we either set the origin in the header or not at all.
			w.Header().Set("Access-Control-Allow-Origin", requestOrigin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerApiDocsHandlers(mux *http.ServeMux) {
	mux.Handle("/docs/", httpswagger.Handler(
		httpswagger.URL("/docs/openapi.yaml"),
	))
	mux.HandleFunc("/docs/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")

		_, err := w.Write([]byte(openapiSpecYaml))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Error().Err(err).Msg("response write error")
		}
	})
}
func (s *Server) Start() error {
	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop() error {
	return s.http.Close()
}

func (s *Server) ListAllActivities(w http.ResponseWriter, r *http.Request) {
	out, err := s.executor.Activities()
	if err != nil {
		s.internalError(w, err, "list activities")
		return
	}

	activities, err := serializeActivities(out)
	if err != nil {
		s.internalError(w, err, "serialize activities")
		return
	}

	s.serializeRes(w, activities)
}

func (s *Server) ListSources(w http.ResponseWriter, r *http.Request) {
	out, err := s.presetRegistry.Sources()
	if err != nil {
		s.internalError(w, err, "list sources")
		return
	}

	res, err := serializeSources(out)
	if err != nil {
		s.internalError(w, err, "serialize sources")
		return
	}

	s.serializeRes(w, res)
}

func (s *Server) CreateSource(w http.ResponseWriter, r *http.Request) {
	var req CreateSourceRequest
	err := deserializeReq(r, &req)
	if err != nil {
		s.badRequest(w, err, "deserialize request")
		return
	}

	out, err := deserializeCreateSourceRequest(req)
	if err != nil {
		s.badRequest(w, err, "deserialize request")
		return
	}

	err = s.executor.Add(out)
	if err != nil {
		s.internalError(w, err, "add source")
		return
	}

	source, err := serializeSource(out)
	if err != nil {
		s.internalError(w, err, "serialize source")
		return
	}

	s.serializeRes(w, source)
}

func (s *Server) DeleteSource(w http.ResponseWriter, r *http.Request, uid string) {
	err := s.executor.Remove(uid)
	if err != nil {
		s.internalError(w, err, "remove source")
		return
	}

	s.serializeRes(w, nil)
}

func (s *Server) GetSource(w http.ResponseWriter, r *http.Request, uid string) {
	out, err := s.executor.Source(uid)
	if err != nil {
		s.internalError(w, err, "remove source")
		return
	}

	source, err := serializeSource(out)
	if err != nil {
		s.internalError(w, err, "serialize source")
		return
	}

	s.serializeRes(w, source)
}

func (s *Server) GetActivitiesSummary(w http.ResponseWriter, r *http.Request, params GetActivitiesSummaryParams) {
	var query string
	if params.Query != nil {
		query = *params.Query
	}

	sortBy, err := deserializeSortBy(params.SortBy)
	if err != nil {
		s.badRequest(w, err, "deserialize sort by")
		return
	}

	sourceIDs := strings.Split(params.Sources, ",")

	summary, err := s.executor.Summary(r.Context(), query, sourceIDs, sortBy)
	if err != nil {
		s.internalError(w, err, "generate summary")
		return
	}

	highlights := make([]ActivityHighlight, 0, len(summary.Highlights))
	for _, h := range summary.Highlights {
		highlights = append(highlights, ActivityHighlight{
			Content:           h.Content,
			SourceActivityIds: h.SourceActivityIDs,
		})
	}

	s.serializeRes(w, ActivitiesSummary{
		Overview:   summary.Overview,
		Highlights: highlights,
	})
}

func (s *Server) SearchActivities(w http.ResponseWriter, r *http.Request, params SearchActivitiesParams) {
	var sourceUIDs []string
	if params.Sources != nil {
		sourceUIDs = strings.Split(*params.Sources, ",")
	}

	var minSimilarity float32
	if params.MinSimilarity != nil {
		minSimilarity = *params.MinSimilarity
	}

	var query string
	if params.Query != nil {
		query = *params.Query
	}

	limit := 20
	if params.Limit != nil {
		if *params.Limit < 1 || *params.Limit > 100 {
			s.badRequest(w, fmt.Errorf("limit must be between 1 and 100"), "validate limit")
			return
		}
		limit = *params.Limit
	}

	sortBy, err := deserializeSortBy(params.SortBy)
	if err != nil {
		s.badRequest(w, err, "deserialize sort by")
		return
	}

	results, err := s.executor.Search(r.Context(), query, sourceUIDs, minSimilarity, limit, sortBy)
	if err != nil {
		s.internalError(w, err, "search activities")
		return
	}

	activities, err := serializeActivities(results)
	if err != nil {
		s.internalError(w, err, "serialize activities")
		return
	}

	s.serializeRes(w, activities)
}

func deserializeReq[Req any](r *http.Request, req *Req) error {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return fmt.Errorf("unsupported content type: %s", contentType)
	}

	reqBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}

	err = json.Unmarshal(reqBytes, req)
	if err != nil {
		return fmt.Errorf("deserialize request body: %w", err)
	}

	return nil
}

func (s *Server) serializeRes(w http.ResponseWriter, res any) {
	w.Header().Add("Content-Type", "application/json")

	if res == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		s.internalError(w, err, "serialize response")
	}
}

func (s *Server) internalError(w http.ResponseWriter, err error, msg string) {
	s.logger.Err(err).Msg(msg)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) badRequest(w http.ResponseWriter, err error, msg string) {
	s.logger.Err(err).Msg(msg)
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func deserializeCreateSourceRequest(req CreateSourceRequest) (sources.Source, error) {
	disc, err := req.Discriminator()
	if err != nil {
		return nil, fmt.Errorf("read discriminator: %w", err)
	}

	sourceType, err := deserializeSourceType(SourceType(disc))
	if err != nil {
		return nil, fmt.Errorf("deserialize source type: %w", err)
	}

	source, err := sources.NewSource(sourceType)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}

	val, err := req.ValueByDiscriminator()
	if err != nil {
		return nil, fmt.Errorf("extract typed request by discriminator: %w", err)
	}

	var configBytes []byte
	switch v := val.(type) {
	case CreateSourceRequestMastodonAccount:
		configBytes, err = json.Marshal(v.MastodonAccount)
	case CreateSourceRequestMastodonTag:
		configBytes, err = json.Marshal(v.MastodonTag)
	case CreateSourceRequestHackernewsPosts:
		configBytes, err = json.Marshal(v.HackernewsPosts)
	case CreateSourceRequestRedditSubreddit:
		configBytes, err = json.Marshal(v.RedditSubreddit)
	case CreateSourceRequestLobstersTag:
		configBytes, err = json.Marshal(v.LobstersTag)
	case CreateSourceRequestLobstersFeed:
		configBytes, err = json.Marshal(v.LobstersFeed)
	case CreateSourceRequestRssFeed:
		configBytes, err = json.Marshal(v.RssFeed)
	case CreateSourceRequestGithubReleases:
		configBytes, err = json.Marshal(v.GithubReleases)
	case CreateSourceRequestGithubIssues:
		configBytes, err = json.Marshal(v.GithubIssues)
	case CreateSourceRequestChangedetectionWebsite:
		configBytes, err = json.Marshal(v.ChangedetectionWebsite)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", disc)
	}
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	if err := json.Unmarshal(configBytes, source); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if verrs := source.Validate(); len(verrs) > 0 {
		// join all validation errors into one error
		var sb strings.Builder
		for i, e := range verrs {
			if i > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(e.Error())
		}
		return nil, fmt.Errorf("invalid source config: %s", sb.String())
	}

	return source, nil
}

func deserializeSourceType(in SourceType) (string, error) {
	switch in {
	case MastodonAccount:
		return mastodon.TypeMastodonAccount, nil
	case MastodonTag:
		return mastodon.TypeMastodonTag, nil
	case HackernewsPosts:
		return hackernews.TypeHackerNewsPosts, nil
	case RedditSubreddit:
		return reddit.TypeRedditSubreddit, nil
	case LobstersTag:
		return lobsters.TypeLobstersTag, nil
	case LobstersFeed:
		return lobsters.TypeLobstersFeed, nil
	case RssFeed:
		return rss.TypeRSSFeed, nil
	case GithubReleases:
		return github.TypeGithubReleases, nil
	case GithubIssues:
		return github.TypeGithubIssues, nil
	case ChangedetectionWebsite:
		return changedetection.TypeChangedetectionWebsite, nil
	}

	return "", fmt.Errorf("unknown source type: %s", in)
}

func serializeActivities(in []*types.DecoratedActivity) ([]*Activity, error) {
	out := make([]*Activity, 0, len(in))

	for _, e := range in {
		activity, err := serializeActivity(e)
		if err != nil {
			return nil, fmt.Errorf("serialize activity: %w", err)
		}
		out = append(out, activity)
	}

	return out, nil
}

func serializeActivity(in *types.DecoratedActivity) (*Activity, error) {
	sourceType, err := serializeSourceType(in.Activity.SourceType())
	if err != nil {
		return nil, fmt.Errorf("serialize source type: %w", err)
	}

	return &Activity{
		Body:         in.Activity.Body(),
		CreatedAt:    in.Activity.CreatedAt(),
		ImageUrl:     in.Activity.ImageURL(),
		FullSummary:  in.Summary.FullSummary,
		ShortSummary: in.Summary.ShortSummary,
		SourceUid:    in.Activity.SourceUID(),
		SourceType:   sourceType,
		Title:        in.Activity.Title(),
		Uid:          in.Activity.UID(),
		Url:          in.Activity.URL(),
		Similarity:   &in.Similarity,
	}, nil
}

func serializeSources(in []sources.Source) ([]Source, error) {
	out := make([]Source, 0, len(in))

	for _, e := range in {
		source, err := serializeSource(e)
		if err != nil {
			return nil, fmt.Errorf("serialize source: %w", err)
		}
		out = append(out, source)
	}

	return out, nil
}

func serializeSource(in sources.Source) (Source, error) {
	sourceType, err := serializeSourceType(in.Type())
	if err != nil {
		return Source{}, fmt.Errorf("serialize source type: %w", err)
	}

	return Source{
		Uid:  in.UID(),
		Type: sourceType,
		Url:  in.URL(),
		Name: in.Name(),
	}, nil
}

func serializeSourceType(in string) (SourceType, error) {
	switch in {
	case mastodon.TypeMastodonAccount:
		return MastodonAccount, nil
	case mastodon.TypeMastodonTag:
		return MastodonTag, nil
	case hackernews.TypeHackerNewsPosts:
		return HackernewsPosts, nil
	case reddit.TypeRedditSubreddit:
		return RedditSubreddit, nil
	case lobsters.TypeLobstersTag:
		return LobstersTag, nil
	case lobsters.TypeLobstersFeed:
		return LobstersFeed, nil
	case rss.TypeRSSFeed:
		return RssFeed, nil
	case github.TypeGithubReleases:
		return GithubReleases, nil
	case github.TypeGithubIssues:
		return GithubIssues, nil
	case changedetection.TypeChangedetectionWebsite:
		return ChangedetectionWebsite, nil
	}

	return "", fmt.Errorf("unknown source type: %s", in)
}

func deserializeSortBy(in *ActivitySortBy) (types.SortBy, error) {
	if in == nil {
		return types.SortByDate, nil
	}

	switch *in {
	case CreationDate:
		return types.SortByDate, nil
	case Similarity:
		return types.SortBySimilarity, nil
	}

	return "", fmt.Errorf("unknown sort by: %s", *in)
}

func fileServerWithCache(fs http.FileSystem, cacheDuration time.Duration) http.Handler {
	server := http.FileServer(fs)
	cacheControlValue := fmt.Sprintf("public, max-age=%d", int(cacheDuration.Seconds()))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: fix always setting cache control even if the file doesn't exist
		w.Header().Set("Cache-Control", cacheControlValue)
		server.ServeHTTP(w, r)
	})
}

func renderTemplate(t *template.Template, data any) (string, error) {
	var b bytes.Buffer
	err := t.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return b.String(), nil
}
