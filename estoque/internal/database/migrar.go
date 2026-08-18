package database

import (
	"embed"
	"fmt"
	"log"
	"sort"

	"gorm.io/gorm"
)

// Os arquivos .sql são embutidos no binário em tempo de compilação. Assim o
// container não precisa carregar a pasta migrations junto — o Dockerfile copia
// só o executável.
//
//go:embed migrations/*.sql
var arquivosMigracao embed.FS

// Migrar executa os arquivos de migrations/ em ordem alfabética. Todo script é
// escrito para poder rodar duas vezes sem erro (IF NOT EXISTS), então não há
// controle de versão aplicada — o que é suficiente para o tamanho deste projeto
// e evitaria dor de cabeça só num sistema com muitas migrations.
func Migrar(db *gorm.DB) error {
	entradas, err := arquivosMigracao.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("database: não foi possível ler as migrations: %w", err)
	}

	nomes := make([]string, 0, len(entradas))
	for _, entrada := range entradas {
		nomes = append(nomes, entrada.Name())
	}
	sort.Strings(nomes)

	for _, nome := range nomes {
		if err := executarMigracao(db, nome); err != nil {
			return err
		}
	}

	log.Printf("estoque: %d migration(s) aplicada(s)", len(nomes))
	return nil
}

func executarMigracao(db *gorm.DB, nome string) error {
	conteudo, err := arquivosMigracao.ReadFile("migrations/" + nome)
	if err != nil {
		return fmt.Errorf("database: falha ao ler %s: %w", nome, err)
	}

	if err := db.Exec(string(conteudo)).Error; err != nil {
		return fmt.Errorf("database: falha ao aplicar %s: %w", nome, err)
	}
	return nil
}
