"""Альфа.Пульс: ML-сервис прогнозирования выручки (Prophet).

Контракт (ТЗ, п. 4.1):
    POST /forecast {"series":[{"ds":"2026-07-01","y":15000.5}, ...], "horizon": 14}
    → {"forecast":[{"ds":"2026-07-15","yhat":14200.0,"yhat_lower":13900.0,"yhat_upper":14500.0}, ...]}

Ошибки возвращаются HTTP-кодами: 400 — слишком короткий ряд, 422 — невалидное
тело, 500 — сбой обучения. Go-сервер при любой ошибке переключается на
fallback-модель Хольта-Винтерса.
"""

import logging
from datetime import date, timedelta

import pandas as pd
from fastapi import FastAPI, HTTPException
from prophet import Prophet
from pydantic import BaseModel, Field

# cmdstanpy заваливает stdout служебными сообщениями — глушим.
logging.getLogger("cmdstanpy").setLevel(logging.WARNING)
logging.getLogger("prophet").setLevel(logging.WARNING)

MIN_POINTS = 14  # меньше двух недель — сезонность не выучить

app = FastAPI(title="alfa-pulse-ml", version="1.0.0")


class SeriesPoint(BaseModel):
    ds: date
    y: float


class ForecastRequest(BaseModel):
    series: list[SeriesPoint]
    horizon: int = Field(default=14, ge=1, le=60)


class ForecastPoint(BaseModel):
    ds: date
    yhat: float
    yhat_lower: float
    yhat_upper: float


class ForecastResponse(BaseModel):
    forecast: list[ForecastPoint]
    model: str = "prophet"


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/forecast", response_model=ForecastResponse)
def forecast(req: ForecastRequest) -> ForecastResponse:
    if len(req.series) < MIN_POINTS:
        raise HTTPException(
            status_code=400,
            detail=f"нужно минимум {MIN_POINTS} точек, получено {len(req.series)}",
        )

    df = pd.DataFrame(
        {"ds": [p.ds for p in req.series], "y": [p.y for p in req.series]}
    )
    df["ds"] = pd.to_datetime(df["ds"])
    df = df.sort_values("ds").drop_duplicates("ds")

    try:
        model = Prophet(
            weekly_seasonality=True,   # главный паттерн дневной выручки
            yearly_seasonality="auto", # включится, если истории хватает
            daily_seasonality=False,   # внутрисуточных данных нет
            interval_width=0.8,
        )
        model.fit(df)
        future = model.make_future_dataframe(periods=req.horizon)
        prediction = model.predict(future).tail(req.horizon)
    except Exception as exc:  # noqa: BLE001 — любой сбой обучения → 500 → fallback в Go
        raise HTTPException(status_code=500, detail=f"сбой обучения модели: {exc}") from exc

    points = [
        ForecastPoint(
            ds=row.ds.date(),
            yhat=round(max(0.0, row.yhat), 2),          # выручка не бывает отрицательной
            yhat_lower=round(max(0.0, row.yhat_lower), 2),
            yhat_upper=round(max(0.0, row.yhat_upper), 2),
        )
        for row in prediction.itertuples()
    ]

    # Prophet мог «съесть» дубликаты дат — гарантируем ровно horizon точек.
    if len(points) != req.horizon:
        last_ds = df["ds"].max().date()
        points = points[: req.horizon]
        while len(points) < req.horizon:
            prev = points[-1] if points else ForecastPoint(
                ds=last_ds, yhat=0, yhat_lower=0, yhat_upper=0
            )
            points.append(
                ForecastPoint(
                    ds=prev.ds + timedelta(days=1),
                    yhat=prev.yhat,
                    yhat_lower=prev.yhat_lower,
                    yhat_upper=prev.yhat_upper,
                )
            )

    return ForecastResponse(forecast=points)
