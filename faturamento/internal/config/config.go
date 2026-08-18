package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	PortaHTTP      string
	URLBanco       string
	URLEstoque     string
	TimeoutEstoque time.Duration
}

// Carregar lê o ambiente e valida. Só devolve erro — quem decide encerrar o
// processo é o main.
//
// A regra de quando dar valor padrão: padrão quando ele é seguro e a falha é
// barulhenta (porta, timeout); obrigatório quando um palpite errado causa dano
// silencioso (endereço de banco e do estoque, que apontariam para o lugar errado
// sem reclamar).
func Carregar() (*Config, error) {
	urlBanco := os.Getenv("DATABASE_URL")
	if urlBanco == "" {
		return nil, errors.New("config: DATABASE_URL não definida")
	}

	urlEstoque := os.Getenv("ESTOQUE_URL")
	if urlEstoque == "" {
		return nil, errors.New("config: ESTOQUE_URL não definida")
	}

	timeout, err := lerDuracao("ESTOQUE_TIMEOUT", 3*time.Second)
	if err != nil {
		return nil, err
	}

	return &Config{
		PortaHTTP:      valorOu(os.Getenv("PORT"), "7081"),
		URLBanco:       urlBanco,
		URLEstoque:     urlEstoque,
		TimeoutEstoque: timeout,
	}, nil
}

func lerDuracao(chave string, padrao time.Duration) (time.Duration, error) {
	bruto := os.Getenv(chave)
	if bruto == "" {
		return padrao, nil
	}

	duracao, err := time.ParseDuration(bruto)
	if err != nil {
		return 0, errors.New("config: " + chave + " precisa ser uma duração válida, ex: 3s")
	}
	return duracao, nil
}

func valorOu(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
