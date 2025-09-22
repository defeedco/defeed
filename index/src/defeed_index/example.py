#!/usr/bin/env python3
"""
Example script demonstrating BERTopic usage for activity topic modeling.

This script shows how to use BERTopic for analyzing activities from the defeed system,
including topic modeling and search capabilities.
"""

from bertopic import BERTopic
from sklearn.feature_extraction.text import CountVectorizer
import pandas as pd
from typing import List, Dict, Any


def create_sample_activities() -> List[str]:
    """Create sample activity data similar to defeed activities."""
    return [
        "New release of Python 3.12 with improved performance and security features",
        "GitHub introduces new AI-powered code review tools for developers",
        "React 18.3 update brings better concurrent rendering capabilities",
        "Docker announces new container security scanning features",
        "Kubernetes 1.28 released with enhanced networking and storage",
        "OpenAI launches GPT-4 Turbo with improved reasoning capabilities",
        "AWS introduces new serverless computing options for edge deployment",
        "TypeScript 5.2 adds new syntax features and better type inference",
        "Rust programming language gains popularity in systems programming",
        "Vue.js 3.4 brings composition API improvements and performance boosts",
        "Machine learning breakthrough in natural language processing",
        "New cybersecurity threats targeting cloud infrastructure discovered",
        "Blockchain technology adoption increases in financial services",
        "Quantum computing research shows promising advances in error correction",
        "Edge computing solutions become more viable for IoT applications",
        "WebAssembly gains traction for high-performance web applications",
        "GraphQL adoption grows among API developers",
        "Microservices architecture patterns evolve with new best practices",
        "DevOps tools integration improves CI/CD pipeline efficiency",
        "Cloud-native development practices become industry standard"
    ]


def demonstrate_bertopic_basic():
    """Demonstrate basic BERTopic functionality."""
    print("🚀 Demonstrating BERTopic basic functionality...")
    
    # Create sample data
    docs = create_sample_activities()
    
    # Initialize BERTopic with custom settings
    vectorizer_model = CountVectorizer(
        ngram_range=(1, 2), 
        stop_words="english", 
        min_df=1
    )
    
    topic_model = BERTopic(
        vectorizer_model=vectorizer_model,
        verbose=True,
        min_topic_size=2
    )
    
    # Fit the model and get topics
    topics, probs = topic_model.fit_transform(docs)
    
    print(f"\n📊 Found {len(set(topics))} topics")
    print(f"📝 Processed {len(docs)} documents")
    
    # Display topic information
    topic_info = topic_model.get_topic_info()
    print(f"\n📋 Topic Overview:")
    print(topic_info.head(10))
    
    return topic_model, topics, docs


def demonstrate_topic_search(topic_model: BERTopic, docs: List[str]):
    """Demonstrate topic-based search functionality."""
    print("\n🔍 Demonstrating topic search...")
    
    # Find topics related to specific terms
    search_terms = ["AI", "security", "programming"]
    
    for term in search_terms:
        try:
            similar_topics = topic_model.find_topics(term)
            print(f"\n🎯 Topics similar to '{term}':")
            for topic_id in similar_topics[:3]:  # Top 3 similar topics
                topic_words = topic_model.get_topic(topic_id)
                if topic_words:
                    words = [word for word, _ in topic_words[:5]]
                    print(f"  Topic {topic_id}: {', '.join(words)}")
        except Exception as e:
            print(f"  Could not find topics for '{term}': {e}")


def demonstrate_document_analysis(topic_model: BERTopic, topics: List[int], docs: List[str]):
    """Demonstrate document-level analysis."""
    print("\n📄 Demonstrating document analysis...")
    
    # Get document information
    doc_info = topic_model.get_document_info(docs)
    
    # Show documents by topic
    for topic_id in set(topics):
        if topic_id != -1:  # Skip outlier topic
            topic_docs = doc_info[doc_info.Topic == topic_id]
            if not topic_docs.empty:
                print(f"\n📂 Topic {topic_id} documents:")
                for _, row in topic_docs.head(2).iterrows():
                    print(f"  - {row['Document'][:80]}...")


def demonstrate_topic_representation(topic_model: BERTopic):
    """Demonstrate topic representation and labeling."""
    print("\n🏷️  Demonstrating topic representation...")
    
    # Get all topics
    topics_dict = topic_model.get_topics()
    
    print("🎨 Top topics and their key terms:")
    for topic_id, words in list(topics_dict.items())[:5]:
        if topic_id != -1:  # Skip outlier topic
            top_words = [word for word, _ in words[:5]]
            print(f"  Topic {topic_id}: {', '.join(top_words)}")
    
    # Generate topic labels
    try:
        topic_labels = topic_model.generate_topic_labels(nr_words=3, topic_prefix=False)
        print(f"\n🏷️  Auto-generated topic labels:")
        if isinstance(topic_labels, dict):
            for topic_id, label in topic_labels.items():
                if topic_id != -1:
                    print(f"  Topic {topic_id}: {label}")
        elif isinstance(topic_labels, list):
            for i, label in enumerate(topic_labels):
                if i != 0:  # Skip outlier topic
                    print(f"  Topic {i-1}: {label}")
    except Exception as e:
        print(f"  Could not generate labels: {e}")


def main():
    """Main demonstration function."""
    print("🎯 Defeed Activity Index - BERTopic Demo")
    print("=" * 50)
    
    try:
        # Basic BERTopic demonstration
        topic_model, topics, docs = demonstrate_bertopic_basic()
        
        # Topic search demonstration
        demonstrate_topic_search(topic_model, docs)
        
        # Document analysis demonstration
        demonstrate_document_analysis(topic_model, topics, docs)
        
        # Topic representation demonstration
        demonstrate_topic_representation(topic_model)
        
        print("\n✅ BERTopic setup and demonstration completed successfully!")
        print("🎉 The Python NLP module is ready for development!")
        
    except Exception as e:
        print(f"❌ Error during demonstration: {e}")
        raise


if __name__ == "__main__":
    main()
