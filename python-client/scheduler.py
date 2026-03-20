"""
APScheduler-based periodic task runner for the Bitso client.
"""

import logging
import os

from apscheduler.schedulers.blocking import BlockingScheduler
from apscheduler.triggers.interval import IntervalTrigger

logger = logging.getLogger(__name__)


def build_scheduler(job_func, users: list[str], books: list[str]) -> BlockingScheduler:
    """
    Build a BlockingScheduler that calls *job_func(users, books)* at a
    configurable interval.

    Interval is controlled by the SCHEDULER_INTERVAL_MINUTES environment
    variable (default: 5 minutes).

    Args:
        job_func: Callable with signature ``(users, books) -> None``.
        users: List of user identifiers to pass to *job_func*.
        books: List of trading-book symbols to pass to *job_func*.

    Returns:
        A configured (but not yet started) BlockingScheduler.
    """
    interval_minutes = int(os.environ.get("SCHEDULER_INTERVAL_MINUTES", "5"))

    scheduler = BlockingScheduler()
    scheduler.add_job(
        func=job_func,
        trigger=IntervalTrigger(minutes=interval_minutes),
        args=[users, books],
        id="bitso_sync",
        name="Bitso data sync",
        replace_existing=True,
    )
    logger.info(
        "Scheduler configured: every %d minute(s)", interval_minutes
    )
    return scheduler
