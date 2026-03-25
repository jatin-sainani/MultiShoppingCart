#!/usr/bin/env python3
import argparse
import json
import math
import statistics
from pathlib import Path


EXPECTED_COUNTS = {
    "create_cart": 50,
    "add_items": 50,
    "get_cart": 50,
}


def percentile(values: list[float], pct: int) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = math.ceil((pct / 100) * len(ordered)) - 1
    index = max(0, min(index, len(ordered) - 1))
    return round(ordered[index], 3)


def validate_dataset(name: str, rows: list[dict]) -> None:
    if len(rows) != 150:
        raise ValueError(f"{name} must contain exactly 150 operations, found {len(rows)}")

    counts = {key: 0 for key in EXPECTED_COUNTS}
    for row in rows:
        counts[row["operation"]] = counts.get(row["operation"], 0) + 1

    for operation, expected in EXPECTED_COUNTS.items():
        if counts.get(operation, 0) != expected:
            raise ValueError(f"{name} must contain {expected} {operation} operations, found {counts.get(operation, 0)}")


def summarize(rows: list[dict]) -> dict:
    latencies = [float(row["response_time"]) for row in rows]
    successes = [row for row in rows if row["success"]]
    by_operation = {}
    for operation in EXPECTED_COUNTS:
        op_rows = [row for row in rows if row["operation"] == operation]
        by_operation[operation] = round(statistics.mean(float(row["response_time"]) for row in op_rows), 3)

    return {
        "avg_response_time_ms": round(statistics.mean(latencies), 3),
        "p50_response_time_ms": percentile(latencies, 50),
        "p95_response_time_ms": percentile(latencies, 95),
        "p99_response_time_ms": percentile(latencies, 99),
        "success_rate_pct": round((len(successes) / len(rows)) * 100, 3),
        "operation_averages_ms": by_operation,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Merge and analyze MySQL and DynamoDB result files.")
    parser.add_argument("--mysql", default="mysql_test_results.json")
    parser.add_argument("--dynamodb", default="dynamodb_test_results.json")
    parser.add_argument("--combined", default="combined_results.json")
    parser.add_argument("--summary", default="analysis_summary.json")
    args = parser.parse_args()

    mysql_rows = json.loads(Path(args.mysql).read_text(encoding="utf-8"))
    dynamo_rows = json.loads(Path(args.dynamodb).read_text(encoding="utf-8"))

    validate_dataset("mysql_test_results.json", mysql_rows)
    validate_dataset("dynamodb_test_results.json", dynamo_rows)

    combined = [{"backend": "mysql", **row} for row in mysql_rows] + [{"backend": "dynamodb", **row} for row in dynamo_rows]
    Path(args.combined).write_text(json.dumps(combined, indent=2), encoding="utf-8")

    summary = {
        "mysql": summarize(mysql_rows),
        "dynamodb": summarize(dynamo_rows),
    }
    Path(args.summary).write_text(json.dumps(summary, indent=2), encoding="utf-8")
    print(json.dumps(summary, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
