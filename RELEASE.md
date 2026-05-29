# Release Runbook

Eywa uses Go sub-modules. Every module is tagged independently and **must be tagged in the correct order** — sub-modules must be tagged only after their `go.mod` references point to the already-published root tag. Tagging out of order publishes sub-modules that pin a stale root version, which is sticky and hard to fix.

## Pre-flight

```bash
# 1. All CI checks must be green on main
# 2. No uncommitted changes
git status

# 3. All tests pass locally
go test . ./internal/... -race
cd redis && go test ./... -race && cd ..
cd mongo && go test ./... -race && cd ..
cd fiber && go test ./... -race && cd ..
```

## Versioning

Eywa follows [Semantic Versioning](https://semver.org).

| Change | Version bump |
|--------|-------------|
| Bug fixes, internal refactors | PATCH (`v1.2.3 → v1.2.4`) |
| New features, new config fields | MINOR (`v1.2.3 → v1.3.0`) |
| Breaking API or behavior change | MAJOR (`v1.2.3 → v2.0.0`) |

For breaking changes, also update the API snapshot:

```bash
go doc -all github.com/wmulabs/eywa > .api-snapshot.txt
git add .api-snapshot.txt
```

## Release Steps (exact order — do not skip)

### Step 1: Tag and push the root module

```bash
VERSION=v1.2.3

git tag "${VERSION}"
git push origin "${VERSION}"
```

### Step 2: Wait for pkg.go.dev to index the root tag

This usually takes 2–5 minutes. Verify before proceeding:

```bash
# Retry until this returns 200
curl -s -o /dev/null -w "%{http_code}" \
  "https://proxy.golang.org/github.com/wmulabs/eywa/@v/${VERSION}.info"
```

You can also trigger indexing manually:

```bash
GOPROXY=https://proxy.golang.org go install "github.com/wmulabs/eywa@${VERSION}" 2>/dev/null || true
```

### Step 3: Update all sub-module go.mod files to pin the new root tag

```bash
VERSION=v1.2.3

for dir in mongo redis mcp \
  providers/anthropic providers/openai providers/gemini \
  providers/bedrock providers/weaviate providers/qdrant \
  providers/pgvector providers/pinecone providers/vertexai \
  gcp/cloudtasks gcp/gcs gcp/gemini \
  channels/whatsapp fiber; do
  (cd "$dir" && go get "github.com/wmulabs/eywa@${VERSION}" && go mod tidy)
done

git add '*/go.mod' '*/go.sum'
git commit -m "chore: bump eywa root to ${VERSION} in sub-module go.mod files"
git push origin main
```

### Step 4: Tag all sub-modules and push

```bash
VERSION=v1.2.3

for dir in mongo redis mcp \
  providers/anthropic providers/openai providers/gemini \
  providers/bedrock providers/weaviate providers/qdrant \
  providers/pgvector providers/pinecone providers/vertexai \
  gcp/cloudtasks gcp/gcs gcp/gemini \
  channels/whatsapp fiber; do
  git tag "${dir}/${VERSION}"
done

git push origin --tags
```

### Step 5: Create GitHub Release

Create a GitHub Release from the root tag (`v1.2.3`). Include:
- Breaking changes (if any)
- New features
- Bug fixes
- Sub-modules updated (with their tags)

### Step 6: Verify

```bash
VERSION=v1.2.3

# Confirm all tags pushed
git tag -l | grep "${VERSION}" | sort

# Confirm root module visible on pkg.go.dev (may take a few minutes)
curl -s "https://pkg.go.dev/github.com/wmulabs/eywa@${VERSION}" | grep -q "${VERSION}" \
  && echo "root OK" || echo "root not yet indexed"

# Spot-check one sub-module
curl -s "https://pkg.go.dev/github.com/wmulabs/eywa/redis@${VERSION}" | grep -q "${VERSION}" \
  && echo "redis OK" || echo "redis not yet indexed"
```

## Module dependency order (for reference)

```
Root:
  github.com/wmulabs/eywa                    ← tag first

Independent of each other (tag after root is indexed):
  github.com/wmulabs/eywa/mongo
  github.com/wmulabs/eywa/redis
  github.com/wmulabs/eywa/mcp
  github.com/wmulabs/eywa/providers/anthropic
  github.com/wmulabs/eywa/providers/openai
  github.com/wmulabs/eywa/providers/gemini
  github.com/wmulabs/eywa/providers/bedrock
  github.com/wmulabs/eywa/providers/weaviate
  github.com/wmulabs/eywa/providers/qdrant
  github.com/wmulabs/eywa/providers/pgvector
  github.com/wmulabs/eywa/providers/pinecone
  github.com/wmulabs/eywa/providers/vertexai
  github.com/wmulabs/eywa/gcp/cloudtasks
  github.com/wmulabs/eywa/gcp/gcs
  github.com/wmulabs/eywa/gcp/gemini
  github.com/wmulabs/eywa/channels/whatsapp

Depends on root + potentially others:
  github.com/wmulabs/eywa/fiber               ← tag last
```
