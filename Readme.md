# 🧠 AI Gateway with Redis Integration

A high-performance, production-ready AI Gateway built with **Go (Gin Framework)**, **OpenAI API**, and **Redis** for session persistence and scalability. Designed for streaming and non-streaming AI interactions, with full support for stateful chat and Redis-backed session management.

---

## 🚀 Features

- 🔌 **OpenAI Proxy API** (`/ai-agent`)
- 🧠 **Contextual Conversations** using Redis-backed session storage
- 🧮 **Session TTL & Expiration** (configurable)
- 📊 **Memory & Stats Endpoints**
- 📁 **Dockerized Setup** with Redis
- ☁️ Compatible with local or cloud-hosted Redis (e.g., Upstash, Redis Cloud)
- 🧰 Simple `.env` configuration for flexible deployments

---

## 📦 Tech Stack

- **Backend**: Go (Gin)
- **AI API**: OpenAI API (GPT-4 / GPT-3.5)
- **Session Storage**: Redis
- **Containerization**: Docker & Docker Compose
- **Environment Management**: `godotenv`

---

## 🌐 API Endpoints

### 🧠 POST `/ai-agent`
Handles AI prompts and session-based memory.

**Request JSON:**
```json
{
  "prompt": "Hello, who are you?",
  "session_id": "optional-session-id",
  "stream": false
}
```

### 📁 GET `/session/:session_id`
Retrieve full session conversation by session ID.

### 📄 GET `/sessions`
List all active session IDs stored in Redis.

### ❌ DELETE `/session/:session_id`
Delete a session and its associated memory.

### 📊 GET `/stats`
Returns system and Redis usage stats:
```json
{
  "total_sessions": 3,
  "memory_usage": "150 KB",
  "uptime": "3 hours"
}
```

---

## 🔐 Environment Configuration

Sample `.env` variables:
```env
# OpenAI Configuration
OPENAI_API_KEY=sk-xxx

# Redis Configuration
REDIS_URL=redis://localhost:6379
SESSION_TTL_HOURS=24

# Server
PORT=8080
```

---

## 🐳 Docker & Redis Integration

Includes:
- `Dockerfile` for multi-stage Go build
- `docker-compose.yml` with Redis service
- Automatic environment injection via `.env`

---

## ☁️ Deployment-Ready

- ✅ Works with **Upstash**, **Redis Cloud**, **Railway**, and more
- ✅ Scalable Redis-backed session store
- ✅ Stateless Go services ready for container orchestration

---

## 📈 Benefits of Redis Integration

| Feature               | Benefit                                           |
|----------------------|----------------------------------------------------|
| Persistent Sessions  | Survive server restarts                           |
| TTL Management       | Automatic memory cleanup                          |
| Shared Memory        | Scalable across containers or nodes               |
| Fast Access          | In-memory key-value retrieval                     |
| Statistics & Metrics | Built-in health and usage monitoring              |

---

## 📬 Example Usage

**Simple Prompt Call:**
```bash
curl -X POST http://localhost:8080/ai-agent \
  -H "Content-Type: application/json" \
  -d '{"prompt": "Tell me a joke!"}'
```

**Stateful Chat with Session:**
```bash
curl -X POST http://localhost:8080/ai-agent \
  -H "Content-Type: application/json" \
  -d '{"prompt": "My name is John", "session_id": "john123"}'
```

---

## 🧩 License

MIT — feel free to use, extend, or contribute!

---

## 👨‍💻 Author

**Muhammad Ashraful**  
2.5+ years full-stack experience — Next.js, Go, AI & cloud-native tools  
[GitHub](https://github.com/your-username) | [LinkedIn](https://linkedin.com/in/your-profile)

---
