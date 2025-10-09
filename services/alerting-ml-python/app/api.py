from __future__ import annotations

from typing import Dict, List, Optional

import pandas as pd
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from .isoforest_detect import run_isolation_forest, anomalies_to_json


# Default thresholds for segment-level anomaly decision
DEFAULT_RATIO_THRESHOLD: float = 0.20
DEFAULT_STREAK_THRESHOLD: int = 20


class Metadata(BaseModel):
    alert_name: Optional[str] = ""
    severity: Optional[str] = ""
    labels: Dict[str, str] = Field(default_factory=dict)


class DataPoint(BaseModel):
    timestamp: str
    value: float


class DetectRequest(BaseModel):
    metadata: Optional[Metadata] = None
    data: List[DataPoint]
    contamination: float = 0.05
    random_state: int = 42
    ratio_threshold: float = DEFAULT_RATIO_THRESHOLD
    streak_threshold: int = DEFAULT_STREAK_THRESHOLD


class Anomaly(BaseModel):
    start: str
    end: str


class DetectResponse(BaseModel):
    metadata: Optional[Metadata] = None
    anomalies: List[Anomaly] = Field(default_factory=list)


def _segment_summary(
    point_results: List[Dict],
    ratio_threshold: float = DEFAULT_RATIO_THRESHOLD,
    streak_threshold: int = DEFAULT_STREAK_THRESHOLD,
) -> Dict:
    total = len(point_results)
    if total == 0:
        return {
            "is_segment_anomaly": False,
            "reason": "empty",
            "rules": {"ratio_threshold": ratio_threshold, "streak_threshold": streak_threshold},
        }

    anomaly_flags = [bool(p.get("is_anomaly")) for p in point_results]
    ratio = sum(anomaly_flags) / total

    max_streak = 0
    current_streak = 0
    for flag in anomaly_flags:
        if flag:
            current_streak += 1
            max_streak = max(max_streak, current_streak)
        else:
            current_streak = 0

    is_anom = (ratio >= ratio_threshold) or (max_streak >= streak_threshold)
    return {
        "is_segment_anomaly": is_anom,
        "anomaly_ratio": ratio,
        "max_consecutive_anomaly": max_streak,
        "rules": {"ratio_threshold": ratio_threshold, "streak_threshold": streak_threshold},
    }


def _extract_anomaly_intervals(point_results: List[Dict]) -> List[Dict]:
    n = len(point_results)
    intervals: List[Dict] = []
    i = 0
    while i < n:
        item = point_results[i]
        if not bool(item.get("is_anomaly")):
            i += 1
            continue

        start_idx = i
        while i < n and bool(point_results[i].get("is_anomaly")):
            i += 1
        end_idx = i - 1

        start_ts = str(point_results[start_idx].get("timestamp"))
        end_ts = str(point_results[end_idx].get("timestamp"))
        intervals.append({"start": start_ts, "end": end_ts})

    return intervals


def create_app() -> FastAPI:
    app = FastAPI(title="alerting-ml", version="0.1.0")

    @app.get("/healthz")
    def healthz() -> Dict[str, str]:
        return {"status": "ok"}

    @app.post("/api/v1/anomaly/detect", response_model=DetectResponse)
    def detect(req: DetectRequest) -> DetectResponse:
        try:
            # Build dataframe from payload
            df = pd.DataFrame([{"timestamp": p.timestamp, "value": p.value} for p in req.data])
            if not {"timestamp", "value"}.issubset(df.columns):
                raise HTTPException(status_code=400, detail="data points must include timestamp and value")

            df = df.sort_values("timestamp").reset_index(drop=True)

            # Run IsolationForest point anomaly detection
            anomalies = run_isolation_forest(df, contamination=req.contamination, random_state=req.random_state)
            point_json = anomalies_to_json(anomalies)

            # Segment-level summary and intervals
            seg = _segment_summary(point_json, req.ratio_threshold, req.streak_threshold)
            intervals: List[Dict] = _extract_anomaly_intervals(point_json) if seg.get("is_segment_anomaly") else []

            return DetectResponse(
                metadata=req.metadata,
                anomalies=[Anomaly(start=iv["start"], end=iv["end"]) for iv in intervals],
            )
        except HTTPException:
            raise
        except Exception as e:  # pragma: no cover
            raise HTTPException(status_code=500, detail=f"detection failed: {e}")

    return app


