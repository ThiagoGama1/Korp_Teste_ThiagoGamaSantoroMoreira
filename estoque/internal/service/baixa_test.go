package service_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/model"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/service"
)

// Estes são testes de integração: rodam contra um Postgres de verdade, porque é
// exatamente o comportamento do banco sob concorrência que está sendo verificado.
// Um banco falso em memória não teria FOR UPDATE nem isolamento de transação, e
// o teste passaria sem provar nada.
//
// Rodar com o compose no ar:
//
//	TEST_DATABASE_URL="postgres://korp:korp@localhost:5442/estoque_db?sslmode=disable" go test ./... -v

const goroutines = 100

func abrirBanco(t *testing.T) *gorm.DB {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida — pulando os testes de integração")
	}

	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		TranslateError: true,
		// Sem log de query: 100 goroutines gerariam milhares de linhas e
		// esconderiam o resultado do teste.
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("não foi possível conectar: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("não foi possível obter o pool: %v", err)
	}
	// O pool precisa comportar a rajada. Com o padrão de 2 conexões ociosas, o
	// teste mediria a lentidão de abrir conexão em vez da contenção no banco.
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(30)

	return db
}

// criarProduto insere um produto com código único e agenda a limpeza, para os
// testes poderem rodar repetidas vezes sem sujar o banco.
func criarProduto(t *testing.T, db *gorm.DB, saldo int) model.Produto {
	t.Helper()

	produto := model.Produto{
		Codigo:    fmt.Sprintf("TESTE-%d", time.Now().UnixNano()),
		Descricao: "Produto de teste",
		Saldo:     saldo,
	}
	if err := db.Create(&produto).Error; err != nil {
		t.Fatalf("não foi possível criar o produto: %v", err)
	}

	t.Cleanup(func() {
		db.Where("chave LIKE ?", "teste-"+produto.Codigo+"%").Delete(&model.Baixa{})
		db.Delete(&model.Produto{}, produto.ID)
	})

	return produto
}

func saldoAtual(t *testing.T, db *gorm.DB, id uint) int {
	t.Helper()

	var produto model.Produto
	if err := db.First(&produto, id).Error; err != nil {
		t.Fatalf("não foi possível reler o produto: %v", err)
	}
	return produto.Saldo
}

// TestBaixaConcorrencia é o cenário do requisito opcional (a): um produto com
// saldo 1 disputado por muitas notas ao mesmo tempo.
//
// Cada goroutine usa uma CHAVE DIFERENTE de propósito — são pedidos legítimos e
// distintos brigando pelo mesmo saldo. Com a mesma chave, a idempotência
// responderia por todas e o teste não provaria nada sobre concorrência.
func TestBaixaConcorrencia(t *testing.T) {
	db := abrirBanco(t)
	produto := criarProduto(t, db, 1)
	svc := service.NovaBaixa(db)

	var sucessos, recusas, outros int64
	var wg sync.WaitGroup

	// O barrier faz as 100 goroutines partirem juntas. Sem ele, cada uma sairia
	// conforme fosse criada e a disputa seria bem menos real.
	partida := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func(n int) {
			defer wg.Done()
			<-partida

			chave := fmt.Sprintf("teste-%s-%d", produto.Codigo, n)
			itens := []service.ItemBaixa{{ProdutoID: produto.ID, Quantidade: 1}}

			_, err := svc.Executar(context.Background(), chave, itens)

			switch {
			case err == nil:
				atomic.AddInt64(&sucessos, 1)
			case errors.Is(err, service.ErrSaldoInsuficiente):
				atomic.AddInt64(&recusas, 1)
			default:
				atomic.AddInt64(&outros, 1)
				t.Errorf("erro inesperado: %v", err)
			}
		}(i)
	}

	close(partida)
	// Wait segura o teste até a última goroutine terminar. Sem isso, a função
	// retornaria e as verificações rodariam com o trabalho pela metade.
	wg.Wait()

	if sucessos != 1 {
		t.Errorf("esperava exatamente 1 baixa confirmada, houve %d", sucessos)
	}
	if recusas != goroutines-1 {
		t.Errorf("esperava %d recusas, houve %d", goroutines-1, recusas)
	}
	if outros != 0 {
		t.Errorf("houve %d erros inesperados", outros)
	}
	if saldo := saldoAtual(t, db, produto.ID); saldo != 0 {
		t.Errorf("esperava saldo 0, ficou %d — o produto foi vendido mais de uma vez", saldo)
	}
}

// TestBaixaIdempotencia é o cenário do requisito opcional (c): a MESMA operação
// repetida muitas vezes.
//
// Aqui todas as goroutines usam a mesma chave. O saldo é folgado de propósito —
// se a idempotência falhar, o problema aparece como saldo debitado várias vezes,
// e não disfarçado de "acabou o estoque".
func TestBaixaIdempotencia(t *testing.T) {
	db := abrirBanco(t)
	produto := criarProduto(t, db, 100)
	svc := service.NovaBaixa(db)

	const quantidade = 2
	chave := "teste-" + produto.Codigo + "-unica"

	var wg sync.WaitGroup
	partida := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-partida

			itens := []service.ItemBaixa{{ProdutoID: produto.ID, Quantidade: quantidade}}
			if _, err := svc.Executar(context.Background(), chave, itens); err != nil {
				t.Errorf("erro inesperado: %v", err)
			}
		}()
	}

	close(partida)
	wg.Wait()

	esperado := 100 - quantidade
	if saldo := saldoAtual(t, db, produto.ID); saldo != esperado {
		t.Errorf("esperava saldo %d (uma única baixa de %d), ficou %d — a operação foi aplicada mais de uma vez",
			esperado, quantidade, saldo)
	}
}

// TestBaixaRecusaEhIdempotente verifica que a recusa também é gravada: repetir
// uma chave que falhou por saldo devolve o mesmo erro, sem tentar debitar de novo.
func TestBaixaRecusaEhIdempotente(t *testing.T) {
	db := abrirBanco(t)
	produto := criarProduto(t, db, 1)
	svc := service.NovaBaixa(db)

	chave := "teste-" + produto.Codigo + "-recusa"
	itens := []service.ItemBaixa{{ProdutoID: produto.ID, Quantidade: 5}}

	primeiro, err := svc.Executar(context.Background(), chave, itens)
	if !errors.Is(err, service.ErrSaldoInsuficiente) {
		t.Fatalf("esperava saldo insuficiente, veio: %v", err)
	}

	segundo, err := svc.Executar(context.Background(), chave, itens)
	if !errors.Is(err, service.ErrSaldoInsuficiente) {
		t.Fatalf("na repetição esperava o mesmo erro, veio: %v", err)
	}

	// Byte a byte: é o corpo guardado sendo devolvido, não um novo montado.
	if string(primeiro) != string(segundo) {
		t.Errorf("as duas respostas deveriam ser idênticas:\n1: %s\n2: %s", primeiro, segundo)
	}
	if saldo := saldoAtual(t, db, produto.ID); saldo != 1 {
		t.Errorf("o saldo não deveria ter mudado numa recusa, ficou %d", saldo)
	}
}
