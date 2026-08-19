-- Um produto so pode aparecer uma vez por nota: quantidades do mesmo produto sao
-- somadas na mesma linha.
--
-- Sem este indice, dois cliques simultaneos em "Adicionar" fariam as duas
-- requisicoes consultarem "nao existe" antes de qualquer uma inserir, e a nota
-- ficaria com duas linhas do mesmo produto. Com o indice, a segunda insercao
-- vira um UPDATE que soma, em vez de uma linha nova.
CREATE UNIQUE INDEX IF NOT EXISTS idx_nota_itens_nota_produto
    ON nota_itens (nota_id, produto_id);
