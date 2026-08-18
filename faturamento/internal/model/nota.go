package model

import "time"

const (
	StatusAberta  = "ABERTA"
	StatusFechada = "FECHADA"
)

// Nota nasce ABERTA e é um rascunho: dá para incluir e remover itens. Imprimir
// é o ato de oficializar — ela vira FECHADA e o saldo dos produtos é debitado.
// Depois disso nada mais muda.
type Nota struct {
	ID     uint   `json:"id"     gorm:"primaryKey"`
	Numero int    `json:"numero" gorm:"not null;uniqueIndex"`
	Status string `json:"status" gorm:"not null"`

	// Gerada e gravada antes de chamar o estoque, e reaproveitada em toda nova
	// tentativa de impressão da mesma nota. É o que impede o saldo de ser
	// debitado duas vezes num clique duplo ou numa retentativa.
	IdempotencyKey *string `json:"-" gorm:"uniqueIndex"`

	// Marca que o estoque confirmou a baixa. Nota ABERTA com esta flag ligada
	// significa que o débito passou e o fechamento não — dá para concluir
	// reenviando a mesma chave.
	BaixaConfirmada bool `json:"baixa_confirmada" gorm:"not null;default:false"`

	CriadoEm   time.Time `json:"criado_em"   gorm:"autoCreateTime"`
	AlteradoEm time.Time `json:"alterado_em" gorm:"autoUpdateTime"`

	Itens []NotaItem `json:"itens" gorm:"foreignKey:NotaID"`
}

func (Nota) TableName() string {
	return "notas"
}

func (n *Nota) EstaAberta() bool {
	return n.Status == StatusAberta
}

// NotaItem guarda cópia do código e da descrição do produto. Os bancos são
// separados, então não existe JOIN possível com a tabela de produtos.
type NotaItem struct {
	ID               uint   `json:"id"                gorm:"primaryKey"`
	NotaID           uint   `json:"nota_id"           gorm:"not null;index"`
	ProdutoID        uint   `json:"produto_id"        gorm:"not null"`
	ProdutoCodigo    string `json:"produto_codigo"    gorm:"not null"`
	ProdutoDescricao string `json:"produto_descricao" gorm:"not null"`
	Quantidade       int    `json:"quantidade"        gorm:"not null"`
}

func (NotaItem) TableName() string {
	return "nota_itens"
}
