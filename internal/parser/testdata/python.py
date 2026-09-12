from typing import Any


class Client:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url


async def fetch(url: str, timeout: int = 30) -> bytes:
    return b""


def render(name: str, options: dict = {}, tags: set = {1, 2}) -> str:
    return name


def _private(x: int) -> int:
    return x
