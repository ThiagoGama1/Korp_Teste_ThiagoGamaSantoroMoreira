-- Roda uma única vez, na primeira criação do volume do Postgres.
-- Se precisar recriar:  docker compose down -v && docker compose up

CREATE DATABASE estoque_db;
CREATE DATABASE faturamento_db;
