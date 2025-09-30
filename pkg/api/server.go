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

	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"

	"github.com/defeedco/defeed/pkg/api/auth"
	"github.com/defeedco/defeed/pkg/sources/providers/github"
	"github.com/defeedco/defeed/pkg/sources/providers/hackernews"
	"github.com/defeedco/defeed/pkg/sources/providers/lobsters"
	"github.com/defeedco/defeed/pkg/sources/providers/mastodon"
	"github.com/defeedco/defeed/pkg/sources/providers/reddit"
	"github.com/defeedco/defeed/pkg/sources/providers/rss"

	"github.com/defeedco/defeed/pkg/feeds"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	httpswagger "github.com/swaggo/http-swagger"

	"github.com/defeedco/defeed/pkg/sources"
	"github.com/rs/zerolog"
)

//go:embed openapi.yaml
var openapiSpecYaml string

type Server struct {
	sourceScheduler *sources.Scheduler
	sourceRegistry  sourceRegistry
	feedRegistry    *feeds.Registry
	logger          *zerolog.Logger
	http            http.Server
}

type sourceRegistry interface {
	FindByUID(ctx context.Context, uid activitytypes.TypedUID) (sourcetypes.Source, error)
	Search(ctx context.Context, params sources.SearchRequest) ([]sourcetypes.Source, error)
}

var _ ServerInterface = (*Server)(nil)

func NewServer(
	logger *zerolog.Logger,
	config *Config,
	authMiddleware *auth.RouteAuthMiddleware,
	sourceRegistry sourceRegistry,
	sourceScheduler *sources.Scheduler,
	feedRegistry *feeds.Registry,
) (*Server, error) {
	mux := http.NewServeMux()

	server := &Server{
		logger:          logger,
		sourceRegistry:  sourceRegistry,
		sourceScheduler: sourceScheduler,
		feedRegistry:    feedRegistry,
		http: http.Server{
			Addr:    fmt.Sprintf("%s:%d", config.Host, config.Port),
			Handler: authMiddleware.Middleware(corsMiddleware(mux, config.CORSOrigin)),
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

func (s *Server) ListFeedActivities(w http.ResponseWriter, r *http.Request, uid string, params ListFeedActivitiesParams) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		s.internalError(w, err, "get user from context")
		return
	}

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

	period := deserializePeriod(params.Period)

	out, err := s.feedRegistry.Activities(r.Context(), uid, user.UserID, sortBy, limit, queryOverride, period)
	if err != nil {
		s.internalError(w, err, "list feed activities")
		return
	}

	activities, err := serializeActivities(out.Results)
	if err != nil {
		s.internalError(w, err, "serialize activities")
		return
	}

	topics, err := serializeTopics(out.Topics)
	if err != nil {
		s.internalError(w, err, "serialize topics")
		return
	}

	s.serializeRes(w, ActivitiesListResponse{
		Results: *activities,
		Topics:  *topics,
	})
}

func (s *Server) ListSources(w http.ResponseWriter, r *http.Request, params ListSourcesParams) {
	var query string
	if params.Query != nil {
		query = *params.Query
	}

	var topics []sourcetypes.TopicTag
	if params.Topics != nil {
		res, err := deserializeTopicTags(*params.Topics)
		if err != nil {
			s.badRequest(w, err, "deserialize topics")
			return
		}
		topics = res
	}

	result, err := s.sourceRegistry.Search(r.Context(), sources.SearchRequest{
		Query:  query,
		Topics: topics,
	})
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

	out, err := s.sourceRegistry.FindByUID(r.Context(), typedUID)
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
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		s.internalError(w, err, "get user from context")
		return
	}

	var req CreateFeedRequest
	err = deserializeReq(r, &req)
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
		UserID:     user.UserID,
	}

	createdFeed, err := s.feedRegistry.Create(r.Context(), createReq)
	if err != nil {
		s.internalError(w, err, "create feed")
		return
	}

	s.serializeRes(w, serializeFeed(createdFeed))
}

func (s *Server) ListFeeds(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		s.internalError(w, err, "get user from context")
		return
	}

	feedList, err := s.feedRegistry.ListByUserID(r.Context(), user.UserID)
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

	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		s.internalError(w, err, "get user from context")
		return
	}

	sourceUIDs, err := deserializeSourceUIDs(req.SourceUids)
	if err != nil {
		s.badRequest(w, err, "deserialize source UIDs")
		return
	}
	updatedFeed, err := s.feedRegistry.Update(r.Context(), feeds.UpdateRequest{
		ID:         uid,
		UserID:     user.UserID,
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
	user, err := auth.UserFromContext(r.Context())
	if err != nil {
		s.internalError(w, err, "get user from context")
		return
	}

	err = s.feedRegistry.Remove(r.Context(), uid, user.UserID)
	if err != nil {
		s.internalError(w, err, "delete feed")
		return
	}

	s.serializeRes(w, map[string]string{"message": "Feed deleted successfully"})
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
		CreatedBy:  in.UserID,
		CreatedAt:  in.CreatedAt,
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

func serializeActivities(in []*activitytypes.DecoratedActivity) (*[]Activity, error) {
	out := make([]Activity, 0, len(in))

	for _, e := range in {
		activity, err := serializeActivity(e)
		if err != nil {
			return nil, fmt.Errorf("serialize activity: %w", err)
		}
		out = append(out, *activity)
	}

	return &out, nil
}

func serializeTopics(in []*feeds.Topic) (*[]ActivityTopic, error) {
	out := make([]ActivityTopic, 0, len(in))

	for _, topic := range in {
		serializedTopic := &ActivityTopic{
			Title:       topic.Title,
			Emoji:       topic.Emoji,
			Summary:     topic.Summary,
			Queries:     topic.Queries,
			ActivityIds: topic.ActivityIDs,
		}
		out = append(out, *serializedTopic)
	}

	return &out, nil
}

func serializeActivity(in *activitytypes.DecoratedActivity) (*Activity, error) {
	sourceType, err := serializeSourceType(in.Activity.SourceUID().Type())
	if err != nil {
		return nil, fmt.Errorf("serialize source type: %w", err)
	}

	return &Activity{
		Body:               in.Activity.Body(),
		CreatedAt:          in.Activity.CreatedAt(),
		ImageUrl:           in.Activity.ImageURL(),
		FullSummary:        in.Summary.FullSummary,
		ShortSummary:       in.Summary.ShortSummary,
		SourceUid:          in.Activity.SourceUID().String(),
		SourceType:         sourceType,
		Title:              in.Activity.Title(),
		Uid:                in.Activity.UID().String(),
		Url:                in.Activity.URL(),
		Similarity:         &in.Similarity,
		UpvotesCount:       in.Activity.UpvotesCount(),
		CommentsCount:      in.Activity.CommentsCount(),
		AmplificationCount: in.Activity.AmplificationCount(),
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

	// Map internal topic tags to API TopicTag
	apiTags := make([]TopicTag, 0)
	for _, t := range in.Topics() {
		apiTags = append(apiTags, TopicTag(t))
	}

	return Source{
		Uid:         in.UID().String(),
		Type:        sourceType,
		Url:         in.URL(),
		IconUrl:     in.Icon(),
		Name:        in.Name(),
		Description: in.Description(),
		TopicTags:   apiTags,
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
	case github.TypeGithubTopic:
		return GithubTopics, nil
		// Note: temporarily removed in commit a8c728a86cefadd20f67a424363dc6f61c41cf66
		// case changedetection.TypeChangedetectionWebsite:
		// return ChangedetectionWebsite, nil
	}

	return "", fmt.Errorf("unknown source type: %s", in)
}

func deserializeTopicTags(in []TopicTag) ([]sourcetypes.TopicTag, error) {
	out := make([]sourcetypes.TopicTag, len(in))
	for i, t := range in {
		des, err := deserializeTopicTag(t)
		if err != nil {
			return nil, fmt.Errorf("deserialize topic tag: %w", err)
		}
		out[i] = des
	}
	return out, nil
}

func deserializeTopicTag(in TopicTag) (sourcetypes.TopicTag, error) {
	switch in {
	case AgenticSystems:
		return sourcetypes.TopicAgenticSystems, nil
	case Llms:
		return sourcetypes.TopicLLMs, nil
	case Startups:
		return sourcetypes.TopicStartups, nil
	case Devtools:
		return sourcetypes.TopicDevTools, nil
	case WebPerformance:
		return sourcetypes.TopicWebPerformance, nil
	case DistributedSystems:
		return sourcetypes.TopicDistributedSystems, nil
	case Databases:
		return sourcetypes.TopicDatabases, nil
	case SecurityEngineering:
		return sourcetypes.TopicSecurityEngineering, nil
	case SystemsProgramming:
		return sourcetypes.TopicSystemsProgramming, nil
	case ProductManagement:
		return sourcetypes.TopicProductManagement, nil
	case GrowthEngineering:
		return sourcetypes.TopicGrowthEngineering, nil
	case AiResearch:
		return sourcetypes.TopicAIResearch, nil
	case Robotics:
		return sourcetypes.TopicRobotics, nil
	case OpenSource:
		return sourcetypes.TopicOpenSource, nil
	case CloudInfrastructure:
		return sourcetypes.TopicCloudInfrastructure, nil
	default:
		return "", fmt.Errorf("unknown topic tag: %s", in)
	}
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

// TODO(social-feed-ranking): should we change the sort to best/new or remove it entirely?
func deserializeSortBy(in *ActivitySortBy) (activitytypes.SortBy, error) {
	if in == nil {
		return activitytypes.SortByWeightedScore, nil
	}

	switch *in {
	case CreationDate:
		return activitytypes.SortBySocialScore, nil
	case Similarity:
		return activitytypes.SortByWeightedScore, nil
	}

	return "", fmt.Errorf("unknown sort by: %s", *in)
}

func deserializePeriod(in *ActivityPeriod) activitytypes.Period {
	if in == nil {
		return activitytypes.PeriodAll
	}

	switch *in {
	case "all":
		return activitytypes.PeriodAll
	case "month":
		return activitytypes.PeriodMonth
	case "week":
		return activitytypes.PeriodWeek
	case "day":
		return activitytypes.PeriodDay
	default:
		return activitytypes.PeriodAll
	}
}
