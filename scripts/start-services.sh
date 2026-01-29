#!/bin/bash

# Start all services using Docker Compose
echo "Starting RAG infrastructure services..."

docker-compose up -d

echo "Waiting for services to be ready..."
sleep 10

# Check service health
echo "Checking service health..."

# MySQL
if docker exec rag-mysql mysqladmin ping -h localhost --silent; then
    echo "✅ MySQL is ready"
else
    echo "❌ MySQL is not ready"
fi

# Redis
if docker exec rag-redis redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis is ready"
else
    echo "❌ Redis is not ready"
fi

# Qdrant
if curl -s http://localhost:6333/health > /dev/null 2>&1; then
    echo "✅ Qdrant is ready"
    echo "   Qdrant Dashboard: http://localhost:6333/dashboard"
else
    echo "❌ Qdrant is not ready"
fi

echo ""
echo "All services started. Use 'docker-compose logs -f' to view logs."
echo "Use 'docker-compose down' to stop all services."

