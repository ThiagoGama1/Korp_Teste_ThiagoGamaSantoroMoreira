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

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/cliente"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/config"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/database"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/middleware"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/routes"
)

const esperaDesligamento = 10 * time.Second

func main() {
	// log.Fatal é o certo aqui: os pacotes internos devolvem erro para quem
	// chamou decidir, e o main é quem tem autoridade para encerrar. Sem config,
	// sem banco ou sem migration, não existe serviço para subir.
	cfg, err := config.Carregar()
	if err != nil {
		log.Fatalf("faturamento: %v", err)
	}

	db, err := database.Conectar(cfg.URLBanco)
	if err != nil {
		log.Fatalf("faturamento: sem conexão com o banco: %v", err)
	}

	if err := database.Migrar(db); err != nil {
		log.Fatalf("faturamento: %v", err)
	}

	disjuntor := cliente.NovoDisjuntor(cfg.DisjuntorFalhas, cfg.DisjuntorEspera)
	estoque := cliente.NovoEstoque(cfg.URLEstoque, cfg.TimeoutEstoque, disjuntor)

	r := gin.Default()
	r.Use(middleware.CORS())
	routes.RegistrarHealth(r, db)
	routes.RegistrarNotas(r, db, estoque)

	// http.Server explícito em vez de r.Run(): o atalho do Gin não devolve o
	// servidor, e sem ele não existe Shutdown().
	srv := &http.Server{
		Addr:    ":" + cfg.PortaHTTP,
		Handler: r,
	}

	// ListenAndServe bloqueia para sempre. Numa goroutine, o main segue e fica
	// parado esperando o sinal logo abaixo.
	go func() {
		log.Printf("faturamento ouvindo em %s (estoque em %s)", srv.Addr, cfg.URLEstoque)

		// Shutdown faz o ListenAndServe retornar ErrServerClosed. Isso não é
		// falha, é o aviso de "parei porque mandaram parar" — tratar como erro
		// mataria o processo no ato e cortaria as requisições em andamento.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("faturamento: %v", err)
		}
	}()

	aguardarDesligamento(srv)
}

// aguardarDesligamento bloqueia até o Ctrl+C ou o SIGTERM que o `docker compose
// stop` envia, e então dá um prazo para as requisições em andamento terminarem.
//
// Isso importa para a demonstração: derrubar o estoque no meio de uma baixa não
// pode cortar a operação pela metade.
func aguardarDesligamento(srv *http.Server) {
	sinal := make(chan os.Signal, 1)
	signal.Notify(sinal, os.Interrupt, syscall.SIGTERM)
	<-sinal

	log.Printf("faturamento: desligando...")

	ctx, cancel := context.WithTimeout(context.Background(), esperaDesligamento)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("faturamento: desligamento forçado: %v", err)
	}
	log.Printf("faturamento: encerrado")
}
