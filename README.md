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

### List Documents

```bash
curl http://localhost:8080/api/v1/documents
```

Response:
```json
{
  "success": true,
  "data": {
    "documents": [
      {
        "id": "doc123",
        "source": "document.md",
        "title": "Document Title",
        "format": "markdown",
        "created_at": "2026-01-25 10:30:00",
        "updated_at": "2026-01-25 10:30:00"
      }
    ]
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

### Conversations (Multi-turn Chat)

Create a conversation:
```bash
curl -X POST http://localhost:8080/api/v1/conversations \
  -H "Content-Type: application/json" \
  -d '{"title": "My Chat"}'
```

List conversations:
```bash
curl http://localhost:8080/api/v1/conversations
```

Send a message:
```bash
curl -X POST http://localhost:8080/api/v1/conversations/{conversation_id}/messages \
  -H "Content-Type: application/json" \
  -d '{"content": "What is RAG?"}'
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
│       └── static/        # Web UI (index.html)
├── pkg/config/           # Configuration management
├── configs/              # Configuration files
├── testdata/             # Test documents
├── test.sh               # Comprehensive test script
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

## Testing

### Quick Test Script

We provide a comprehensive test script that validates all major features:

```bash
# Make sure server is running first
make run-ollama  # or make run-config

# In another terminal, run the test script
./test.sh
```

The test script will:
1. ✅ Check server health
2. ✅ Verify initial document list
3. ✅ Upload a test document
4. ✅ Monitor document processing job
5. ✅ Verify document appears in list
6. ✅ Create a conversation
7. ✅ Send a test message (if document processing completed)
8. ✅ List all conversations

### Manual Testing Steps

#### 1. Start the Server

**Option A: Using Ollama (Local, No API Keys Required)**
```bash
# Install Ollama: https://ollama.ai
# Pull required models:
ollama pull nomic-embed-text
ollama pull llama3.2

# Start server
make run-ollama
```

**Option B: Using OpenAI/Anthropic**
```bash
# Set API keys
export OPENAI_API_KEY="your-key"
export ANTHROPIC_API_KEY="your-key"  # optional

# Start server
make run-config
```

#### 2. Test Document Upload

```bash
# Upload a document
curl -X POST http://localhost:8080/api/v1/documents \
  -F "file=@testdata/sample-doc.md"

# Check job status (replace JOB_ID with actual ID from response)
curl http://localhost:8080/api/v1/jobs/{JOB_ID}

# List documents (should show uploaded document after processing)
curl http://localhost:8080/api/v1/documents | jq '.data.documents[]'
```

#### 3. Test Web UI

Open browser and navigate to:
```
http://localhost:8080
```

Features to test:
- ✅ Upload document via drag-and-drop or file picker
- ✅ View uploaded documents in right sidebar
- ✅ Create new conversation
- ✅ Send messages and receive responses with citations
- ✅ View conversation history

#### 4. Test API Endpoints

```bash
# Health check
curl http://localhost:8080/api/v1/health | jq '.'

# Create conversation
CONV_ID=$(curl -s -X POST http://localhost:8080/api/v1/conversations \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Chat"}' | jq -r '.data.id')

# Send message
curl -X POST http://localhost:8080/api/v1/conversations/$CONV_ID/messages \
  -H "Content-Type: application/json" \
  -d '{"content":"What is RAG?"}' | jq '.'

# List conversations
curl http://localhost:8080/api/v1/conversations | jq '.data.conversations[]'
```

### Expected Behavior

- **Document Upload**: Returns immediately with `job_id`, processing happens asynchronously
- **Document List**: Shows all successfully processed documents with metadata
- **Web UI**: Right sidebar displays uploaded documents, updates automatically after upload
- **Conversations**: Multi-turn chat with context retention
- **Citations**: Responses include `[citation:N]` references to source documents

### Troubleshooting

**Documents not appearing in list:**
- Check job status: `curl http://localhost:8080/api/v1/jobs/{JOB_ID}`
- Verify embedder/LLM is configured correctly
- Check server logs for errors
- Job may have failed during processing - check the error field in job status

**Document processing fails at 50% progress:**
This usually indicates an embedding error. Common causes:
- **No embedder configured**: Check that `embedding.provider` is set correctly in config
- **Ollama not running**: Start Ollama service: `ollama serve`
- **Model not downloaded**: Pull required model: `ollama pull nomic-embed-text`
- **Connection error**: Check `embedding.ollama.base_url` matches your Ollama instance
- **API key missing**: If using OpenAI, ensure `OPENAI_API_KEY` environment variable is set

**Ollama connection errors:**
- Ensure Ollama is running: `ollama serve`
- Verify models are pulled: `ollama list`
- Check `configs/config-ollama.yaml` base_url matches your Ollama instance
- Test Ollama directly: `curl http://localhost:11434/api/embeddings`

**PDF parsing errors:**
- Install poppler-utils: `sudo apt-get install poppler-utils` (Linux) or `brew install poppler` (macOS)

**Getting detailed error information:**
```bash
# Check job status with error details
curl http://localhost:8080/api/v1/jobs/{JOB_ID} | jq '.data.error'

# Check server logs for detailed error messages
# Look for lines containing "job failed" or "error"
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Build
make build

# Run comprehensive integration test
./test.sh
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

