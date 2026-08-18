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
)
