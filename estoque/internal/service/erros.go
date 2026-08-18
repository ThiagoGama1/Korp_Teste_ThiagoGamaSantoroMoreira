package service

import "errors"

// Erros de domínio. O service devolve estes; o handler traduz cada um para um
// código HTTP. Nenhuma das duas camadas precisa conhecer os detalhes da outra:
// o service não sabe o que é um 409, e o handler não sabe o que é um UNIQUE.
var (
	ErrDadosInvalidos = errors.New("dados inválidos")
	ErrCodigoEmUso    = errors.New("código já cadastrado")
	ErrNaoEncontrado  = errors.New("registro não encontrado")
)
