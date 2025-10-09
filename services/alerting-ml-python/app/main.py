from __future__ import annotations

import uvicorn

from .api import create_app


app = create_app()


if __name__ == "__main__":
    uvicorn.run("app.main:app", host="0.0.0.0", port=8081, workers=1)
