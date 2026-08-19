package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	PortaHTTP      string
	URLBanco       string
	URLEstoque     string
	TimeoutEstoque time.Duration

	DisjuntorFalhas int
	DisjuntorEspera time.Duration
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

	espera, err := lerDuracao("DISJUNTOR_ESPERA", 10*time.Second)
	if err != nil {
		return nil, err
	}

	return &Config{
		PortaHTTP:       valorOu(os.Getenv("PORT"), "7081"),
		URLBanco:        urlBanco,
		URLEstoque:      urlEstoque,
		TimeoutEstoque:  timeout,
		DisjuntorFalhas: lerInteiro("DISJUNTOR_FALHAS", 5),
		DisjuntorEspera: espera,
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

// lerInteiro não devolve erro: um valor inválido aqui cai no padrão em vez de
// impedir o serviço de subir. Diferente do endereço do banco, um limite de
// falhas errado não causa dano silencioso.
func lerInteiro(chave string, padrao int) int {
	valor, err := strconv.Atoi(os.Getenv(chave))
	if err != nil || valor <= 0 {
		return padrao
	}
	return valor
}

func valorOu(v, padrao string) string {
	if v == "" {
		return padrao
	}
	return v
}
