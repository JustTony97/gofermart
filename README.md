# Gophermart

HTTP-сервис накопительной системы лояльности. Позволяет пользователям регистрироваться, загружать номера заказов для начисления баллов, отслеживать статус их обработки и списывать накопленные баллы в счёт оплаты новых покупок.

## Как работает система

1. Пользователь регистрируется (`POST /api/user/register`) или входит в систему (`POST /api/user/login`) и получает JWT-токен.
2. Пользователь отправляет номер совершённого заказа (`POST /api/user/orders`). Сервис валидирует номер по алгоритму Луна и сохраняет его в PostgreSQL со статусом `NEW`.
3. Фоновый воркер регистрации забирает заказы в статусе `NEW`, отправляет их во внешний сервис расчёта начислений **Accrual** и переводит в статус `PROCESSING`.
4. Фоновый воркер опроса проверяет статус заказов `PROCESSING` в **Accrual** (`GET /api/orders/{number}`):
   - при статусе `PROCESSED` — обновляет статус заказа в БД и атомарно начисляет баллы на баланс пользователя;
   - при статусе `INVALID` (или отсутствии заказа в Accrual) — переводит заказ в статус `INVALID`.
5. Накопленные баллы можно списать в счёт оплаты нового заказа (`POST /api/user/balance/withdraw`).

## Архитектура

Сервис работает в одном процессе и состоит из HTTP API и двух фоновых воркеров:

- **HTTP API** — регистрация, авторизация, приём заказов (`NEW`), баланс и списания.
- **OrderPoller** — берёт новые заказы (`NEW`), отправляет их в Accrual и переводит в `PROCESSING`.
- **AccrualPoller** — опрашивает Accrual по заказам в `PROCESSING` и начисляет баллы (`PROCESSED` / `INVALID`).

```mermaid
flowchart LR
    Client([Клиент]) <--> API[HTTP API]
    API <--> DB[(PostgreSQL)]
    W1[OrderPoller] <--> DB
    W1 --> Accrual([Accrual])
    W2[AccrualPoller] <--> DB
    W2 <--> Accrual
```

## API-эндпоинты

| Метод  | Эндпоинт                     | Авторизация | Описание                                        |
| :----- | :--------------------------- | :---------: | :---------------------------------------------- |
| `POST` | `/api/user/register`         |     Нет     | Регистрация нового пользователя и выдача токена |
| `POST` | `/api/user/login`            |     Нет     | Аутентификация пользователя и выдача токена     |
| `POST` | `/api/user/orders`           |     Да      | Загрузка номера заказа для расчёта начислений   |
| `GET`  | `/api/user/orders`           |     Да      | Список загруженных заказов (от новых к старым)  |
| `GET`  | `/api/user/balance`          |     Да      | Текущий баланс и общая сумма списанных баллов   |
| `POST` | `/api/user/balance/withdraw` |     Да      | Списание баллов в счёт оплаты нового заказа     |
| `GET`  | `/api/user/withdrawals`      |     Да      | История списаний баллов пользователя            |

## Настройка базы данных

1. Запустите PostgreSQL:

```bash
docker run --name gophermart-pg \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 -d postgres:16
```

2. Примените миграции (используется [golang-migrate](https://github.com/golang-migrate/migrate.git)):

```bash
migrate -database "postgresql://postgres:password@localhost:5432/postgres?sslmode=disable" -path ./migrations up
```

## Сборка и запуск

1. Запустите внешний сервис расчёта начислений **Accrual**:

```bash
./cmd/accrual/accrual_darwin_arm64 -a :8080
```

2. Соберите и запустите **Gophermart**:

```bash
go build -o gophermart cmd/gophermart/main.go
./gophermart
```

Для запуска без предварительной сборки:

```bash
go run cmd/gophermart/main.go
```

## Переменные окружения и флаги

Переменные окружения имеют приоритет над флагами командной строки.

| Переменная окружения      | Флаг          | По умолчанию                                                             | Описание                                |
| :------------------------ | :------------ | :----------------------------------------------------------------------- | :-------------------------------------- |
| `SERVER_ADDRESS`          | `-a`          | `:8000`                                                                  | Адрес запуска HTTP-сервера              |
| `DATABASE_DSN`            | `-d`          | `postgresql://postgres:password@localhost:5432/postgres?sslmode=disable` | Строка подключения к PostgreSQL         |
| `JWT_SECRET`              | `-jwt-secret` | `supersecretkey`                                                         | Секретный ключ для подписи JWT          |
| `JWT_EXP`                 | `-jwt-exp`    | `3h`                                                                     | Время жизни JWT-токена                  |
| `ORDER_REGISTER_INTERVAL` | `-r`          | `1s`                                                                     | Интервал регистрации новых заказов      |
| `ORDER_REGISTER_WORKERS`  | `-rw`         | `3`                                                                      | Количество воркеров регистрации заказов |
| `ACCRUAL_POLL_INTERVAL`   | `-p`          | `5s`                                                                     | Интервал опроса сервиса Accrual         |
| `ACCRUAL_POLL_WORKERS`    | `-pw`         | `3`                                                                      | Количество воркеров опроса Accrual      |
| `ACCRUAL_BASE_URL`        | `-ac`         | `http://localhost:8080`                                                  | Адрес внешнего сервиса Accrual          |
