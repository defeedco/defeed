# Defeed Activity Index

A modern Python module for natural language processing of activities, providing topic analysis and semantic search capabilities for the defeed system.

## Features

- **Topic Modeling**: Discover topics in activity feeds using [BERTopic](https://github.com/MaartenGr/BERTopic)
- **Semantic Search**: Find similar activities and topics using transformer-based embeddings
- **Activity Analysis**: Process activities from various sources (GitHub, Reddit, RSS, etc.)
- **Modern Python**: Built with uv package manager and latest Python practices

## Quick Start

### Prerequisites

- Python 3.11+
- [uv](https://github.com/astral-sh/uv) package manager

### Installation

```bash
# Clone and navigate to the index module
cd /path/to/defeed-api/index

# Install dependencies
uv sync
```

### Running the Example

```bash
# Run the BERTopic demonstration
uv run python -m defeed_index.example
```

This will demonstrate:
- Basic topic modeling on sample activity data
- Topic-based search functionality
- Document analysis and clustering
- Automatic topic labeling

## Development

### Project Structure

```
index/
├── src/defeed_index/
│   ├── __init__.py
│   └── example.py          # BERTopic demonstration
├── pyproject.toml          # Project configuration
├── README.md
└── .venv/                  # Virtual environment (created by uv)
```

### Adding Dependencies

```bash
# Add new dependencies
uv add package-name

# Add development dependencies
uv add --dev package-name
```

### Running Scripts

```bash
# Run any script in the project environment
uv run python script.py

# Or activate the environment and run normally
source .venv/bin/activate
python script.py
```

## Activity Data Model

The module is designed to work with activities that have the following structure:

- **UID**: Unique identifier
- **Title**: Activity title/headline
- **Body**: Full activity content
- **URL**: Source URL
- **CreatedAt**: Timestamp
- **SourceType**: Type of source (github, reddit, rss, etc.)
- **Embedding**: Vector representation (optional)
- **Summary**: Short and full summaries (optional)

## Next Steps

This example demonstrates the basic setup. The module can be extended with:

1. **Activity Data Models**: Python classes matching Go activity types
2. **Database Integration**: Connect to PostgreSQL for activity data
3. **Advanced NLP**: Sentiment analysis, entity extraction, summarization
4. **API Integration**: REST endpoints for topic analysis
5. **Caching**: Redis integration for performance
6. **Monitoring**: Logging and metrics for production use

## Dependencies

- **bertopic**: Topic modeling with transformers
- **scikit-learn**: Machine learning utilities
- **pandas**: Data manipulation
- **numpy**: Numerical computing
- **sentence-transformers**: Semantic embeddings (via bertopic)
- **torch**: Deep learning framework (via bertopic)

## License

MIT License - see the main project license.
