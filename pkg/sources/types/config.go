package types

type ProviderConfig struct {
	GithubAPIToken string `env:"GITHUB_API_TOKEN,default="`

	RedditClientID     string `env:"REDDIT_CLIENT_ID,default="`
	RedditClientSecret string `env:"REDDIT_CLIENT_SECRET,default="`

	MastodonClientID     string `env:"MASTODON_CLIENT_ID,default="`
	MastodonClientSecret string `env:"MASTODON_CLIENT_SECRET,default="`

	ChangedetectionToken string `env:"CHANGEDETECTION_TOKEN,default="`
}
