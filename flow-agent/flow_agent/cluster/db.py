from __future__ import annotations

import os
from pathlib import Path
from typing import Any

from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession, async_sessionmaker, create_async_engine


class ControlPlane:
    """Small PostgreSQL control-plane wrapper.

    The engine is created lazily so importing the API does not attempt a network
    connection. Startup calls ``connect`` and ``initialize`` explicitly.
    """

    def __init__(self, url: str | None = None) -> None:
        self.url = (url or os.getenv("DATABASE_URL", "")).strip()
        self.engine: AsyncEngine | None = None
        self.sessions: async_sessionmaker[AsyncSession] | None = None

    async def connect(self) -> None:
        if not self.url:
            raise RuntimeError("DATABASE_URL is required for the Gateway control plane")
        if not self.url.startswith("postgresql+asyncpg://"):
            raise RuntimeError("DATABASE_URL must use postgresql+asyncpg://")
        self.engine = create_async_engine(self.url, pool_pre_ping=True, pool_size=10, max_overflow=20)
        self.sessions = async_sessionmaker(self.engine, expire_on_commit=False)
        async with self.engine.connect() as connection:
            await connection.execute(text("SELECT 1"))

    async def initialize(self) -> None:
        if self.engine is None:
            await self.connect()
        schema = Path(__file__).with_name("schema.sql").read_text(encoding="utf-8")
        # PostgreSQL accepts multiple statements through a driver connection only
        # when executed as one text query. This schema contains no user input.
        async with self.engine.begin() as connection:  # type: ignore[union-attr]
            for statement in schema.split(";"):
                statement = statement.strip()
                if statement:
                    await connection.execute(text(statement))

    async def close(self) -> None:
        if self.engine is not None:
            await self.engine.dispose()
            self.engine = None
            self.sessions = None

    async def execute(self, statement: str, **params: Any) -> None:
        if self.sessions is None:
            raise RuntimeError("control plane is not connected")
        async with self.sessions.begin() as session:
            await session.execute(text(statement), params)
