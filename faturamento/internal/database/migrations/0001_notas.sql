-- A numeração das notas vem de uma SEQUENCE do Postgres, não de um COUNT ou
-- MAX(numero)+1. A sequence é atômica: duas requisições simultâneas recebem
-- números diferentes sem disputar nada. Com MAX+1, as duas leriam o mesmo valor.
CREATE SEQUENCE IF NOT EXISTS notas_numero_seq START 1;

CREATE TABLE IF NOT EXISTS notas (
    id               BIGSERIAL   PRIMARY KEY,
    numero           INTEGER     NOT NULL UNIQUE,
    status           TEXT        NOT NULL CHECK (status IN ('ABERTA', 'FECHADA')),
    idempotency_key  TEXT        UNIQUE,
    baixa_confirmada BOOLEAN     NOT NULL DEFAULT false,
    criado_em        TIMESTAMPTZ NOT NULL DEFAULT now(),
    alterado_em      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- produto_id não tem FOREIGN KEY: o produto vive no banco do outro serviço e
-- não há como o Postgres validar isso. produto_codigo e produto_descricao são
-- cópias tiradas no momento em que o item entrou na nota — sem elas seria
-- preciso chamar o estoque só para exibir a nota. É também o comportamento
-- correto no domínio: a nota registra o que foi vendido naquele momento, mesmo
-- que o cadastro do produto mude depois.
CREATE TABLE IF NOT EXISTS nota_itens (
    id                BIGSERIAL PRIMARY KEY,
    nota_id           BIGINT    NOT NULL REFERENCES notas(id) ON DELETE CASCADE,
    produto_id        BIGINT    NOT NULL,
    produto_codigo    TEXT      NOT NULL,
    produto_descricao TEXT      NOT NULL,
    quantidade        INTEGER   NOT NULL CHECK (quantidade > 0)
);

CREATE INDEX IF NOT EXISTS idx_nota_itens_nota_id ON nota_itens (nota_id);
