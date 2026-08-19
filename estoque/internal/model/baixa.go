package model

import "time"

const (
	BaixaConfirmada = "CONFIRMADA"
	BaixaRecusada   = "RECUSADA"
)

// Baixa e o registro de idempotencia: guarda que uma chave ja foi processada e
// qual resposta foi devolvida na ocasiao.
type Baixa struct {
	ID     uint   `json:"id"     gorm:"primaryKey"`
	Chave  string `json:"chave"  gorm:"uniqueIndex;not null"`
	Status string `json:"status" gorm:"not null"`

	// []byte com type:jsonb em vez de um tipo de gorm.io/datatypes: evita uma
	// dependencia inteira para guardar bytes que ja chegam em JSON.
	Resposta []byte `json:"resposta" gorm:"type:json;not null"`

	CriadoEm time.Time `json:"criado_em" gorm:"autoCreateTime"`
}

func (Baixa) TableName() string {
	return "baixas"
}
