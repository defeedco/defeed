import logging
from typing import List, Optional, Dict, Any
from bertopic import BERTopic
from bertopic.representation import KeyBERTInspired
from hdbscan import HDBSCAN
from sentence_transformers import SentenceTransformer
from sklearn.feature_extraction.text import CountVectorizer
import pandas as pd
from datetime import datetime, timedelta

from umap import UMAP

from .repository import ActivityRepository
from .types import DecoratedActivity, SearchRequest, SearchResult


class Registry:
    """
    Registry provides an abstraction layer on top of the ActivityRepository
    and implements activity analysis using BERTopic for topic modeling.
    """
    
    def __init__(self, repository: ActivityRepository):
        self.repository = repository
        self.logger = logging.getLogger(__name__)
        self.activities: List[DecoratedActivity] = []
        self.documents: List[str] = []
        self.topics: List[int] = []

        self.vectorizer_model = CountVectorizer(
            ngram_range=(1, 2),
            stop_words="english",
            min_df=2,  # Require at least 2 documents for a term
            max_df=0.95  # Ignore terms that appear in more than 95% of documents
        )

        self.embedding_model = SentenceTransformer("thenlper/gte-small")
        self.umap_model = UMAP(
            n_components=5,
            min_dist=0.0,
            metric='cosine',
            random_state=42 # Set for reproducibility
        )

        self.hdbscan_model = HDBSCAN(
            min_cluster_size=5,
            metric='euclidean',
            cluster_selection_method='eom'
        )

        self.representation_model = KeyBERTInspired()

        self.topic_model = BERTopic(
            vectorizer_model=self.vectorizer_model,
            embedding_model=self.embedding_model,
            representation_model=self.representation_model,
            umap_model=self.umap_model,
            hdbscan_model=self.hdbscan_model,
            verbose=True,
            min_topic_size=3,  # Minimum 3 activities per topic
            nr_topics="auto"  # Let BERTopic determine the optimal number of topics
        )
        
    def seed(self) -> None:
        """
        Load existing activities from the repository and seed the BERTopic index.
        """

        self.activities = self.repository.list(
            from_date=datetime.now() - timedelta(days=10),
        )

        if not self.activities:
            self.logger.warning("No activities found in database")
            return
        
        # Prepare documents for BERTopic
        self.documents = []
        for activity in self.activities:
            # full_summary is formatted in Markdown with predefined sections.
            # This will skew the topic modeling results, so use short_summary instead.
            self.documents.append(activity.summary.short_summary)
        
        self.logger.info(f"Prepared {len(self.documents)} documents for topic modeling")

        embeddings = self.embedding_model.encode(self.documents, show_progress_bar=True)

        self.topics, probabilities = self.topic_model.fit_transform(self.documents, embeddings)

        self.logger.info(f"BERTopic modeling completed:")
        self.logger.info(f"  - Found {len(set(self.topics))} topics")
        self.logger.info(f"  - Processed {len(self.documents)} documents")