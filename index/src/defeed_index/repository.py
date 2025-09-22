from dataclasses import dataclass
import psycopg2
import psycopg2.extras
from typing import List, Optional, Dict, Any, Union
import logging
import os

from .types import Activity, ActivitySummary, DecoratedActivity, SearchRequest, SearchResult, SortBy, Period

@dataclass
class ActivityRepositoryConfig:
    host: str
    port: int
    database: str
    user: str
    password: str

class ActivityRepository:
    def __init__(self, config: ActivityRepositoryConfig):
        self.connection_string = _build_connection_string(config)
        self.logger = logging.getLogger(__name__)

    def list(self, limit: int) -> List[DecoratedActivity]:
        """
        Read all activities from the database.
        This is the main function needed for seeding BERTopic.
        """
        query = f"""
        SELECT 
            id,
            uid,
            source_uid,
            source_type,
            title,
            body,
            url,
            image_url,
            created_at,
            short_summary,
            full_summary,
            raw_json,
            embedding
        FROM activities
        WHERE embedding IS NOT NULL
        ORDER BY created_at DESC
        LIMIT {limit}
        """
        
        try:
            with self._get_connection() as conn:
                with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                    cur.execute(query)
                    rows = cur.fetchall()
                    
                    activities = []
                    for row in rows:
                        try:
                            activity = _deserialize_decorated_activity(dict(row))
                            activities.append(activity)
                        except Exception as e:
                            self.logger.warning(f"Failed to parse activity {row.get('id', 'unknown')}: {e}")
                            continue
                    
                    self.logger.info(f"Loaded {len(activities)} activities from database")
                    return activities
                    
        except Exception as e:
            self.logger.error(f"Failed to get activities: {e}")
            raise
    
    def _get_connection(self):
        return psycopg2.connect(self.connection_string)
    

def _build_connection_string(config: ActivityRepositoryConfig) -> str:
    conn_str = f"postgresql://{config.user}"
    if config.password:
        conn_str += f":{config.password}"
    conn_str += f"@{config.host}:{config.port}/{config.database}"

    return conn_str

def _deserialize_decorated_activity(self, row: Dict[str, Any]) -> DecoratedActivity:
    """Deserialize a database row into a DecoratedActivity object."""
    activity = _deserialize_activity(row)
    summary = _deserialize_activity_summary(row)
    embedding = None
    if row.get('embedding'):
        embedding = row['embedding']
    similarity = row.get('similarity', 0.0)
    return DecoratedActivity(
        activity=activity,
        summary=summary,
        embedding=embedding,
        similarity=similarity
    )

def _deserialize_activity(self, row: Dict[str, Any]) -> Activity:
    """Deserialize a database row into an Activity object."""
    return Activity(
        uid=row['id'],
        source_uid=row['source_uid'],
        source_type=row['source_type'],
        title=row['title'],
        body=row['body'],
        url=row['url'],
        image_url=row['image_url'],
        created_at=row['created_at'],
        raw_json=row['raw_json']
    )

def _deserialize_activity_summary(self, row: Dict[str, Any]) -> Optional[ActivitySummary]:
    """Deserialize a database row into an ActivitySummary object if data is available."""
    if row.get('short_summary') and row.get('full_summary'):
        return ActivitySummary(
            short_summary=row['short_summary'],
            full_summary=row['full_summary']
        )
    return None