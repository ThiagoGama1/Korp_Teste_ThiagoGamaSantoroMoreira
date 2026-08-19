-- Registro de idempotencia das baixas de saldo.
--
-- A coluna chave e UNIQUE, e isso nao e organizacao: e a garantia contra a
-- corrida. Se duas requisicoes com a mesma chave entrarem ao mesmo tempo, as
-- duas debitam e as duas tentam inserir aqui; a segunda viola o UNIQUE, sua
-- transacao sofre rollback, e o debito dela e desfeito junto.
CREATE TABLE IF NOT EXISTS baixas (
    id        BIGSERIAL   PRIMARY KEY,
    chave     TEXT        NOT NULL UNIQUE,

    -- RECUSADA tambem e gravada: um retry por saldo insuficiente precisa
    -- receber o MESMO 409, nao tentar debitar de novo. Operacao repetida
    -- devolve o mesmo resultado — inclusive quando o resultado e "nao".
    status    TEXT        NOT NULL CHECK (status IN ('CONFIRMADA', 'RECUSADA')),

    -- O corpo inteiro devolvido na primeira vez. Guardado pronto em vez de
    -- remontado a partir de colunas: o formato muda entre sucesso e recusa, e
    -- remontar e onde entraria a chance de a resposta divergir.
    --
    -- JSON e nao JSONB de proposito: JSONB normaliza o documento (reordena as
    -- chaves e reinsere espacos), entao o texto que sai nao e o que entrou. JSONB
    -- vale quando se consulta dentro do documento; aqui ele so e guardado e
    -- devolvido, e o que importa e a resposta sair identica a primeira.
    resposta  JSON        NOT NULL,

    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
