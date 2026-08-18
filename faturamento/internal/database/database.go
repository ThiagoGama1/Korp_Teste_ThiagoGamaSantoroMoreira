package database

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	tentativas    = 10
	esperaEntre   = 2 * time.Second
	conexoesMax   = 25
	vidaDaConexao = 5 * time.Minute
)

// Conectar insiste porque o Postgres aceita conexão TCP antes de estar pronto
// para responder query. O healthcheck do Compose cobre a subida normal; o retry
// cobre rodar fora do Docker e o banco cair depois de tudo no ar.
func Conectar(urlBanco string) (*gorm.DB, error) {
	var ultimoErro error

	for i := 1; i <= tentativas; i++ {
		db, err := tentarConectar(urlBanco)
		if err == nil {
			log.Printf("faturamento: banco conectado")
			return db, nil
		}

		ultimoErro = err
		log.Printf("faturamento: banco indisponível (tentativa %d/%d): %v", i, tentativas, err)

		if i < tentativas {
			time.Sleep(esperaEntre)
		}
	}
	return nil, ultimoErro
}

// tentarConectar faz uma tentativa e desiste. A política de insistir mora no
// Conectar — separar as duas deixa cada função com uma responsabilidade só.
func tentarConectar(urlBanco string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(urlBanco), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}

	// gorm.Open pode devolver sucesso sem ter falado com o banco: o
	// database/sql abre conexão de forma preguiçosa. O Ping é o teste real.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	// O padrão do Go é 2 conexões ociosas e nenhum teto de abertas. Com carga em
	// rajada isso derruba e reabre conexão o tempo todo, e sem teto dá para
	// estourar o limite de clientes do Postgres.
	sqlDB.SetMaxOpenConns(conexoesMax)
	sqlDB.SetMaxIdleConns(conexoesMax)
	sqlDB.SetConnMaxLifetime(vidaDaConexao)

	return db, nil
}
