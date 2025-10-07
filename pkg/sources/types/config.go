package types

type ProviderConfig struct {
	GithubAPIKey string `env:"GITHUB_API_KEY,default="`

	RedditClientID     string `env:"REDDIT_CLIENT_ID,default="`
	RedditClientSecret string `env:"REDDIT_CLIENT_SECRET,default="`

	MastodonClientID     string `env:"MASTODON_CLIENT_ID,default="`
	MastodonClientSecret string `env:"MASTODON_CLIENT_SECRET,default="`

	ProductHuntAPIToken string `env:"PRODUCTHUNT_API_TOKEN,default="`
}
