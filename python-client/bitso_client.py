"""
Client for consuming the Bitso Go API wrapper.
Credentials are loaded from environment variables.
"""

import logging
import os

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

logger = logging.getLogger(__name__)

_DEFAULT_TIMEOUT = 10  # seconds
_DEFAULT_RETRIES = 3


def _build_session(retries: int = _DEFAULT_RETRIES) -> requests.Session:
    session = requests.Session()
    retry = Retry(
        total=retries,
        backoff_factor=0.5,
        status_forcelist=[429, 500, 502, 503, 504],
        allowed_methods=["GET"],
    )
    adapter = HTTPAdapter(max_retries=retry)
    session.mount("http://", adapter)
    session.mount("https://", adapter)
    return session


class BitsoClient:
    """HTTP client for the Go API wrapper running on port 8181."""

    def __init__(self, user: str, base_url: str | None = None):
        """
        Args:
            user: Identifier used to look up environment-variable credentials.
                  Environment variables must follow the pattern:
                  <USER_UPPER>_API_KEY and <USER_UPPER>_API_SECRET
            base_url: Base URL of the Go wrapper (defaults to GO_API_URL env
                      variable or http://localhost:8181).
        """
        self.user = user
        self.base_url = (
            base_url
            or os.environ.get("GO_API_URL", "http://localhost:8181")
        ).rstrip("/")
        self._session = _build_session()

        user_upper = user.upper()
        self.api_key = os.environ.get(f"{user_upper}_API_KEY", "")
        self.api_secret = os.environ.get(f"{user_upper}_API_SECRET", "")

        if not self.api_key or not self.api_secret:
            logger.warning(
                "Credentials for user '%s' not found in environment variables "
                "(%s_API_KEY / %s_API_SECRET). "
                "Only public endpoints (ticker) will work.",
                user,
                user_upper,
                user_upper,
            )

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _auth_headers(self, command: str) -> dict:
        """Build the headers expected by the Go wrapper."""
        if command == "ticker":
            return {"X-COMMAND": command}
        return {
            "X-API-KEY": self.api_key,
            "X-API-SECRET": self.api_secret,
            "X-USER": self.user,
            "X-COMMAND": command,
        }

    def _get(self, path: str, command: str, params: dict | None = None) -> dict:
        url = f"{self.base_url}{path}"
        headers = self._auth_headers(command)
        try:
            response = self._session.get(
                url, headers=headers, params=params, timeout=_DEFAULT_TIMEOUT
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.HTTPError as exc:
            logger.error("HTTP error for %s: %s – %s", url, exc, exc.response.text)
            raise
        except requests.exceptions.RequestException as exc:
            logger.error("Request failed for %s: %s", url, exc)
            raise

    # ------------------------------------------------------------------
    # Public endpoints
    # ------------------------------------------------------------------

    def get_ticker(self, book: str) -> dict:
        """Return the current ticker for *book* (e.g. 'btc_mxn')."""
        return self._get("/ticker", "ticker", params={"book": book})

    def get_balance(self) -> dict:
        """Return the non-zero account balances for the authenticated user."""
        return self._get("/balance", "balance")

    def get_open_orders(self, book: str | None = None) -> dict:
        """Return the open (pending) orders for the authenticated user."""
        params = {"book": book} if book else {}
        return self._get("/open_orders", "open_orders", params=params)

    def get_trades(self, book: str | None = None, limit: int = 100) -> dict:
        """Return the trade history for the authenticated user."""
        params: dict = {"limit": str(limit)}
        if book:
            params["book"] = book
        return self._get("/trades", "trades", params=params)

    def get_orders(self, book: str | None = None, limit: int = 100) -> dict:
        """Return the user trade history (alias consumed as 'orders')."""
        params: dict = {"limit": str(limit)}
        if book:
            params["book"] = book
        return self._get("/trades", "orders", params=params)
