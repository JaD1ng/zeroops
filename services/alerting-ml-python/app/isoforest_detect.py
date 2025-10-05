from __future__ import annotations

import json
import argparse
from dataclasses import dataclass
from typing import List, Dict, Any, Tuple, Union

import numpy as np
import pandas as pd
from sklearn.ensemble import IsolationForest


@dataclass
class PointAnomaly:
    timestamp: str
    value: float
    score: float  # anomaly score (the lower, the more abnormal in sklearn's convention)
    is_anomaly: bool


def load_time_series_from_json(path: str) -> pd.DataFrame:
    with open(path, "r", encoding="utf-8") as f:
        payload = json.load(f)
    records = payload.get("data", [])
    df = pd.DataFrame(records)
    if not {"timestamp", "value"}.issubset(df.columns):
        raise ValueError("输入JSON缺少 timestamp 或 value 字段")
    # 排序并确保数值型
    df = df.sort_values("timestamp").reset_index(drop=True)
    df["value"] = pd.to_numeric(df["value"], errors="coerce")
    df = df.dropna(subset=["value"])  # 简单清洗
    return df


def detect_point_anomalies(values: np.ndarray,
                           contamination: float = 0.05,
                           random_state: int = 42,
                           n_estimators: int = 200,
                           max_samples: Union[str, int] = "auto") -> Tuple[np.ndarray, np.ndarray]:
    """
    返回: (pred_labels, scores)
    - pred_labels: 1 正常, -1 异常（与sklearn一致）
    - scores: decision_function 得分（越小越异常）
    """
    model = IsolationForest(
        n_estimators=n_estimators,
        contamination=contamination,
        random_state=random_state,
        max_samples=max_samples,
        n_jobs=-1,
    )
    values_2d = values.reshape(-1, 1)
    model.fit(values_2d)
    pred_labels = model.predict(values_2d)  # 1 或 -1
    scores = model.decision_function(values_2d)  # 值越小越异常
    return pred_labels, scores


def run_isolation_forest(df: pd.DataFrame,
                         contamination: float = 0.05,
                         random_state: int = 42) -> List[PointAnomaly]:
    values = df["value"].to_numpy(dtype=float)
    pred_labels, scores = detect_point_anomalies(values, contamination=contamination, random_state=random_state)
    results: List[PointAnomaly] = []
    for ts, val, score, label in zip(df["timestamp"].tolist(), values.tolist(), scores.tolist(), pred_labels.tolist()):
        results.append(PointAnomaly(timestamp=ts, value=val, score=score, is_anomaly=(label == -1)))
    return results


def anomalies_to_json(anomalies: List[PointAnomaly]) -> List[Dict[str, Any]]:
    return [
        {
            "timestamp": a.timestamp,
            "value": a.value,
            "score": a.score,
            "is_anomaly": a.is_anomaly,
        }
        for a in anomalies
    ]


def cli() -> None:
    parser = argparse.ArgumentParser(description="IsolationForest 时序逐点异常检测")
    parser.add_argument("--input", required=True, help="输入JSON路径，结构需包含 data[timestamp,value]")
    parser.add_argument("--output", required=True, help="输出结果JSON路径")
    parser.add_argument("--contamination", type=float, default=0.05, help="异常比例(0-0.5)，默认0.05")
    parser.add_argument("--random_state", type=int, default=42)
    args = parser.parse_args()

    df = load_time_series_from_json(args.input)
    anomalies = run_isolation_forest(df, contamination=args.contamination, random_state=args.random_state)

    out = {
        "metadata": {
            "method": "IsolationForest",
            "contamination": args.contamination,
            "random_state": args.random_state,
            "total_points": len(anomalies),
        },
        "point_anomalies": anomalies_to_json(anomalies),
    }

    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)


if __name__ == "__main__":
    cli()
