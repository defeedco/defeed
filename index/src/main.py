#!/usr/bin/env python3
"""
Usage example for the defeed_index module.

This demonstrates how to use the ActivityRepository and Registry classes
to load activities from PostgreSQL and analyze them with BERTopic.
"""

import logging
import os
from dotenv import load_dotenv
from defeed_index import Registry, ActivityRepository, ActivityRepositoryConfig

# Set up logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

def main():
    """Main example function."""
    print("🚀 Defeed Index Usage Example")
    print("=" * 50)

    load_dotenv()
    
    config = ActivityRepositoryConfig(
        host=os.getenv('DB_HOST'),
        port=int(os.getenv('DB_PORT')),
        database=os.getenv('DB_NAME'),
        user=os.getenv('DB_USER'),
        password=os.getenv('DB_PASSWORD')
    )

    try:
        # Initialize the repository
        print("📊 Initializing ActivityRepository...")
        repository = ActivityRepository(config)
        
        # Initialize the registry
        print("🧠 Initializing Registry...")
        registry = Registry(repository)
        
        # Seed the BERTopic index with existing activities
        print("🌱 Seeding BERTopic index...")
        registry.seed()
        
        # Get topic information
        print("\n📋 Topic Information:")
        topic_info = registry.get_topic_info()
        if topic_info is not None:
            print(topic_info.head(10))
        
        # Get all topics summary
        print("\n🎯 Topic Summaries:")
        topics_summary = registry.get_all_topics_summary()
        for i, topic in enumerate(topics_summary[:3]):  # Show first 3 topics
            print(f"\nTopic {topic['topic_id']}:")
            print(f"  Keywords: {[kw['word'] for kw in topic['keywords'][:5]]}")
            print(f"  Activity count: {topic['activity_count']}")
            print(f"  Sample activities: {len(topic['activities'])}")
        
        # Example: Find topics similar to a query
        print("\n🔍 Finding topics similar to 'AI':")
        similar_topics, scores = registry.find_similar_topics("AI", top_n=3)
        for topic_id, score in zip(similar_topics, scores):
            keywords = registry.get_topic_keywords(topic_id, num_words=5)
            if keywords:
                words = [word for word, _ in keywords]
                print(f"  Topic {topic_id} (score: {score:.3f}): {', '.join(words)}")
        
        print("\n✅ Example completed successfully!")
        
    except Exception as e:
        print(f"❌ Error: {e}")
        raise


if __name__ == "__main__":
    main()
