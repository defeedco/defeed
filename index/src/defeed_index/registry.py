import logging
from typing import List, Optional, Dict, Any
from bertopic import BERTopic
from sklearn.feature_extraction.text import CountVectorizer
import pandas as pd

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

        vectorizer_model = CountVectorizer(
            ngram_range=(1, 2),
            stop_words="english",
            min_df=2,  # Require at least 2 documents for a term
            max_df=0.95  # Ignore terms that appear in more than 95% of documents
        )

        self.topic_model = BERTopic(
            vectorizer_model=vectorizer_model,
            verbose=True,
            min_topic_size=3,  # Minimum 3 activities per topic
            nr_topics="auto"  # Let BERTopic determine the optimal number of topics
        )
        
    def seed(self) -> None:
        """
        Load existing activities from the repository and seed the BERTopic index.
        """

        self.activities = self.repository.list(limit=100)

        if not self.activities:
            self.logger.warning("No activities found in database")
            return
        
        # Prepare documents for BERTopic
        self.documents = []
        for activity in self.activities:
            if activity.summary and activity.summary.full_summary:
                doc_text = activity.summary.full_summary
            else:
                doc_text = f"{activity.activity.title} {activity.summary.short_summary}"
            
            self.documents.append(doc_text)
        
        self.logger.info(f"Prepared {len(self.documents)} documents for topic modeling")
        
        self.topics, probabilities = self.topic_model.fit_transform(self.documents)

        self.logger.info(f"BERTopic modeling completed:")
        self.logger.info(f"  - Found {len(set(self.topics))} topics")
        self.logger.info(f"  - Processed {len(self.documents)} documents")
    
    def get_topic_info(self) -> Optional[pd.DataFrame]:
        return self.topic_model.get_topic_info()
    
    def get_topic_activities(self, topic_id: int) -> List[DecoratedActivity]:
        topic_activities = []
        for i, assigned_topic in enumerate(self.topics):
            if assigned_topic == topic_id and i < len(self.activities):
                topic_activities.append(self.activities[i])
        
        return topic_activities
    
    def get_topic_keywords(self, topic_id: int, num_words: int = 10) -> List[tuple]:
        return self.topic_model.get_topic(topic_id)[:num_words]
    
    def find_similar_topics(self, query: str, top_n: int = 5) -> tuple[list[int], list[float]]:
        return self.topic_model.find_topics(query, top_n=top_n)

    def get_topic_summary(self, topic_id: int) -> Dict[str, Any]:
        keywords = self.get_topic_keywords(topic_id)
        activities = self.get_topic_activities(topic_id)
        
        return {
            "topic_id": topic_id,
            "keywords": [{"word": word, "score": score} for word, score in keywords],
            "activity_count": len(activities),
            "activities": [
                {
                    "uid": activity.activity.uid,
                    "title": activity.activity.title,
                    "created_at": activity.activity.created_at.isoformat(),
                    "source_type": activity.activity.source_type
                }
                for activity in activities[:10]  # Limit to first 10 for summary
            ]
        }
    
    def get_all_topics_summary(self) -> List[Dict[str, Any]]:
        """Get summaries for all discovered topics."""
        if self.topic_model is None:
            return []
        
        topic_info = self.get_topic_info()
        if topic_info is None:
            return []
        
        summaries = []
        for _, row in topic_info.iterrows():
            topic_id = row['Topic']
            if topic_id != -1:  # Skip outlier topic
                summary = self.get_topic_summary(topic_id)
                summaries.append(summary)
        
        return summaries
