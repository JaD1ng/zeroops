# alerting-ml-python

FastAPI service that exposes HTTP + OpenAPI endpoint to perform anomaly detection on a single time series using IsolationForest. Intended to be called by the Go `internal/alerting/service/healthcheck` scheduler.

## Run locally

```bash
# From repo root
cd services/alerting-ml-python
python -m app.main
# Service listens on 0.0.0.0:8081
```

Or with Docker:

```bash
docker build -t zeroops/alerting-ml:dev services/alerting-ml-python
docker run --rm -p 8081:8081 zeroops/alerting-ml:dev
```

## API

- OpenAPI: `api/openapi/alerting-ml.yaml`
- Health: `GET /healthz`
- Detect: `POST /api/v1/anomaly/detect`

Request example:

```json
{
  "metadata": {
    "alert_name": "http_latency",
    "severity": "P0",
    "labels": {"service": "s3", "version": "v1.0.4"}
  },
  "data": [
    {"timestamp": "2025-01-01T00:00:00Z", "value": 12.3},
    {"timestamp": "2025-01-01T00:01:00Z", "value": 18.2}
  ]
}
```

Response example:

```json
{
  "metadata": {
    "alert_name": "http_latency",
    "severity": "P0",
    "labels": {"service": "s3", "version": "v1.0.4"}
  },
  "anomalies": [
    {"start": "2025-01-01T00:00:00Z", "end": "2025-01-01T00:06:00Z"}
  ]
}
```

## Go integration

- Environment variable in Go service:
  - `ANOMALY_DETECTION_API_URL` default: `http://localhost:8081/api/v1/anomaly/detect`
- The Go client already posts payload matching the request format and accepts RFC3339 timestamps in response. No code changes are required besides configuring the service URL.

## Notes

- Thresholds can be tuned per-request via `ratio_threshold` and `streak_threshold`.
- `contamination` controls IsolationForest anomaly proportion; default 0.05.

