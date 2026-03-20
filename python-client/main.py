"""
Entry point for the Bitso Python client.

Loads configuration from environment variables (or a local .env file),
then starts a scheduler that periodically queries the Go API wrapper and
stores the results in MongoDB.
"""

import logging
import os
import sys

from dotenv import load_dotenv

from bitso_client import BitsoClient
from mongo_handler import MongoHandler
from scheduler import build_scheduler

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s – %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%S",
    stream=sys.stdout,
)
logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Core sync job
# ---------------------------------------------------------------------------

def sync_user(user: str, books: list[str], mongo: MongoHandler) -> None:
    """Fetch all data for *user* and upsert into MongoDB."""
    client = BitsoClient(user=user)

    # Ticker (public, no auth required)
    for book in books:
        try:
            data = client.get_ticker(book)
            mongo.save_ticker(data)
            logger.info("Ticker saved for book '%s'", book)
        except Exception as exc:
            logger.error("Failed to fetch ticker for book '%s': %s", book, exc)

    # Balance
    try:
        data = client.get_balance()
        mongo.save_balance(user, data)
        logger.info("Balance saved for user '%s'", user)
    except Exception as exc:
        logger.error("Failed to fetch balance for user '%s': %s", user, exc)

    # Open orders
    try:
        for book in books:
            data = client.get_open_orders(book=book)
            mongo.save_open_orders(user, data)
        logger.info("Open orders saved for user '%s'", user)
    except Exception as exc:
        logger.error("Failed to fetch open orders for user '%s': %s", user, exc)

    # Trade history
    try:
        for book in books:
            data = client.get_trades(book=book)
            mongo.save_trades(user, data)
        logger.info("Trades saved for user '%s'", user)
    except Exception as exc:
        logger.error("Failed to fetch trades for user '%s': %s", user, exc)


def run_sync(users: list[str], books: list[str]) -> None:
    """Top-level job called by the scheduler."""
    mongo_uri = os.environ["MONGODB_URI"]
    mongo_db = os.environ["MONGODB_DB"]

    mongo = MongoHandler(uri=mongo_uri, database=mongo_db)
    try:
        for user in users:
            logger.info("Starting sync for user '%s'", user)
            sync_user(user, books, mongo)
            logger.info("Sync complete for user '%s'", user)
    finally:
        mongo.close()


# ---------------------------------------------------------------------------
# Bootstrap
# ---------------------------------------------------------------------------

def main() -> None:
    # Load .env file when present (ignored in production containers)
    load_dotenv()

    # Required configuration
    mongo_uri = os.environ.get("MONGODB_URI")
    mongo_db = os.environ.get("MONGODB_DB")
    if not mongo_uri or not mongo_db:
        logger.error(
            "MONGODB_URI and MONGODB_DB environment variables are required."
        )
        sys.exit(1)

    # Users and books come from env vars (comma-separated)
    users_raw = os.environ.get("BITSO_USERS", "")
    books_raw = os.environ.get("BITSO_BOOKS", "btc_mxn")
    users = [u.strip() for u in users_raw.split(",") if u.strip()]
    books = [b.strip() for b in books_raw.split(",") if b.strip()]

    if not users:
        logger.error(
            "BITSO_USERS environment variable is required "
            "(comma-separated list of user identifiers)."
        )
        sys.exit(1)

    logger.info(
        "Starting Bitso client: users=%s books=%s", users, books
    )

    # Run once immediately before starting the scheduler
    run_sync(users, books)

    scheduler = build_scheduler(run_sync, users, books)
    try:
        scheduler.start()
    except (KeyboardInterrupt, SystemExit):
        logger.info("Scheduler stopped.")


if __name__ == "__main__":
    main()
