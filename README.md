# Go RAG Knowledge Base System

A production-oriented RAG (Retrieval-Augmented Generation) knowledge base system built with Go, featuring clear module boundaries, multiple LLM/embedding provider support, and enterprise-grade architecture.

## Features

- **Document Ingestion**: Support for Markdown, PDF, and plain text documents
- **Structure-Aware Chunking**: Respects document structure (headings, paragraphs) with configurable overlap
- **Vector Storage**: SQLite-based storage with sqlite-vec extension support
- **Multi-Provider Support**: 
  - LLM: OpenAI, Anthropic Claude, Ollama
  - Embeddings: OpenAI, Ollama
- **Citation System**: Automatic `[citation:N]` formatting with source tracking
- **Async Processing**: Background job queue for document processing
- **Incremental Indexing**: Hash-based change detection, atomic document updates

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP API Layer                          │
│  /api/v1/documents  /api/v1/query  /api/v1/jobs  /api/v1/health │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                     Core Modules                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │ingestion │ │ chunking │ │  index   │ │retrieval │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
│  ┌──────────┐ ┌──────────┐                                  │
│  │  prompt  │ │generation│                                  │
│  └──────────┘ └──────────┘                                  │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│              Storage & External Services                     │
│  ┌──────────────┐  ┌────────┐  ┌──────────┐  ┌──────┐      │
│  │SQLite+sqlite-vec│  │ OpenAI │  │Anthropic │  │Ollama│      │
│  └──────────────┘  └────────┘  └──────────┘  └──────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+
- CGO enabled (for SQLite)
- Optional: `pdftotext` from poppler-utils (for PDF support)
- Optional: Ollama (for local models)

### Installation

```bash
# Clone repository
git clone <repo-url>
cd RAG

# Download dependencies
go mod download

# Build
CGO_ENABLED=1 go build -o rag-server ./cmd/server
```

### Configuration

Create a `configs/config.yaml` file:

```yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 60s

database:
  path: "./data/rag.db"

embedding:
  provider: "openai"
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"

llm:
  default_provider: "openai"
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4"

chunking:
  max_size: 1000
  overlap: 100

retrieval:
  default_top_k: 5
```

### Running

```bash
# Set API keys
export OPENAI_API_KEY="your-key"
export ANTHROPIC_API_KEY="your-key"  # optional

# Run server
./rag-server -config configs/config.yaml

# Or use make
make run-config
```

## API Endpoints

### Document Upload (Async)

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -F "file=@document.md" \
  -F 'metadata={"project":"docs"}'
```

Response:
```json
{
  "success": true,
  "data": {
    "job_id": "abc123",
    "status": "pending",
    "message": "Document processing started"
  }
}
```

### Job Status

```bash
curl http://localhost:8080/api/v1/jobs/abc123
```

### RAG Query

```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How does the system work?",
    "options": {
      "top_k": 5,
      "provider": "openai"
    }
  }'
```

Response:
```json
{
  "success": true,
  "data": {
    "answer": "The system works by... [citation:1][citation:2]",
    "citations": [
      {
        "id": 1,
        "source": "docs/overview.md",
        "section": "Architecture",
        "preview": "The RAG system consists of..."
      }
    ],
    "tokens_used": 1250
  }
}
```

### Health Check

```bash
curl http://localhost:8080/api/v1/health
```

## Project Structure

```
/home/krc/RAG/
├── cmd/server/           # Application entry point
├── internal/
│   ├── ingestion/        # Document parsing (Markdown, PDF, Text)
│   ├── chunking/         # Structure-aware text chunking
│   ├── index/            # Vector storage interfaces
│   │   └── sqlite/       # SQLite implementation
│   ├── retrieval/        # Search and ranking
│   ├── prompt/           # Prompt construction and citations
│   ├── generation/       # LLM clients
│   │   ├── openai/
│   │   ├── anthropic/
│   │   └── ollama/
│   ├── embedding/        # Embedding providers
│   ├── job/              # Async job processing
│   └── api/              # HTTP handlers and routing
├── pkg/config/           # Configuration management
├── configs/              # Configuration files
└── Makefile
```

## Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `server.port` | HTTP server port | 8080 |
| `database.path` | SQLite database path | ./data/rag.db |
| `embedding.provider` | Default embedding provider | openai |
| `llm.default_provider` | Default LLM provider | openai |
| `chunking.max_size` | Max chunk size (chars) | 1000 |
| `chunking.overlap` | Chunk overlap (chars) | 100 |
| `retrieval.default_top_k` | Default results count | 5 |
| `prompt.max_context_tokens` | Max context tokens | 3000 |

## Development

```bash
# Run tests
make test

# Run with coverage
make test-coverage

# Format code
make fmt

# Build
make build
```

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Vector Storage | SQLite + sqlite-vec | Zero external dependencies, easy deployment |
| Chunking Strategy | Structure-aware + overlap | Preserves semantics, avoids information loss |
| Citation Format | `[citation:N]` | Industry standard, easy to parse |
| Async Processing | Go channels | Simple, no external queue required |
| Multi-provider | Interface abstraction | Easily switch/compare providers |

## License

None

