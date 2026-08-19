package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/cliente"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/model"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/service"
)

// Teste de integração que atravessa a fronteira entre os dois serviços: precisa
// do compose no ar, porque é justamente o comportamento combinado deles que está
// sendo verificado.
//
//	TEST_DATABASE_URL="postgres://korp:korp@localhost:5442/faturamento_db?sslmode=disable" \
//	TEST_ESTOQUE_URL="http://localhost:7080" go test ./... -v
//
// A existência deste arquivo é uma lição em si: a suíte do estoque testa 100
// goroutines disputando saldo e passa, mas não alcança nada do lado de cá. Um
// teste que cobre só um lado da fronteira dá confiança sobre a metade errada.
const goroutines = 30

func ambienteDeTeste(t *testing.T) (*gorm.DB, *cliente.Estoque, string) {
	t.Helper()

	urlBanco := os.Getenv("TEST_DATABASE_URL")
	urlEstoque := os.Getenv("TEST_ESTOQUE_URL")
	if urlBanco == "" || urlEstoque == "" {
		t.Skip("defina TEST_DATABASE_URL e TEST_ESTOQUE_URL para rodar os testes de integração")
	}

	db, err := gorm.Open(postgres.Open(urlBanco), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("não foi possível conectar: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("não foi possível obter o pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(30)

	// Limite de falhas alto: o disjuntor não deve interferir num teste cujo
	// objetivo é a idempotência, e não a resiliência.
	disjuntor := cliente.NovoDisjuntor(1000, time.Second)

	return db, cliente.NovoEstoque(urlEstoque, 10*time.Second, disjuntor), urlEstoque
}

// criarProdutoNoEstoque usa a API HTTP porque o faturamento não tem — e não deve
// ter — acesso ao banco do estoque.
func criarProdutoNoEstoque(t *testing.T, urlEstoque string, saldo int) uint {
	t.Helper()

	corpo, _ := json.Marshal(map[string]any{
		"codigo":    fmt.Sprintf("TESTE-FAT-%d", time.Now().UnixNano()),
		"descricao": "Produto de teste do faturamento",
		"saldo":     saldo,
	})

	resp, err := http.Post(urlEstoque+"/produtos", "application/json", bytes.NewReader(corpo))
	if err != nil {
		t.Fatalf("não foi possível criar o produto: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("o estoque respondeu %d ao criar o produto", resp.StatusCode)
	}

	var produto struct {
		ID uint `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&produto); err != nil {
		t.Fatalf("resposta ilegível do estoque: %v", err)
	}
	return produto.ID
}

func saldoNoEstoque(t *testing.T, urlEstoque string, produtoID uint) int {
	t.Helper()

	resp, err := http.Get(fmt.Sprintf("%s/produtos/%d", urlEstoque, produtoID))
	if err != nil {
		t.Fatalf("não foi possível ler o produto: %v", err)
	}
	defer resp.Body.Close()

	var produto struct {
		Saldo int `json:"saldo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&produto); err != nil {
		t.Fatalf("resposta ilegível do estoque: %v", err)
	}
	return produto.Saldo
}

func criarNotaComItem(t *testing.T, svc *service.Nota, produtoID uint, quantidade int) *model.Nota {
	t.Helper()

	nota, err := svc.Criar()
	if err != nil {
		t.Fatalf("não foi possível criar a nota: %v", err)
	}
	if _, err := svc.AdicionarItem(context.Background(), nota.ID, produtoID, quantidade); err != nil {
		t.Fatalf("não foi possível adicionar o item: %v", err)
	}
	return nota
}

// TestImprimirConcorrenteDebitaUmaVez é o teste que faltava.
//
// Muitas requisições de impressão da MESMA nota, ao mesmo tempo — o duplo clique
// levado ao extremo. A garantia é que o estoque seja debitado uma única vez.
//
// Antes da correção este teste falhava: garantirChave lia "sem chave", cada
// goroutine gerava uma chave aleatória diferente, e o estoque via N pedidos
// distintos em vez de N repetições do mesmo.
func TestImprimirConcorrenteDebitaUmaVez(t *testing.T) {
	db, estoque, urlEstoque := ambienteDeTeste(t)
	svc := service.NovaNota(db, estoque)

	const saldoInicial = 500
	const quantidade = 3

	produtoID := criarProdutoNoEstoque(t, urlEstoque, saldoInicial)
	nota := criarNotaComItem(t, svc, produtoID, quantidade)

	var sucessos, falhas int64
	var wg sync.WaitGroup
	partida := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-partida

			if _, err := svc.Imprimir(context.Background(), nota.ID); err != nil {
				// Só a primeira fecha a nota; as demais podem chegar depois e
				// receber "nota não está aberta". Isso é correto — o que não pode
				// é o saldo sair duas vezes.
				atomic.AddInt64(&falhas, 1)
				return
			}
			atomic.AddInt64(&sucessos, 1)
		}()
	}

	close(partida)
	wg.Wait()

	if sucessos == 0 {
		t.Fatalf("nenhuma impressão teve sucesso (%d falhas)", falhas)
	}

	esperado := saldoInicial - quantidade
	if saldo := saldoNoEstoque(t, urlEstoque, produtoID); saldo != esperado {
		t.Errorf("esperava saldo %d (uma única baixa de %d), ficou %d — o estoque foi debitado %d vezes",
			esperado, quantidade, saldo, (saldoInicial-saldo)/quantidade)
	}

	var final model.Nota
	if err := db.First(&final, nota.ID).Error; err != nil {
		t.Fatalf("não foi possível reler a nota: %v", err)
	}
	if final.Status != model.StatusFechada {
		t.Errorf("a nota deveria ter fechado, está %s", final.Status)
	}
}

// TestNotaRecusadaPodeSerImpressaDepois cobre o segundo achado: uma nota recusada
// por saldo não pode ficar travada para sempre.
//
// Quando o estoque recusa, nada é debitado — então a chave é descartada e a
// próxima tentativa vale como pedido novo, relendo o saldo atual. Sem isso, a
// nota replayaria o 409 guardado mesmo depois de o estoque ser reposto.
func TestNotaRecusadaPodeSerImpressaDepois(t *testing.T) {
	db, estoque, urlEstoque := ambienteDeTeste(t)
	svc := service.NovaNota(db, estoque)

	produtoID := criarProdutoNoEstoque(t, urlEstoque, 1)
	nota := criarNotaComItem(t, svc, produtoID, 5) // pede mais do que existe

	if _, err := svc.Imprimir(context.Background(), nota.ID); err == nil {
		t.Fatal("esperava recusa por saldo insuficiente")
	}

	// A chave precisa ter sido descartada: sem isso a próxima tentativa reenviaria
	// a mesma e receberia o mesmo 409, independente do saldo.
	var apos model.Nota
	if err := db.First(&apos, nota.ID).Error; err != nil {
		t.Fatalf("não foi possível reler a nota: %v", err)
	}
	if apos.IdempotencyKey != nil {
		t.Errorf("a chave deveria ter sido descartada após a recusa, está %q", *apos.IdempotencyKey)
	}

	// Reduz o pedido para caber no saldo e tenta de novo.
	var item model.NotaItem
	if err := db.Where("nota_id = ?", nota.ID).First(&item).Error; err != nil {
		t.Fatalf("não foi possível ler o item: %v", err)
	}
	if _, err := svc.RemoverItem(nota.ID, item.ID); err != nil {
		t.Fatalf("não foi possível remover o item: %v", err)
	}
	if _, err := svc.AdicionarItem(context.Background(), nota.ID, produtoID, 1); err != nil {
		t.Fatalf("não foi possível readicionar o item: %v", err)
	}

	if _, err := svc.Imprimir(context.Background(), nota.ID); err != nil {
		t.Fatalf("a nota deveria imprimir depois de reduzida, veio: %v", err)
	}

	if saldo := saldoNoEstoque(t, urlEstoque, produtoID); saldo != 0 {
		t.Errorf("esperava saldo 0 após a impressão, ficou %d", saldo)
	}
}
