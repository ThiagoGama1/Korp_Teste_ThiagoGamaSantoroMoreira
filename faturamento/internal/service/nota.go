package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/cliente"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/model"
)

type Nota struct {
	db      *gorm.DB
	estoque *cliente.Estoque
}

func NovaNota(db *gorm.DB, estoque *cliente.Estoque) *Nota {
	return &Nota{db: db, estoque: estoque}
}

func (s *Nota) Listar() ([]model.Nota, error) {
	var notas []model.Nota
	if err := s.db.Preload("Itens").Order("numero DESC").Find(&notas).Error; err != nil {
		return nil, fmt.Errorf("service: falha ao listar notas: %w", err)
	}
	return notas, nil
}

func (s *Nota) Buscar(id uint) (*model.Nota, error) {
	var nota model.Nota
	err := s.db.Preload("Itens").First(&nota, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("service: falha ao buscar nota %d: %w", id, err)
	}
	return &nota, nil
}

// Criar abre uma nota vazia. O número vem de uma SEQUENCE do Postgres: ela é
// atômica, então duas criações simultâneas recebem números diferentes sem
// disputar nada. Um MAX(numero)+1 leria o mesmo valor nas duas.
func (s *Nota) Criar() (*model.Nota, error) {
	var numero int
	if err := s.db.Raw("SELECT nextval('notas_numero_seq')").Scan(&numero).Error; err != nil {
		return nil, fmt.Errorf("service: falha ao gerar número da nota: %w", err)
	}

	nota := model.Nota{
		Numero: numero,
		Status: model.StatusAberta,
		Itens:  []model.NotaItem{},
	}

	if err := s.db.Create(&nota).Error; err != nil {
		return nil, fmt.Errorf("service: falha ao criar nota: %w", err)
	}
	return &nota, nil
}

// AdicionarItem consulta o estoque para copiar código e descrição do produto.
// Os bancos são separados: sem essa cópia não haveria como exibir a nota sem
// chamar o estoque a cada leitura.
//
// Se o produto já estiver na nota, a quantidade é somada em vez de criar uma
// segunda linha. Isso mantém um item por produto, o que simplifica a baixa.
func (s *Nota) AdicionarItem(ctx context.Context, notaID, produtoID uint, quantidade int) (*model.Nota, error) {
	if quantidade <= 0 {
		return nil, fmt.Errorf("%w: quantidade precisa ser maior que zero", ErrDadosInvalidos)
	}

	nota, err := s.exigirNotaAberta(notaID)
	if err != nil {
		return nil, err
	}

	produto, err := s.estoque.BuscarProduto(ctx, produtoID)
	if err != nil {
		return nil, err
	}

	if err := s.gravarItem(nota.ID, produto, quantidade); err != nil {
		return nil, err
	}
	return s.Buscar(notaID)
}

func (s *Nota) RemoverItem(notaID, itemID uint) (*model.Nota, error) {
	if _, err := s.exigirNotaAberta(notaID); err != nil {
		return nil, err
	}

	resultado := s.db.Where("nota_id = ?", notaID).Delete(&model.NotaItem{}, itemID)
	if resultado.Error != nil {
		return nil, fmt.Errorf("service: falha ao remover item %d: %w", itemID, resultado.Error)
	}
	// Delete de id inexistente não é erro no GORM: apaga zero linhas e retorna
	// sucesso. O filtro por nota_id também impede remover item de outra nota.
	if resultado.RowsAffected == 0 {
		return nil, ErrNaoEncontrado
	}
	return s.Buscar(notaID)
}

// exigirNotaAberta é a guarda usada por toda operação de escrita. Concentrar a
// regra aqui evita que cada método decida sozinho o que "pode alterar" significa.
func (s *Nota) exigirNotaAberta(notaID uint) (*model.Nota, error) {
	nota, err := s.Buscar(notaID)
	if err != nil {
		return nil, err
	}
	if !nota.EstaAberta() {
		return nil, ErrNotaNaoAberta
	}
	return nota, nil
}

func (s *Nota) gravarItem(notaID uint, produto *cliente.Produto, quantidade int) error {
	var existente model.NotaItem
	err := s.db.Where("nota_id = ? AND produto_id = ?", notaID, produto.ID).First(&existente).Error

	if err == nil {
		existente.Quantidade += quantidade
		return s.db.Save(&existente).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("service: falha ao consultar item: %w", err)
	}

	item := model.NotaItem{
		NotaID:           notaID,
		ProdutoID:        produto.ID,
		ProdutoCodigo:    produto.Codigo,
		ProdutoDescricao: produto.Descricao,
		Quantidade:       quantidade,
	}
	return s.db.Create(&item).Error
}

// Imprimir é o coração do teste: fecha a nota no faturamento e debita o saldo no
// estoque. São dois bancos em dois serviços, então não existe transação cobrindo
// os dois. A consistência vem da idempotência, não de compensação.
//
// A ordem importa:
//
//  1. valida (aberta, com itens)
//  2. gera e GRAVA a chave antes de qualquer chamada de rede
//  3. debita no estoque com essa chave
//  4. só então fecha a nota
//
// Se o estoque estiver fora no passo 3, nada foi debitado e a nota continua
// ABERTA: o usuário tenta de novo. Se o débito passar e o passo 4 falhar, a nota
// fica ABERTA com a chave gravada — uma nova impressão manda a MESMA chave, o
// estoque devolve o resultado guardado sem redebitar, e o fechamento se completa.
func (s *Nota) Imprimir(ctx context.Context, notaID uint) (*model.Nota, error) {
	nota, err := s.Buscar(notaID)
	if err != nil {
		return nil, err
	}
	if !nota.EstaAberta() {
		return nil, ErrNotaNaoAberta
	}
	if len(nota.Itens) == 0 {
		return nil, ErrNotaVazia
	}

	chave, err := s.garantirChave(nota)
	if err != nil {
		return nil, err
	}

	// Chamada feita mesmo quando BaixaConfirmada já é true: com a mesma chave o
	// estoque devolve o resultado anterior sem debitar. Um caminho só, em vez de
	// dois com estados diferentes.
	if _, err := s.estoque.Baixar(ctx, chave, itensParaBaixa(nota)); err != nil {
		return nil, err
	}

	if err := s.fechar(nota.ID); err != nil {
		return nil, err
	}
	return s.Buscar(notaID)
}

// garantirChave grava a chave no banco ANTES de o estoque ser chamado.
//
// Se ela fosse gerada na hora da chamada, cada nova tentativa criaria uma chave
// diferente, o estoque trataria como pedidos distintos e debitaria de novo — que
// é exatamente o bug que a idempotência existe para impedir. A chave é da nota,
// não da tentativa.
func (s *Nota) garantirChave(nota *model.Nota) (string, error) {
	if nota.IdempotencyKey != nil && *nota.IdempotencyKey != "" {
		return *nota.IdempotencyKey, nil
	}

	chave, err := gerarChave(nota.ID)
	if err != nil {
		return "", err
	}

	if err := s.db.Model(&model.Nota{}).Where("id = ?", nota.ID).
		Update("idempotency_key", chave).Error; err != nil {
		return "", fmt.Errorf("service: falha ao gravar a chave de idempotência: %w", err)
	}

	nota.IdempotencyKey = &chave
	return chave, nil
}

// fechar usa Updates com colunas explícitas em vez de Save(nota): Save tentaria
// gravar também os itens carregados pelo Preload, e nesse ponto eles não mudaram.
func (s *Nota) fechar(notaID uint) error {
	err := s.db.Model(&model.Nota{}).Where("id = ?", notaID).Updates(map[string]any{
		"status":           model.StatusFechada,
		"baixa_confirmada": true,
	}).Error
	if err != nil {
		return fmt.Errorf("service: falha ao fechar a nota %d: %w", notaID, err)
	}
	return nil
}

func itensParaBaixa(nota *model.Nota) []cliente.ItemBaixa {
	itens := make([]cliente.ItemBaixa, 0, len(nota.Itens))
	for _, item := range nota.Itens {
		itens = append(itens, cliente.ItemBaixa{
			ProdutoID:  item.ProdutoID,
			Quantidade: item.Quantidade,
		})
	}
	return itens
}

// gerarChave usa crypto/rand em vez de math/rand: o valor precisa ser único
// entre processos e reinícios, e math/rand sem semente repete a mesma sequência
// a cada boot.
func gerarChave(notaID uint) (string, error) {
	aleatorio := make([]byte, 8)
	if _, err := rand.Read(aleatorio); err != nil {
		return "", fmt.Errorf("service: falha ao gerar a chave de idempotência: %w", err)
	}
	return fmt.Sprintf("nota-%d-%s", notaID, hex.EncodeToString(aleatorio)), nil
}
