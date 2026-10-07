Asi es, solo un Wrapper mas para el API de Bitso.

Estoy aprendiendo Golang y me interesa tener un servicio siempre disponible en Docker para obtener respuestas JSON para implementarlas en algunos proyectos que pienso realizar
- Un bot para compras automaticas de monedas en Bitso (mejora de comisiones y tipo de cambio respecto a usar la App o la Web) este sería en Python y mongoDB
- usar JS para un desarrollo web, graficas y algunos datos historicos

A la fecha solo he integrado los endpoints siguientes del Bitso API:
- /ticker (sin `book` devuelve todos los libros)
- /orders
- /balance
- /user_trades
- /open_orders
- /fundings (depósitos)
- /withdrawals (retiros)
- /saldo y /saldos: saldo total y por moneda valuado en MXN (ver abajo)
- /health: estado del servicio, sin token ni llamada a Bitso (`{"status":"ok","credenciales":"ok"}`)

Variables de entorno: ver `.env.example`.


## Sync a MongoDB (`cmd/sync`)

Copia de Bitso a la base `defi` (mismo esquema que el antiguo `datos_b.py`):

| Colección | Fuente | Clave |
|---|---|---|
| `trades` | `/user_trades` | user, exchange, tid |
| `orders` | trades agrupados por `oid` (`status: executed`) | user, exchange, oid |
| `abonos` | `/fundings` (SPEI, cripto y Earnings) | user, exchange, fid |
| `retiros` | `/withdrawals` | user, exchange, wid |
| `balance` | `/balance` + `/ticker` + `/fees`, solo el día 15 y el último del mes | user, coin, fecha |

`fecha` se guarda en hora de `TZ` (America/Mexico_City) y `created_at` como fecha UTC.

```
go run ./cmd/sync -once      # una corrida
go run ./cmd/sync -full      # recorre todo el historial
go run ./cmd/sync -balance   # fuerza el snapshot de saldos
docker compose up -d --build bitso-sync   # servicio diario a SYNC_TIME
```

## Saldos en MXN (`/saldo`, `/saldos`)

Calculados en vivo: bid × (1 − comisión del libro) × saldo; monedas sin libro en MXN se valúan vía USD.
Si no se mandan `X-API-KEY`/`X-API-SECRET` se usa la key de `.env`; con `WRAPPER_TOKEN` definido hay
que mandar `X-Wrapper-Token`. La respuesta se cachea 60 s.

```
GET /saldo   -> {"success":true,"fecha":"...","total_mxn":12361.97}
GET /saldos  -> {"success":true,"fecha":"...","total_mxn":12361.97,
                 "saldos":[{"coin":"sol","saldo":0.57627225,"disponible":0.57627225,
                            "precio_mxn":2143.24,"saldo_mxn":1225.46}, ...]}
```

`/saldo/historial?desde=AAAA-MM-DD` suma por fecha los snapshots de la colección `balance`
(día 15 y fin de mes; `neto` de cada moneda, `total` del MXN). Mismo token que `/saldo`.

```
GET /saldo/historial?desde=2025-11-01
    -> {"success":true,"data":[{"fecha":"2026-09-15","total_mxn":11980.4}, ...]}
```


## Wallets fuera de Bitso (`/wallets/{wallet}`)

Cantidades capturadas a mano por wallet (`cold`, `mp`, …; colección `wallets`), valuadas
con los tickers públicos de Bitso al bid, sin comisión. Por moneda (ver `bitso.WalletBooks`):
BTC con `btc_mxn`, SOL con `sol_mxn`, XRP con `xrp_usd` × `usd_mxn`; cualquier otra con
`<coin>_mxn` o `<coin>_usd` (el USD sale de `usd_mxn`).
El PUT exige `X-Wrapper-Token` siempre; sin `WRAPPER_TOKEN` en `.env` no se puede escribir.

```
PUT /wallets/cold  {"btc":0.05,"sol":12,"xrp":350}   # fija esas monedas; 0 la quita
GET /wallets/cold  -> {"success":true,"fecha":"...","total_mxn":...,
                       "saldos":[{"coin":"btc","saldo":0.05,"libro":"btc_mxn","precio_mxn":...,"saldo_mxn":...}]}
GET /wallets/cold/historial?desde=AAAA-MM-DD   # snapshots de wallets_balance
```

`bitso-sync` guarda la valuación de cada wallet en `wallets_balance` los mismos días que el
snapshot de saldos (15 y fin de mes, o con `-balance`).
