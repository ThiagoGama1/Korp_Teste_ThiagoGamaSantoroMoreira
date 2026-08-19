package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/config"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/database"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/middleware"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/routes"
)

const esperaDesligamento = 10 * time.Second

func main() {
	cfg, err := config.Carregar()
	if err != nil {
		log.Fatalf("estoque: %v", err)
	}

	db, err := database.Conectar(cfg.UrlBanco)
	if err != nil {
		log.Fatalf("estoque: sem conexão com o banco: %v", err)
	}

	if err := database.Migrar(db); err != nil {
		log.Fatalf("estoque: %v", err)
	}

	r := gin.Default()
	r.Use(middleware.CORS())
	routes.RegistrarHealth(r, db)
	routes.RegistrarProdutos(r, db)
	routes.RegistrarBaixas(r, db)

	srv := &http.Server{
		Addr:    ":" + cfg.PortaHTTP,
		Handler: r,
	}

	go func() {
		log.Printf("estoque ouvindo em %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("estoque: %v", err)
		}
	}()

	aguardarDesligamento(srv)
}

// aguardarDesligamento segura o programa vivo até o sistema operacional pedir
// para parar, e então dá um prazo para as requisições em andamento terminarem.
func aguardarDesligamento(srv *http.Server) {
	// Canal com buffer 1: o sinal chega num instante que não controlamos. Num
	// canal sem buffer, se ninguém estivesse lendo exatamente naquele momento, o
	// aviso seria descartado e o serviço nunca desligaria.
	sinal := make(chan os.Signal, 1)

	// A partir daqui, Ctrl+C e o SIGTERM que o `docker compose stop` envia
	// deixam de matar o processo na hora e passam a virar mensagem neste canal.
	signal.Notify(sinal, os.Interrupt, syscall.SIGTERM)

	// Ler de um canal vazio bloqueia. É esta linha que mantém o main vivo
	// enquanto o servidor atende na goroutine.
	<-sinal

	log.Printf("estoque: desligando...")

	// Prazo para terminar o que está em andamento. Background() e não o context
	// de uma requisição: aqui não existe requisição, é o programa inteiro
	// descendo.
	ctx, cancel := context.WithTimeout(context.Background(), esperaDesligamento)
	defer cancel()

	// Shutdown para de aceitar conexão nova e espera as em andamento. Devolve
	// erro se o prazo estourar antes de todas terminarem.
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("estoque: desligamento forçado: %v", err)
	}

	log.Printf("estoque: encerrado")
}
