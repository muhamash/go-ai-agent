# Redis Integration Setup

## 1. Install Redis Dependencies

First, you need to install the Redis Go client:

```bash
go mod init ai-gateway
go get github.com/go-redis/redis/v8
go get github.com/gin-gonic/gin
go get github.com/gin-contrib/cors
go get github.com/joho/godotenv
```

## 2. Environment Configuration (.env file)

Create a `.env` file in your project root:

```env
# OpenAI Configuration
OPENAI_API_KEY=sk-your-openai-api-key-here

# Redis Configuration
REDIS_URL=redis://localhost:6379
# OR use individual Redis settings:
# REDIS_PASSWORD=your-redis-password
# REDIS_DB=0

# Session Configuration
SESSION_TTL_HOURS=24

# Server Configuration
PORT=8080
```

## 3. Docker Compose for Local Development

Create `docker-compose.yml`:

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    
  ai-gateway:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - redis
    environment:
      - REDIS_URL=redis://redis:6379
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    env_file:
      - .env

volumes:
  redis_data:
```

## 4. Dockerfile

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o ai-gateway main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/ai-gateway .
COPY --from=builder /app/.env .

CMD ["./ai-gateway"]
```

## 5. Installation Options

### Option A: Local Redis Installation

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install redis-server
sudo systemctl start redis-server
sudo systemctl enable redis-server
```

**macOS:**
```bash
brew install redis
brew services start redis
```

**Windows:**
- Download Redis from GitHub releases
- Or use WSL2 with Ubuntu installation

### Option B: Docker Redis
```bash
# Start Redis with Docker
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Or use docker-compose
docker-compose up -d redis
```

### Option C: Cloud Redis Services
- **AWS ElastiCache**
- **Google Cloud Memorystore**
- **Azure Cache for Redis**
- **Redis Cloud**
- **Railway/Heroku Redis**

## 6. Running the Application

### Local Development:
```bash
# Start Redis (if not running)
redis-server

# Run the Go application
go run main.go
```

### With Docker:
```bash
# Build and run everything
docker-compose up --build
```

## 7. Redis Features Implemented

### Session Management:
- **Persistent Sessions**: Sessions survive server restarts
- **TTL Management**: Automatic session expiration
- **Scalability**: Multiple server instances can share sessions

### Key Features:
- ✅ **Session Storage**: Complete conversation history
- ✅ **Automatic Expiration**: Configurable TTL (default 24 hours)
- ✅ **Memory Efficiency**: Only active sessions in memory
- ✅ **Scalability**: Horizontal scaling support
- ✅ **Persistence**: Sessions survive server restarts
- ✅ **Statistics**: Redis memory usage and session stats

## 8. API Usage Examples

### Basic Chat:
```bash
curl -X POST http://localhost:8080/ai-agent \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Hello, AI!"}'
```

### With Session Management:
```bash
# First message
curl -X POST http://localhost:8080/ai-agent \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "My name is John and I love programming",
    "session_id": "user_123"
  }'

# Continue conversation (AI remembers context)
curl -X POST http://localhost:8080/ai-agent \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What did I just tell you about myself?",
    "session_id": "user_123"
  }'
```

### Session Management:
```bash
# Get session info
curl http://localhost:8080/session/user_123

# List all sessions
curl http://localhost:8080/sessions

# Delete session
curl -X DELETE http://localhost:8080/session/user_123

# Get system stats
curl http://localhost:8080/stats
```

## 9. Redis Configuration Options

### Environment Variables:
- `REDIS_URL`: Full Redis connection URL
- `REDIS_PASSWORD`: Redis password (if required)
- `REDIS_DB`: Redis database number (default: 0)
- `SESSION_TTL_HOURS`: Session expiration time in hours

### Production Considerations:
- Use Redis Cluster for high availability
- Configure Redis persistence (RDB + AOF)
- Set up Redis monitoring
- Use connection pooling
- Implement Redis auth and SSL

## 10. Benefits of Redis Integration

### Performance:
- **Fast Access**: In-memory storage for quick session retrieval
- **Scalability**: Multiple server instances share sessions
- **Memory Efficiency**: Automatic cleanup of expired sessions

### Reliability:
- **Persistence**: Sessions survive server restarts
- **Atomic Operations**: Thread-safe session updates
- **Backup**: Redis persistence options (RDB/AOF)

### Monitoring:
- **Statistics**: Built-in session and memory stats
- **Health Checks**: Redis connection monitoring
- **Debugging**: Session inspection endpoints

This Redis integration transforms your AI Gateway into a production-ready, scalable service with persistent conversation memory!