package types

// TopicTag represents a niche interest category used to personalize sources.
// Keep values lowercase snake_case to match OpenAPI and external clients.
type TopicTag string

const (
	TopicLLMs                TopicTag = "llms"
	TopicStartups            TopicTag = "startups"
	TopicAgenticSystems      TopicTag = "agentic_systems"
	TopicDevTools            TopicTag = "devtools"
	TopicWebPerformance      TopicTag = "web_performance"
	TopicDistributedSystems  TopicTag = "distributed_systems"
	TopicDatabases           TopicTag = "databases"
	TopicSecurityEngineering TopicTag = "security_engineering"
	TopicSystemsProgramming  TopicTag = "systems_programming"
	TopicProductManagement   TopicTag = "product_management"
	TopicGrowthEngineering   TopicTag = "growth_engineering"
	TopicAIResearch          TopicTag = "ai_research"
	TopicRobotics            TopicTag = "robotics"
	TopicOpenSource          TopicTag = "open_source"
	TopicCloudInfrastructure TopicTag = "cloud_infrastructure"
)

// WordToTopic maps a free-form string to a TopicTag when possible.
// It supports a small set of synonyms to avoid duplicating logic in providers.
func WordToTopic(s string) (TopicTag, bool) {
	switch normalize(s) {
	case "llm", "llms", "large_language_models", "chatgpt", "gpt", "local_llm", "local_llms":
		return TopicLLMs, true
	case "startup", "startups", "founders", "entrepreneurship":
		return TopicStartups, true
	case "agent", "agentic", "agentic_systems", "agents":
		return TopicAgenticSystems, true
	case "devtools", "developer_tools", "engineering", "software_engineering":
		return TopicDevTools, true
	case "webperf", "web_performance", "performance":
		return TopicWebPerformance, true
	case "distributed", "distributed_systems", "concurrency", "scalability":
		return TopicDistributedSystems, true
	case "database", "databases", "db", "postgres", "mysql", "storage":
		return TopicDatabases, true
	case "security", "security_engineering", "infosec", "netsec", "appsec", "cybersecurity":
		return TopicSecurityEngineering, true
	case "systems", "systems_programming", "kernel", "linux", "rust", "c", "c++":
		return TopicSystemsProgramming, true
	case "product", "product_management", "pm":
		return TopicProductManagement, true
	case "growth", "growth_engineering", "experimentation", "ab_testing":
		return TopicGrowthEngineering, true
	case "ai", "ml", "machine_learning", "ai_research", "research":
		return TopicAIResearch, true
	case "robotics", "robot", "autonomy":
		return TopicRobotics, true
	case "oss", "open_source", "opensource":
		return TopicOpenSource, true
	case "cloud", "cloud_infrastructure", "kubernetes", "aws", "gcp", "azure":
		return TopicCloudInfrastructure, true
	}
	return "", false
}

func normalize(s string) string {
	// lightweight normalization: lower and replace spaces/hyphens
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		if c == ' ' || c == '-' || c == '/' || c == '\\' {
			c = '_'
		}
		out = append(out, c)
	}
	return string(out)
}
