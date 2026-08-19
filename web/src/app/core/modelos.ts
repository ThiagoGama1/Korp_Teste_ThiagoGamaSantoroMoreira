// Espelham o JSON que os dois serviços devolvem. Manter os nomes idênticos aos
// campos da API evita uma camada de tradução que não agrega nada aqui.

export interface Produto {
  id: number;
  codigo: string;
  descricao: string;
  saldo: number;
  criado_em: string;
  alterado_em: string;
}

export interface NotaItem {
  id: number;
  nota_id: number;
  produto_id: number;
  produto_codigo: string;
  produto_descricao: string;
  quantidade: number;
}

export type StatusNota = 'ABERTA' | 'FECHADA';

export interface Nota {
  id: number;
  numero: number;
  status: StatusNota;
  baixa_confirmada: boolean;
  criado_em: string;
  alterado_em: string;
  itens: NotaItem[];
}

/** Formato de erro que os dois serviços usam, definido no ARQUITETURA.md. */
export interface RespostaErro {
  erro: {
    codigo: string;
    mensagem: string;
    detalhes?: unknown;
  };
}
