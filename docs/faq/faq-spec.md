Below is the **updated, clean, Codex-ready English specification**, now including **SearchModeExact** as a standalone mode.

This is a full, self-contained spec you can paste directly into your Codex spec or technical doc.

---

# FAQ Search Modes Specification (with SearchModeExact)

> **Infra note**: Local development uses SQLite for FAQ questions, answer cache, and trending queries. PostgreSQL/pgvector and Valkey/Redis remain optional legacy/integration backends configured through environment variables or `configs/config.yaml`.
> The canonical question table name is `questions` in both SQLite and Postgres. Older local SQLite databases that contain `faq_questions` are migrated into `questions` during startup and the legacy table is dropped.

This system supports four search modes:

* **SearchModeExact** – exact text match only
* **SearchModeSimilarity** – vector similarity search via pgvector
* **SearchModeSemanticHash** – hash-based approximate lookup (LSH)
* **SearchModeHybrid** – exact match → similarity fallback

Every mode attempts to reuse cached answers if possible; otherwise it inserts the question, generates an answer via the LLM, and stores the question plus cache data in the configured local or integration backend. The default local backend is SQLite.

---

# 1. Data Model

### 1.1 `questions` table

```sql
CREATE EXTENSION IF NOT EXISTS vector; -- 在数据库里启用 extension
CREATE TABLE questions (
    id              BIGSERIAL PRIMARY KEY,
    question_text   TEXT NOT NULL,
    embedding       VECTOR(1536) NOT NULL,     -- OpenAI embedding vector, small 1536, large 3072, large-v2 4096
    semantic_hash   BIGINT,                     -- used only for SemanticHash mode
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

SQLite uses the same table name and domain fields. The local adapter stores `embedding` as JSON text and stores `semantic_hash` as text to avoid requiring pgvector in local development.

### 1.2 Answer Cache

Local SQLite stores answer cache rows in `faq_answer_cache`, keyed by `question_id` and referencing `questions(id)`. Legacy/integration Valkey deployments may still use keys such as `"q:<question_id>"` for serialized answer JSON.

### 1.3 Trending Queries

Local SQLite stores recommendation counts in `faq_trending_queries`.

---

# 2. Common Functions

### 2.1 OpenAI Embedding

```text
Vector embedQuestion(questionText: string)
```

### 2.2 Semantic Hash Function

```text
uint64 semanticHash(Vector embedding)
```

* Implemented using SimHash / Random Hyperplane LSH
* Deterministic
* Uses pre-initialized random projection vectors

### 2.3 Cache Functions

```text
(string, bool) kvGet(key: string)
void kvSet(key: string, value: string)
```

For local SQLite, these operations map to `faq_answer_cache`; Valkey/KV behavior is retained only for legacy/integration deployments.

### 2.4 LLM Answer Generation

```text
string askLLM(questionText: string)
```

---

# 3. Search Modes

## 3.1 **SearchModeExact**

### Overview

Perform exact text match only (fastest, deterministic).
No semantic logic and no vector search.

### Steps

1. Query Postgres:

   ```sql
   SELECT id
   FROM questions
   WHERE question_text = $1
   LIMIT 1;
   ```
2. If exact match found:

   * Retrieve `"q:<id>"` from KV
   * If found → return answer
   * If missing → regenerate via LLM, store, return
3. If no match:

   * Compute embedding: `emb = embedQuestion(text)`
   * Insert:

     ```sql
     INSERT INTO questions (question_text, embedding)
     VALUES ($1, $2)
     RETURNING id;
     ```
   * Generate answer via LLM
   * Store `"q:<id>" → answer`
   * Return answer

### Use Case

* When strict textual equality is required
* Fast, deterministic, no approximation

---

## 3.2 **SearchModeSimilarity (pgvector)**

### Overview

Use pgvector to find nearest neighbor by vector distance.

### Steps

1. Compute embedding: `emb = embedQuestion(text)`
2. Query nearest neighbor:

   ```sql
   SELECT id, embedding
   FROM questions
   ORDER BY embedding <-> $emb
   LIMIT 1;
   ```
3. Check similarity threshold:

   * If distance < threshold → treat as match:

     * Load `"q:<id>"` → return
   * Else → treat as new FAQ
4. On new FAQ:

   * Insert `(question_text, embedding)`
   * Generate answer via LLM
   * Store `"q:<id>" → answer`
   * Return answer

### Use Case

* High accuracy semantic matching
* Best default mode

---

## 3.3 **SearchModeSemanticHash (LSH)**

### Overview

Approximate semantic lookup using hashing.
Does not use pgvector for retrieval.

### Steps

1. Compute embedding: `emb = getEmbedding(text)`
2. Compute semantic hash: `h = semanticHash(emb)`
3. Lookup by hash:

   ```sql
   SELECT id
   FROM questions
   WHERE semantic_hash = $1
   LIMIT 1;
   ```
4. If match found:

   * Load `"q:<id>"` → return
5. If no match:

   * Insert:

     ```sql
     INSERT INTO questions (question_text, embedding, semantic_hash)
     VALUES ($1, $2, $3)
     RETURNING id;
     ```
   * Generate answer with LLM
   * Store `"q:<id>" → answer`
   * Return answer

### Use Case

* Extremely fast approximate searching
* Suitable for large-scale (millions+) FAQ datasets

---

## 3.4 **SearchModeHybrid (Exact → Similarity)**

### Overview

First try exact match, then fallback to semantic similarity.

### Recommended Behavior

```
Exact search → pgvector similarity search
```

SemanticHash fallback is optional and NOT recommended for default behavior.

### Steps

1. **Exact match**:

   ```sql
   SELECT id
   FROM questions
   WHERE question_text = $1
   LIMIT 1;
   ```

   * If found → load `"q:<id>"` → return
2. **If no exact match**, perform full **SearchModeSimilarity** procedure:

   * embedding
   * pgvector nearest neighbor
   * threshold check
   * insert if needed
   * generate answer / store to KV
   * return answer

### Use Case

* Deterministic behavior with semantic fallback
* Good for user-facing FAQ UX

---

# 4. Summary Table

| Mode                       | Exact Match | pgvector | Semantic Hash | Characteristics                                |
| -------------------------- | ----------- | -------- | ------------- | ---------------------------------------------- |
| **SearchModeExact**        | ✔ Yes       | ✖ No     | ✖ No          | Fast, deterministic, no semantic matching      |
| **SearchModeSimilarity**   | ✖ No        | ✔ Yes    | ✖ No          | Highest semantic accuracy                      |
| **SearchModeSemanticHash** | ✖ No        | ✖ No     | ✔ Yes         | Very fast approximate lookup for huge datasets |
| **SearchModeHybrid**       | ✔ Yes       | ✔ Yes    | Optional      | Exact match first → semantic fallback          |

---
