"""
MongoDB handler with bulk write (upsert) operations.
"""

import logging
from datetime import datetime, timezone
from typing import Any

from pymongo import MongoClient, UpdateOne
from pymongo.errors import BulkWriteError, PyMongoError

logger = logging.getLogger(__name__)


class MongoHandler:
    """Manages MongoDB connections and bulk upsert operations."""

    def __init__(self, uri: str, database: str):
        """
        Args:
            uri: MongoDB connection string (e.g. mongodb://user:pass@host:27017).
            database: Name of the MongoDB database to use.
        """
        self._client = MongoClient(uri, serverSelectionTimeoutMS=5000)
        self._db = self._client[database]
        logger.info("Connected to MongoDB database '%s'", database)

    def close(self) -> None:
        self._client.close()

    # ------------------------------------------------------------------
    # Generic helpers
    # ------------------------------------------------------------------

    def _collection(self, name: str):
        return self._db[name]

    def bulk_upsert(
        self,
        collection_name: str,
        documents: list[dict],
        filter_keys: list[str],
    ) -> dict:
        """
        Upsert *documents* into *collection_name* using bulkWrite.

        Each document is matched on the combination of *filter_keys*.
        A ``fetched_at`` timestamp is added automatically.

        Args:
            collection_name: Target MongoDB collection.
            documents: List of documents to upsert.
            filter_keys: Field names used to build the upsert filter
                         (e.g. ['oid'] or ['book', 'tid']).

        Returns:
            A dict with ``matched``, ``modified``, ``upserted`` counts.
        """
        if not documents:
            logger.debug("bulk_upsert: no documents for '%s'", collection_name)
            return {"matched": 0, "modified": 0, "upserted": 0}

        now = datetime.now(timezone.utc)
        operations = []
        for doc in documents:
            doc_copy = dict(doc)
            doc_copy["fetched_at"] = now
            flt = {k: doc_copy[k] for k in filter_keys if k in doc_copy}
            operations.append(
                UpdateOne(flt, {"$set": doc_copy}, upsert=True)
            )

        try:
            result = self._collection(collection_name).bulk_write(
                operations, ordered=False
            )
            summary = {
                "matched": result.matched_count,
                "modified": result.modified_count,
                "upserted": result.upserted_count,
            }
            logger.info(
                "bulk_upsert '%s': %s", collection_name, summary
            )
            return summary
        except BulkWriteError as exc:
            logger.error(
                "BulkWriteError for '%s': %s", collection_name, exc.details
            )
            raise
        except PyMongoError as exc:
            logger.error("PyMongoError for '%s': %s", collection_name, exc)
            raise

    # ------------------------------------------------------------------
    # Domain-specific upserts
    # ------------------------------------------------------------------

    def save_ticker(self, ticker_data: dict) -> dict:
        """Save a ticker snapshot (keyed by book + created_at)."""
        payload = ticker_data.get("payload", {})
        if not payload:
            return {"matched": 0, "modified": 0, "upserted": 0}
        return self.bulk_upsert(
            "tickers", [payload], filter_keys=["book", "created_at"]
        )

    def save_balance(self, user: str, balance_data: dict) -> dict:
        """Save balance items (keyed by user + currency)."""
        balances: list[dict[str, Any]] = (
            balance_data.get("payload", {}).get("balances", [])
        )
        for item in balances:
            item["user"] = user
        return self.bulk_upsert("balances", balances, filter_keys=["user", "currency"])

    def save_open_orders(self, user: str, orders_data: dict) -> dict:
        """Save open orders (keyed by oid)."""
        orders: list[dict[str, Any]] = orders_data.get("payload", [])
        for item in orders:
            item["user"] = user
        return self.bulk_upsert("open_orders", orders, filter_keys=["oid"])

    def save_trades(self, user: str, trades_data: dict) -> dict:
        """Save user trades (keyed by tid)."""
        trades: list[dict[str, Any]] = trades_data.get("payload", [])
        for item in trades:
            item["user"] = user
        return self.bulk_upsert("trades", trades, filter_keys=["tid"])

    def save_orders(self, user: str, orders_data: dict) -> dict:
        """Save closed/historical orders (keyed by oid)."""
        orders: list[dict[str, Any]] = orders_data.get("payload", [])
        for item in orders:
            item["user"] = user
        return self.bulk_upsert("orders", orders, filter_keys=["oid"])
