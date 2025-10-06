#!/usr/bin/env python3
"""
Usage example for the defeed_index module.

This demonstrates how to use the ActivityRepository and Registry classes
to load activities from PostgreSQL and analyze them with BERTopic.
"""

import logging
import os
from dotenv import load_dotenv
from matplotlib import pyplot as plt

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
    print(topic_info)

    fig = registry.topic_model.visualize_document_datamap(
        registry.documents,
        topics=registry.topics,
        width=1200,
        height=800
    )
    plt.savefig("./out/datamapplot_1.png", dpi=300, bbox_inches='tight')
    plt.show()

if __name__ == "__main__":
    main()
