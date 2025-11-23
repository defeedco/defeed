from dataclasses import dataclass
from datetime import datetime
from enum import Enum
from typing import List, Optional


class SortBy(Enum):
    SIMILARITY = "similarity"
    DATE = "date"


class Period(Enum):
    ALL = "all"
    MONTH = "month"
    WEEK = "week"
    DAY = "day"


@dataclass
class ActivitySummary:
    short_summary: str
    full_summary: str


@dataclass
class Activity:
    uid: str
    source_uid: str
    source_type: str
    title: str
    body: str
    url: str
    image_url: str
    created_at: datetime
    raw_json: str


@dataclass
class DecoratedActivity:
    activity: Activity
    summary: Optional[ActivitySummary]
    embedding: Optional[List[float]]
    similarity: float = 0.0


@dataclass
class SearchRequest:
    source_uids: Optional[List[str]] = None
    activity_uids: Optional[List[str]] = None
    min_similarity: float = 0.0
    limit: int = 50
    cursor: Optional[str] = None
    sort_by: SortBy = SortBy.DATE
    period: Period = Period.ALL
    query_embedding: Optional[List[float]] = None


@dataclass
class SearchResult:
    activities: List[DecoratedActivity]
    next_cursor: Optional[str] = None
    has_more: bool = False
