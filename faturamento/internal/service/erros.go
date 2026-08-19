package service

import "errors"

// Erros de domínio. O service devolve estes; o handler traduz cada um num código
// HTTP. O service não sabe o que é um 409, e o handler não sabe o que é uma
// transação.
var (
	ErrDadosInvalidos = errors.New("dados inválidos")
	ErrNaoEncontrado  = errors.New("registro não encontrado")
	ErrNotaNaoAberta  = errors.New("a nota não está aberta")
	ErrNotaVazia      = errors.New("a nota não tem itens")

	// ErrBaixaPendente: o estoque já debitou, mas a nota não chegou a fechar.
	// Alterar itens nesse estado faria o estoque replayar a confirmação antiga
	// na próxima impressão, e a nota fecharia com produtos que nunca saíram.
	ErrBaixaPendente = errors.New("a baixa desta nota já foi confirmada")
)
