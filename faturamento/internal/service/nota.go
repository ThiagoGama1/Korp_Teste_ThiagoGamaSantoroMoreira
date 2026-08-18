package service

import (
	"context"
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
