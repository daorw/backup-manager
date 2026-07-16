# API Communication Patterns

## Backend → Response Format

All API endpoints under `/api/v1` use this wrapper:

**Success:**
```json
{"data": <payload>}
```

**Error:**
```json
{"error": "<message>"}
```

## Backend → Error Mapping

```go
func respondError(c *gin.Context, err error) {
    msg := err.Error()
    switch {
    case strings.Contains(msg, "not found"):
        c.JSON(http.StatusNotFound, gin.H{"error": msg})
    case strings.Contains(msg, "is required") || strings.Contains(msg, "invalid"):
        c.JSON(http.StatusBadRequest, gin.H{"error": msg})
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
    }
}
```

## Backend → Router Setup Pattern

```go
func SetupRouter(
    repoHandler *handler.RepoHandler,
    // ... other handlers ...
) *gin.Engine {
    gin.SetMode(gin.ReleaseMode)

    r := gin.New()
    r.Use(CORSMiddleware(DefaultCORSConfig()))
    r.Use(ErrorRecoveryMiddleware())

    v1 := r.Group("/api/v1")
    {
        v1.GET("/health", systemHandler.Health)
        v1.POST("/repos", repoHandler.Create)
        v1.GET("/repos", repoHandler.List)
        // ... more routes ...
    }

    return r
}
```

## Frontend → Axios Client

```typescript
const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});

// Response interceptor: unwrap { data: ... }
api.interceptors.response.use(
  (response) => {
    if (response.data && typeof response.data === 'object' && 'data' in response.data) {
      response.data = response.data.data;
    }
    return response;
  },
  (error) => {
    const message = error.response?.data?.error || error.message || 'Request failed';
    return Promise.reject(new Error(message));
  }
);
```

## Frontend → API Functions

Exported functions from `api/client.ts`:

```typescript
// GET — params in URL
export async function fetchRepos(): Promise<BackupRepo[]> {
  const { data } = await api.get('/repos');
  return data;
}

// GET with query
export async function fetchHistory(repoId: string, limit = 20, offset = 0): Promise<CommitEntry[]> {
  const { data } = await api.get(`/repos/${repoId}/backup/history`, {
    params: { limit, offset },
  });
  return data;
}

// POST — JSON body
export async function createRepo(req: CreateRepoRequest): Promise<BackupRepo> {
  const { data } = await api.post('/repos', req);
  return data;
}

// PUT — JSON body
export async function updateConfig(repoId: string, req: UpdateConfigRequest): Promise<void> {
  await api.put(`/repos/${repoId}/config`, req);
}

// DELETE
export async function deleteRepo(id: string): Promise<void> {
  await api.delete(`/repos/${id}`);
}
```

## Type Synchronization

Backend Go structs map to frontend TypeScript interfaces:

| Go Type | TypeScript | Notes |
|---------|------------|-------|
| `string` | `string` | — |
| `int64` | `number` | Size/count fields |
| `bool` | `boolean` | — |
| `time.Time` | `string` | ISO 8601/RFC 3339 |
| `*time.Time` | `string \| null` | Optional timestamp |
| `[]byte` | `string` | Base64-encoded in JSON responses |
| Enum type | `'a' \| 'b' \| 'c'` | String literal union |

**Example mapping:**

Go:
```go
type Repo struct {
    ID           string     `json:"id"`
    Name         string     `json:"name"`
    Status       RepoStatus `json:"status"`
    LastBackupAt *time.Time `json:"last_backup_at,omitempty"`
}
```

TypeScript:
```typescript
export interface Repo {
  id: string;
  name: string;
  status: 'active' | 'error' | 'backing_up';
  last_backup_at?: string;
}
```

## CORS Configuration

```go
func DefaultCORSConfig() CORSConfig {
    return CORSConfig{
        AllowOrigins: []string{"*"},
        AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
    }
}
```

Preflight (`OPTIONS`) requests return `204 No Content` immediately.
