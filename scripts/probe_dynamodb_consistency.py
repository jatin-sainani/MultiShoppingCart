#!/usr/bin/env python3
import argparse
import json
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib import error, request


def utc_timestamp() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def call_api(base_url: str, method: str, path: str, payload: dict | None) -> tuple[int, bytes]:
    body = None if payload is None else json.dumps(payload).encode("utf-8")
    req = request.Request(
        base_url.rstrip("/") + path,
        data=body,
        method=method,
        headers={"Content-Type": "application/json"},
    )
    try:
        with request.urlopen(req, timeout=10) as response:
            return response.status, response.read()
    except error.HTTPError as exc:
        return exc.code, exc.read()


def main() -> int:
    parser = argparse.ArgumentParser(description="Probe eventual-consistency behavior through the API.")
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--iterations", type=int, default=20)
    parser.add_argument("--output", default="dynamodb_consistency_observations.json")
    args = parser.parse_args()

    observations = []
    for i in range(args.iterations):
        create_status, create_body = call_api(args.base_url, "POST", "/shopping-carts", {"customer_id": 9000 + i})
        cart_id = None
        if create_status == 201:
            cart_id = json.loads(create_body.decode("utf-8"))["shopping_cart_id"]

        create_read_status, _ = call_api(args.base_url, "GET", f"/shopping-carts/{cart_id}", None) if cart_id else (0, b"")
        add_status, _ = call_api(
            args.base_url,
            "POST",
            f"/shopping-carts/{cart_id}/items",
            {"product_id": 4000 + i, "quantity": 1},
        ) if cart_id else (0, b"")
        add_read_status, add_read_body = call_api(args.base_url, "GET", f"/shopping-carts/{cart_id}", None) if cart_id else (0, b"")

        item_visible = False
        if add_read_status == 200:
            decoded = json.loads(add_read_body.decode("utf-8"))
            item_visible = any(item["product_id"] == 4000 + i and item["quantity"] == 1 for item in decoded.get("items", []))

        observations.append(
            {
                "iteration": i + 1,
                "timestamp": utc_timestamp(),
                "create_status": create_status,
                "immediate_get_after_create_status": create_read_status,
                "add_status": add_status,
                "immediate_get_after_add_status": add_read_status,
                "item_visible_immediately": item_visible,
            }
        )
        time.sleep(0.2)

    output_path = Path(args.output)
    output_path.write_text(json.dumps(observations, indent=2), encoding="utf-8")
    print(f"wrote {len(observations)} observations to {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
