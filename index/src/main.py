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
    load_dotenv()
    
    config = ActivityRepositoryConfig(
        host=os.getenv('DB_HOST'),
        port=int(os.getenv('DB_PORT')),
        database=os.getenv('DB_NAME'),
        user=os.getenv('DB_USER'),
        password=os.getenv('DB_PASSWORD')
    )

    print("📊 Initializing ActivityRepository...")
    repository = ActivityRepository(config)

    print("🧠 Initializing Registry...")
    registry = Registry(repository)

    print("🌱 Seeding BERTopic index...")
    registry.seed()

    print("\n📋 Topic Information:")
    topic_info = registry.topic_model.get_topic_info()
    if topic_info is not None:
        print(topic_info.head(50))

if __name__ == "__main__":
    main()
