package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	sourcetypes "github.com/glanceapp/glance/pkg/sources/types"

	"github.com/glanceapp/glance/pkg/sources/providers/changedetection"
	"github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	"github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	"github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"

	"github.com/glanceapp/glance/pkg/feeds"
	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/sources/nlp"
	httpswagger "github.com/swaggo/http-swagger"

	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/glanceapp/glance/pkg/sources"
	"github.com/rs/zerolog"
)

//go:embed openapi.yaml
var openapiSpecYaml string

type UserIDContextKey string

const userIDContextKey UserIDContextKey = "userID"

type Server struct {
	executor     *sources.Executor
	registry     *sources.Registry
	feedRegistry *feeds.Registry
	logger       *zerolog.Logger
	http         http.Server
}

var _ ServerInterface = (*Server)(nil)

func NewServer(logger *zerolog.Logger, cfg *Config, db *postgres.DB) (*Server, error) {
	limiter := lib.NewOpenAILimiter(logger)

	summarizerModel, err := openai.New(
		openai.WithModel("gpt-5-nano-2025-08-07"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return nil, fmt.Errorf("create summarizer model: %w", err)
	}

	embedderModel, err := openai.New(
		openai.WithEmbeddingModel("text-embedding-3-large"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return nil, fmt.Errorf("create embedder model: %w", err)
	}

	summarizer := nlp.NewSummarizer(summarizerModel)
	embedder := nlp.NewEmbedder(embedderModel)

	executor := sources.NewExecutor(
		logger,
		summarizer,
		embedder,
		postgres.NewActivityRepository(db),
		postgres.NewSourceRepository(db),
	)
	if err := executor.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize executor: %w", err)
	}

	registry := sources.NewRegistry(logger)
	if err := registry.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize preset registry: %w", err)
	}

	feedStore := postgres.NewFeedRepository(db)
	feedRegistry := feeds.NewRegistry(feedStore, executor, registry, summarizer)

	mux := http.NewServeMux()

	server := &Server{
		logger:       logger,
		registry:     registry,
		executor:     executor,
		feedRegistry: feedRegistry,
		http: http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler: authMiddleware(corsMiddleware(mux, cfg.CORSOrigin)),
		},
	}

	HandlerFromMux(server, mux)
	server.registerApiDocsHandlers(mux)

	return server, nil
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Docs should be public
		if strings.HasPrefix(r.URL.Path, "/docs") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			// For the time being, we allow unauthenticated requests but with certain limitations.
			emptyUserID := ""
			ctx := context.WithValue(r.Context(), userIDContextKey, emptyUserID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "invalid authorization header", http.StatusUnauthorized)
			return
		}

		authToken := strings.TrimPrefix(authHeader, "Bearer ")
		if authToken == "" {
			http.Error(w, "invalid auth token", http.StatusUnauthorized)
			return
		}

		// TODO(auth): authorize when user resource/auth is implemented
		userID := authToken

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func (s *Server) ListFeedActivities(w http.ResponseWriter, r *http.Request, uid string, params ListFeedActivitiesParams) {
	userID := r.Context().Value(userIDContextKey).(string)

	var queryOverride string
	if params.Query != nil {
		queryOverride = *params.Query
	}

	limit := 20
	if params.Limit != nil {
		limit = *params.Limit
	}

	sortBy, err := deserializeSortBy(params.SortBy)
	if err != nil {
		s.badRequest(w, err, "deserialize sort by")
		return
	}

	out, err := s.feedRegistry.Activities(r.Context(), uid, userID, sortBy, limit, queryOverride)
	if err != nil {
		s.internalError(w, err, "list feed activities")
		return
	}

	activities, err := serializeActivities(out)
	if err != nil {
		s.internalError(w, err, "serialize activities")
		return
	}

	s.serializeRes(w, activities)
}

func (s *Server) ListSources(w http.ResponseWriter, r *http.Request, params ListSourcesParams) {
	var query string
	if params.Query != nil {
		query = *params.Query
	}

	result, err := s.registry.Search(r.Context(), query)
	if err != nil {
		s.internalError(w, err, "search source presets")
		return
	}

	res, err := serializeSources(result)
	if err != nil {
		s.internalError(w, err, "serialize sources")
		return
	}

	s.serializeRes(w, res)
}

func (s *Server) GetSource(w http.ResponseWriter, r *http.Request, uid string) {
	typedUID, err := sources.NewTypedUID(uid)
	if err != nil {
		s.badRequest(w, err, "deserialize source UID")
		return
	}

	out, err := s.registry.FindByUID(r.Context(), typedUID)
	if err != nil {
		s.internalError(w, err, fmt.Sprintf("find source by UID: %s", typedUID.String()))
		return
	}

	source, err := serializeSource(out)
	if err != nil {
		s.internalError(w, err, "serialize source")
		return
	}

	s.serializeRes(w, source)
}

func (s *Server) CreateOwnFeed(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDContextKey).(string)

	var req CreateFeedRequest
	err := deserializeReq(r, &req)
	if err != nil {
		s.badRequest(w, err, "deserialize request")
		return
	}

	sourceUIDs, err := deserializeSourceUIDs(req.SourceUids)
	if err != nil {
		s.badRequest(w, err, "deserialize source UIDs")
		return
	}

	createReq := feeds.CreateRequest{
		Name:       req.Name,
		Icon:       req.Icon,
		Query:      req.Query,
		SourceUIDs: sourceUIDs,
		UserID:     userID,
	}

	createdFeed, err := s.feedRegistry.Create(r.Context(), createReq)
	if err != nil {
		s.internalError(w, err, "create feed")
		return
	}

	s.serializeRes(w, serializeFeed(createdFeed))
}

func (s *Server) ListFeeds(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDContextKey).(string)

	feedList, err := s.feedRegistry.ListByUserID(r.Context(), userID)
	if err != nil {
		s.internalError(w, err, "list feeds")
		return
	}

	s.serializeRes(w, serializeFeeds(feedList))
}

func (s *Server) UpdateOwnFeed(w http.ResponseWriter, r *http.Request, uid string) {
	var req UpdateFeedRequest
	err := deserializeReq(r, &req)
	if err != nil {
		s.badRequest(w, err, "deserialize request")
		return
	}

	userID := r.Context().Value(userIDContextKey).(string)

	sourceUIDs, err := deserializeSourceUIDs(req.SourceUids)
	if err != nil {
		s.badRequest(w, err, "deserialize source UIDs")
		return
	}
	updatedFeed, err := s.feedRegistry.Update(r.Context(), feeds.UpdateRequest{
		ID:         uid,
		UserID:     userID,
		Name:       req.Name,
		Icon:       req.Icon,
		Query:      req.Query,
		SourceUIDs: sourceUIDs,
	})
	if err != nil {
		s.internalError(w, err, "update feed")
		return
	}

	s.serializeRes(w, serializeFeed(updatedFeed))
}

func (s *Server) DeleteOwnFeed(w http.ResponseWriter, r *http.Request, uid string) {
	userID := r.Context().Value(userIDContextKey).(string)

	err := s.feedRegistry.Remove(r.Context(), uid, userID)
	if err != nil {
		s.internalError(w, err, "delete feed")
		return
	}

	s.serializeRes(w, map[string]string{"message": "Feed deleted successfully"})
}

func (s *Server) GetFeedSummary(w http.ResponseWriter, r *http.Request, feedID string, params GetFeedSummaryParams) {
	userID := r.Context().Value(userIDContextKey).(string)

	var queryOverride string
	if params.Query != nil {
		queryOverride = *params.Query
	}

	feedSummary, err := s.feedRegistry.Summary(r.Context(), feedID, userID, queryOverride)
	if err != nil {
		if errors.Is(err, feeds.ErrInsufficientActivity) {
			s.statusCode(w, http.StatusAccepted)
			return
		}

		s.internalError(w, err, "generate feed summary")
		return
	}

	s.serializeRes(w, serializeFeedSummary(feedSummary))
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

func (s *Server) statusCode(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
}

func serializeFeedSummary(in *activitytypes.ActivitiesSummary) FeedSummary {
	highlights := make([]FeedHighlight, 0, len(in.Highlights))
	for _, h := range in.Highlights {
		highlights = append(highlights, FeedHighlight{
			Content:           h.Content,
			SourceActivityIds: h.SourceActivityIDs,
		})
	}

	return FeedSummary{
		Overview:   in.Overview,
		Highlights: highlights,
		CreatedAt:  in.CreatedAt.Format(time.RFC3339),
	}
}

func serializeFeeds(in []*feeds.Feed) []Feed {
	out := make([]Feed, len(in))
	for i, f := range in {
		out[i] = serializeFeed(f)
	}
	return out
}

func serializeFeed(in *feeds.Feed) Feed {
	return Feed{
		Uid:        in.ID,
		Name:       in.Name,
		Icon:       in.Icon,
		Query:      in.Query,
		IsPublic:   in.Public,
		SourceUids: serializeSourceUIDs(in.SourceUIDs),
	}
}

func serializeSourceUIDs(in []activitytypes.TypedUID) []string {
	out := make([]string, len(in))
	for i, uid := range in {
		out[i] = uid.String()
	}
	return out
}

func serializeActivities(in []*activitytypes.DecoratedActivity) ([]*Activity, error) {
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

func serializeActivity(in *activitytypes.DecoratedActivity) (*Activity, error) {
	sourceType, err := serializeSourceType(in.Activity.SourceUID().Type())
	if err != nil {
		return nil, fmt.Errorf("serialize source type: %w", err)
	}

	return &Activity{
		Body:         in.Activity.Body(),
		CreatedAt:    in.Activity.CreatedAt(),
		ImageUrl:     in.Activity.ImageURL(),
		FullSummary:  in.Summary.FullSummary,
		ShortSummary: in.Summary.ShortSummary,
		SourceUid:    in.Activity.SourceUID().String(),
		SourceType:   sourceType,
		Title:        in.Activity.Title(),
		Uid:          in.Activity.UID().String(),
		Url:          in.Activity.URL(),
		Similarity:   &in.Similarity,
	}, nil
}

func serializeSources(in []sourcetypes.Source) ([]Source, error) {
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

func serializeSource(in sourcetypes.Source) (Source, error) {
	sourceType, err := serializeSourceType(in.UID().Type())
	if err != nil {
		return Source{}, fmt.Errorf("serialize source type: %w", err)
	}

	return Source{
		Uid:         in.UID().String(),
		Type:        sourceType,
		Url:         in.URL(),
		Name:        in.Name(),
		Description: in.Description(),
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

func deserializeSourceUIDs(in []string) ([]activitytypes.TypedUID, error) {
	out := make([]activitytypes.TypedUID, len(in))
	for i, uid := range in {
		uid, err := sources.NewTypedUID(uid)
		if err != nil {
			return nil, fmt.Errorf("deserialize source UID: %w", err)
		}
		out[i] = uid
	}
	return out, nil
}

func deserializeSortBy(in *ActivitySortBy) (activitytypes.SortBy, error) {
	if in == nil {
		return activitytypes.SortByDate, nil
	}

	switch *in {
	case CreationDate:
		return activitytypes.SortByDate, nil
	case Similarity:
		return activitytypes.SortBySimilarity, nil
	}

	return "", fmt.Errorf("unknown sort by: %s", *in)
}
