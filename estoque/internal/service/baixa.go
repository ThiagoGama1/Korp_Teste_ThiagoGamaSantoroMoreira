package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/model"
)

// ErrSaldoInsuficiente é devolvido junto com um corpo JSON já pronto, listando
// quais produtos faltaram. O handler repassa esse corpo como está.
var ErrSaldoInsuficiente = errors.New("saldo insuficiente")

type Baixa struct {
	db *gorm.DB
}

func NovaBaixa(db *gorm.DB) *Baixa {
	return &Baixa{db: db}
}

type ItemBaixa struct {
	ProdutoID  uint `json:"produto_id"`
	Quantidade int  `json:"quantidade"`
}

type itemBaixado struct {
	ProdutoID     uint   `json:"produto_id"`
	ProdutoCodigo string `json:"produto_codigo"`
	SaldoAnterior int    `json:"saldo_anterior"`
	SaldoAtual    int    `json:"saldo_atual"`
}

type faltaDeSaldo struct {
	ProdutoID     uint   `json:"produto_id"`
	ProdutoCodigo string `json:"produto_codigo"`
	Solicitado    int    `json:"solicitado"`
	Disponivel    int    `json:"disponivel"`
}

// Executar debita o saldo dos produtos — todos ou nenhum.
//
// Devolve o corpo JSON já serializado em vez de uma struct, porque a resposta
// precisa ser guardada e reenviada byte a byte idêntica numa repetição da mesma
// chave. Remontar a partir de campos abriria espaço para divergir.
//
// O erro acompanha o corpo: ErrSaldoInsuficiente vem com o JSON da recusa, para
// o handler devolver 409 com os detalhes.
func (s *Baixa) Executar(ctx context.Context, chave string, itens []ItemBaixa) ([]byte, error) {
	if err := validarItens(chave, itens); err != nil {
		return nil, err
	}

	// Ordenar por id antes de tocar no banco evita impasse: duas requisições com
	// os mesmos produtos em ordem inversa travariam uma na linha que a outra
	// segura, e ficariam presas até o Postgres matar uma delas.
	sort.Slice(itens, func(a, b int) bool { return itens[a].ProdutoID < itens[b].ProdutoID })

	if corpo, err, encontrada := s.buscarRegistro(chave); encontrada {
		return corpo, err
	}

	corpo, errDeNegocio, err := s.processar(ctx, chave, itens)

	// Corrida: outra requisição com a mesma chave gravou primeiro, e o UNIQUE
	// derrubou esta transação — o débito daqui foi desfeito junto. A resposta
	// correta é a que o vencedor gravou.
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		corpoGravado, errGravado, encontrada := s.buscarRegistro(chave)
		if encontrada {
			return corpoGravado, errGravado
		}
	}
	if err != nil {
		return nil, err
	}

	return corpo, errDeNegocio
}

// processar roda a transação inteira. Devolve três coisas: o corpo da resposta,
// o erro de negócio (saldo insuficiente, que não é falha) e o erro técnico.
//
// Separar os dois últimos importa: erro de negócio ainda faz commit, porque a
// recusa precisa ficar gravada; erro técnico derruba tudo.
func (s *Baixa) processar(ctx context.Context, chave string, itens []ItemBaixa) ([]byte, error, error) {
	var corpo []byte
	var errDeNegocio error

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		produtos, err := travarProdutos(tx, itens)
		if err != nil {
			return err
		}

		faltas := conferirSaldos(itens, produtos)
		if len(faltas) > 0 {
			corpo, errDeNegocio, err = registrarRecusa(tx, chave, faltas)
			return err
		}

		corpo, err = debitar(tx, chave, itens, produtos)
		return err
	})

	return corpo, errDeNegocio, err
}

// travarProdutos lê as linhas com FOR UPDATE, o que as bloqueia para qualquer
// outra transação até esta terminar.
//
// É o que fecha a janela entre conferir o saldo e debitá-lo: sem o lock, duas
// requisições poderiam ler "tem 1 disponível" antes de qualquer uma gravar, e as
// duas debitariam o mesmo item.
func travarProdutos(tx *gorm.DB, itens []ItemBaixa) (map[uint]model.Produto, error) {
	ids := make([]uint, 0, len(itens))
	for _, item := range itens {
		ids = append(ids, item.ProdutoID)
	}

	var produtos []model.Produto
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&produtos).Error
	if err != nil {
		return nil, fmt.Errorf("service: falha ao travar produtos: %w", err)
	}

	porID := make(map[uint]model.Produto, len(produtos))
	for _, produto := range produtos {
		porID[produto.ID] = produto
	}
	return porID, nil
}

// conferirSaldos devolve tudo o que falta, não só o primeiro problema. Quem está
// na tela prefere corrigir os três itens de uma vez a descobrir um por vez.
func conferirSaldos(itens []ItemBaixa, produtos map[uint]model.Produto) []faltaDeSaldo {
	var faltas []faltaDeSaldo

	for _, item := range itens {
		produto, existe := produtos[item.ProdutoID]
		if !existe {
			faltas = append(faltas, faltaDeSaldo{
				ProdutoID:  item.ProdutoID,
				Solicitado: item.Quantidade,
				Disponivel: 0,
			})
			continue
		}
		if produto.Saldo < item.Quantidade {
			faltas = append(faltas, faltaDeSaldo{
				ProdutoID:     produto.ID,
				ProdutoCodigo: produto.Codigo,
				Solicitado:    item.Quantidade,
				Disponivel:    produto.Saldo,
			})
		}
	}
	return faltas
}

func debitar(tx *gorm.DB, chave string, itens []ItemBaixa, produtos map[uint]model.Produto) ([]byte, error) {
	baixados := make([]itemBaixado, 0, len(itens))

	for _, item := range itens {
		produto := produtos[item.ProdutoID]

		// A condição saldo >= ? é redundante com o FOR UPDATE acima, e fica de
		// propósito: se algum caminho futuro debitar sem travar antes, o banco
		// ainda recusa em vez de deixar o saldo negativo.
		resultado := tx.Model(&model.Produto{}).
			Where("id = ? AND saldo >= ?", item.ProdutoID, item.Quantidade).
			UpdateColumn("saldo", gorm.Expr("saldo - ?", item.Quantidade))

		if resultado.Error != nil {
			return nil, fmt.Errorf("service: falha ao debitar o produto %d: %w", item.ProdutoID, resultado.Error)
		}
		// RowsAffected == 0 significa que a condição não bateu. Não é erro do
		// GORM — no SQL, "não alterou nada" é sucesso.
		if resultado.RowsAffected == 0 {
			return nil, fmt.Errorf("service: saldo do produto %d mudou durante a baixa", item.ProdutoID)
		}

		baixados = append(baixados, itemBaixado{
			ProdutoID:     produto.ID,
			ProdutoCodigo: produto.Codigo,
			SaldoAnterior: produto.Saldo,
			SaldoAtual:    produto.Saldo - item.Quantidade,
		})
	}

	corpo, err := json.Marshal(map[string]any{
		"status": model.BaixaConfirmada,
		"itens":  baixados,
	})
	if err != nil {
		return nil, fmt.Errorf("service: falha ao montar a resposta da baixa: %w", err)
	}

	if err := gravarRegistro(tx, chave, model.BaixaConfirmada, corpo); err != nil {
		return nil, err
	}
	return corpo, nil
}

// registrarRecusa grava a recusa e devolve commit, não rollback.
//
// Nada foi debitado — o FOR UPDATE só leu. Então a transação pode confirmar, e
// precisa: é o registro da recusa que faz um retry com a mesma chave receber o
// mesmo 409 em vez de tentar debitar de novo.
func registrarRecusa(tx *gorm.DB, chave string, faltas []faltaDeSaldo) ([]byte, error, error) {
	corpo, err := json.Marshal(map[string]any{
		"erro": map[string]any{
			"codigo":   "SALDO_INSUFICIENTE",
			"mensagem": fmt.Sprintf("Saldo insuficiente para %d produto(s)", len(faltas)),
			"detalhes": faltas,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("service: falha ao montar a recusa: %w", err)
	}

	if err := gravarRegistro(tx, chave, model.BaixaRecusada, corpo); err != nil {
		return nil, nil, err
	}
	return corpo, ErrSaldoInsuficiente, nil
}

// gravarRegistro insere a chave dentro da mesma transação do débito.
//
// Se fosse gravada depois do commit, existiria um instante com o saldo já
// baixado e a chave ainda inexistente — e um retry nesse instante debitaria de
// novo, que é exatamente o que a idempotência existe para impedir.
func gravarRegistro(tx *gorm.DB, chave, status string, corpo []byte) error {
	registro := model.Baixa{Chave: chave, Status: status, Resposta: corpo}

	if err := tx.Create(&registro).Error; err != nil {
		// Propagado cru: quem chamou precisa reconhecer o erro de chave
		// duplicada para saber que houve corrida.
		return err
	}
	return nil
}

// buscarRegistro é a porta de entrada da idempotência: chave conhecida devolve o
// que foi gravado, sem tocar em saldo nenhum.
func (s *Baixa) buscarRegistro(chave string) ([]byte, error, bool) {
	var registro model.Baixa
	err := s.db.Where("chave = ?", chave).First(&registro).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, false
	}
	if err != nil {
		return nil, fmt.Errorf("service: falha ao consultar a chave de idempotência: %w", err), true
	}

	if registro.Status == model.BaixaRecusada {
		return registro.Resposta, ErrSaldoInsuficiente, true
	}
	return registro.Resposta, nil, true
}

func validarItens(chave string, itens []ItemBaixa) error {
	if chave == "" {
		return fmt.Errorf("%w: o header Idempotency-Key é obrigatório", ErrDadosInvalidos)
	}
	if len(itens) == 0 {
		return fmt.Errorf("%w: informe ao menos um item", ErrDadosInvalidos)
	}

	for _, item := range itens {
		if item.ProdutoID == 0 {
			return fmt.Errorf("%w: produto_id é obrigatório", ErrDadosInvalidos)
		}
		if item.Quantidade <= 0 {
			return fmt.Errorf("%w: a quantidade precisa ser maior que zero", ErrDadosInvalidos)
		}
	}
	return nil
}
