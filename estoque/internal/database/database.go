package database

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func tentarConectar(URLBanco string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(URLBanco), &gorm.Config{TranslateError: true})

	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	err = sqlDB.Ping()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
func Conectar(URLBanco string) (*gorm.DB, error) {
	var ultimoErro error
	for i := 1; i <= 10; i++ {
		db, err := tentarConectar(URLBanco)
		if err == nil {
			return db, nil
		}
		ultimoErro = err
		log.Printf("estoque: banco indisponível (tentativa %d/10): %v", i, err)
		if i < 10 {
			time.Sleep(2 * time.Second)
		}
	}
	return nil, ultimoErro
}
