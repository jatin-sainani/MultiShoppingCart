#!/usr/bin/env python3
import argparse
import json
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib import error, request


def utc_timestamp() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def call_api(base_url: str, method: str, path: str, payload: dict | None) -> tuple[int, bytes, float, bool]:
    body = None if payload is None else json.dumps(payload).encode("utf-8")
    req = request.Request(
        base_url.rstrip("/") + path,
        data=body,
        method=method,
        headers={"Content-Type": "application/json"},
    )

    start = time.perf_counter()
    try:
        with request.urlopen(req, timeout=10) as response:
            response_body = response.read()
            elapsed_ms = round((time.perf_counter() - start) * 1000, 3)
            return response.status, response_body, elapsed_ms, True
    except error.HTTPError as exc:
        elapsed_ms = round((time.perf_counter() - start) * 1000, 3)
        return exc.code, exc.read(), elapsed_ms, False


def record(results: list[dict], operation: str, status_code: int, response_time: float, success: bool) -> None:
    results.append(
        {
            "operation": operation,
            "response_time": response_time,
            "success": success,
            "status_code": status_code,
            "timestamp": utc_timestamp(),
        }
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Run the required 150-operation shopping cart test.")
    parser.add_argument("--base-url", required=True, help="API base URL, for example http://localhost:8080")
    parser.add_argument("--output", required=True, help="Where to write the JSON result array")
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")
    output_path = Path(args.output)
    results: list[dict] = []
    cart_ids: list[int] = []

    for i in range(50):
        status, body, elapsed_ms, ok = call_api(base_url, "POST", "/shopping-carts", {"customer_id": 1000 + i})
        record(results, "create_cart", status, elapsed_ms, ok and status == 201)
        if status == 201:
            cart_ids.append(json.loads(body.decode("utf-8"))["shopping_cart_id"])

    if len(cart_ids) != 50:
        print(f"expected 50 carts, got {len(cart_ids)}", file=sys.stderr)
        output_path.write_text(json.dumps(results, indent=2), encoding="utf-8")
        return 1

    for i, cart_id in enumerate(cart_ids):
        status, _, elapsed_ms, ok = call_api(
            base_url,
            "POST",
            f"/shopping-carts/{cart_id}/items",
            {"product_id": 2000 + i, "quantity": (i % 5) + 1},
        )
        record(results, "add_items", status, elapsed_ms, ok and status == 204)

    for cart_id in cart_ids:
        status, _, elapsed_ms, ok = call_api(base_url, "GET", f"/shopping-carts/{cart_id}", None)
        record(results, "get_cart", status, elapsed_ms, ok and status == 200)

    output_path.write_text(json.dumps(results, indent=2), encoding="utf-8")
    print(f"wrote {len(results)} operations to {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
