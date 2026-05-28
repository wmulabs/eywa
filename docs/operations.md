# ⚙️ Operations Guide

## Data Retention

### Chronicle (Audit Log)

Chronicle records every Pulse interaction and grows without bound by default.
MongoDB does not automatically delete documents.

**Recommended TTL index (apply once at deployment):**

```javascript
// MongoDB shell — delete Chronicle records after 90 days
db.chronicles.createIndex(
  { "created_at": 1 },
  { expireAfterSeconds: 7776000 } // 90 days
)
```

Adjust `expireAfterSeconds` to match your retention policy.

**GDPR deletion:** Chronicle entries contain `MemoryKey` (session identifier).
To delete all records for a user, run:

```javascript
db.chronicles.deleteMany({ "memory_key": "user:12345" })
```

Automate this in your data subject erasure flow.

### Echo (Message History)

Echoes persist indefinitely by default. Apply a TTL index or archive old sessions:

```javascript
// Delete Echo sessions inactive for 1 year
db.echoes.createIndex(
  { "updated_at": 1 },
  { expireAfterSeconds: 31536000 }
)
```

### Chronicle Immutability

MongoDB's Chronicle implementation is append-only **by convention**, not enforcement.
Any process with write access can modify or delete records.

For deployments requiring genuine tamper-proof audit logs:
- Use MongoDB Atlas Data Federation (read-only audit collection)
- Apply `{ w: "majority", j: true }` write concern and restrict database roles
- Export nightly to an immutable store (AWS S3 Object Lock, GCS with Bucket Lock)

---

## Operational Security

### Secrets Management

| Secret | How to pass | Never do |
|---|---|---|
| Oracle API keys | Environment variable | Hard-code in source |
| MongoDB URI | Environment variable or Secret Manager | Log it |
| Redis password | Environment variable or Secret Manager | Commit to VCS |
| JWT signing key | Environment variable or Secret Manager | Hard-code |
| OIDC service account key | GCP Workload Identity (preferred) | Ship JSON key file |

**On GCP Cloud Run:** Use Secret Manager references in Cloud Run service YAML.

### What Not to Log

Eywa's structured logging (Zap) logs metadata, not message content. When extending
Eywa (custom Scouts, Actions, Voices), follow the same principle:

**Never log:**
- `event.UserMessage` — may contain PII, credentials, or health data
- Action parameters that include user-provided input
- Oracle responses in full — may echo PII from the conversation
- Session `MemoryKey` values in external systems (they identify users)

**Safe to log:**
- `event.EventType`, Spirit name, Action name, error codes
- Latency, token counts (from Ledger), success/failure flags

### Network Isolation

The internal endpoint `POST /internal/execute-event` (used by Cloud Tasks) must not
be publicly reachable. Options:

- **Cloud Run:** Use a VPC connector and route Cloud Tasks through the internal network
- **Kubernetes:** Use a `NetworkPolicy` to restrict `/internal/*` to Cloud Tasks source IPs
- **Nginx/ingress:** Block `/internal/*` routes at the edge

The Fiber integration provides an OIDC middleware for Cloud Tasks — use it even on
internal networks as defense in depth.

---

## Model Deprecation

When a provider retires a model (e.g. `gpt-4-0314`, `claude-2`):

1. **Identify affected Spirits:**
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     https://your-service/api/v1/spirits | \
     jq '.[] | select(.model_name | test("deprecated-model-name"))'
   ```

2. **Update via Management API:**
   ```bash
   curl -X PUT -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     https://your-service/api/v1/spirits/my-spirit \
     -d '{"model_name": "new-model-name"}'
   ```

3. **Or use model routing (proactive):** Configure `WeaveConfig.ModelRoutingRules` to
   redirect from the deprecated model to a replacement before the deprecation date.

4. **Monitor Chronicle** for `ErrReasoningFailed` spikes in the 24 hours after a model
   retirement date — they indicate Spirits still using the old model name.

5. **Test after update:** Send a test Pulse to each updated Spirit via
   `POST /api/v1/events/:event_key` before returning traffic.
