CREATE TABLE IF NOT EXISTS produtos (
    id          BIGSERIAL   PRIMARY KEY,
    codigo      TEXT        NOT NULL UNIQUE,
    descricao   TEXT        NOT NULL,
    saldo       INTEGER     NOT NULL DEFAULT 0 CHECK (saldo >= 0),
    criado_em   TIMESTAMPTZ NOT NULL DEFAULT now(),
    alterado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
