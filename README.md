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
