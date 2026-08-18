package model

import "time"

// Produto é um item controlado pelo estoque.
//
// Saldo nunca deve ficar negativo. Existem duas barreiras para isso: a query de
// baixa só debita quando há saldo suficiente, e a tabela tem um CHECK como
// última defesa caso alguma escrita passe por outro caminho.
type Produto struct {
	ID         uint      `json:"id"          gorm:"primaryKey"`
	Codigo     string    `json:"codigo"      gorm:"uniqueIndex;not null"`
	Descricao  string    `json:"descricao"   gorm:"not null"`
	Saldo      int       `json:"saldo"       gorm:"not null"`
	CriadoEm   time.Time `json:"criado_em"   gorm:"autoCreateTime"`
	AlteradoEm time.Time `json:"alterado_em" gorm:"autoUpdateTime"`
}

// TableName fixa o nome da tabela. Sem isso o GORM pluraliza sozinho e o nome
// passa a depender de uma convenção implícita — melhor deixar explícito, já que
// as migrations são escritas em SQL na mão.
func (Produto) TableName() string {
	return "produtos"
}
