package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/model"
)

type Produto struct {
	db *gorm.DB
}

func NovoProduto(db *gorm.DB) *Produto {
	return &Produto{db: db}
}

// DadosProduto é o que entra numa criação ou atualização. Existe separado do
// model porque o cliente não escolhe id nem timestamps — se ele mandar, é
// ignorado.
type DadosProduto struct {
	Codigo    string
	Descricao string
	Saldo     int
}

func (s *Produto) Listar() ([]model.Produto, error) {
	var produtos []model.Produto
	if err := s.db.Order("codigo").Find(&produtos).Error; err != nil {
		return nil, fmt.Errorf("service: falha ao listar produtos: %w", err)
	}
	return produtos, nil
}

func (s *Produto) Buscar(id uint) (*model.Produto, error) {
	var produto model.Produto
	err := s.db.First(&produto, id).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("service: falha ao buscar produto %d: %w", id, err)
	}
	return &produto, nil
}

func (s *Produto) Criar(dados DadosProduto) (*model.Produto, error) {
	limpos, err := validar(dados)
	if err != nil {
		return nil, err
	}

	produto := model.Produto{
		Codigo:    limpos.Codigo,
		Descricao: limpos.Descricao,
		Saldo:     limpos.Saldo,
	}

	if err := s.db.Create(&produto).Error; err != nil {
		return nil, traduzirErroDeEscrita(err)
	}
	return &produto, nil
}

func (s *Produto) Atualizar(id uint, dados DadosProduto) (*model.Produto, error) {
	limpos, err := validar(dados)
	if err != nil {
		return nil, err
	}

	produto, err := s.Buscar(id)
	if err != nil {
		return nil, err
	}

	produto.Codigo = limpos.Codigo
	produto.Descricao = limpos.Descricao
	produto.Saldo = limpos.Saldo

	if err := s.db.Save(produto).Error; err != nil {
		return nil, traduzirErroDeEscrita(err)
	}
	return produto, nil
}

func (s *Produto) Remover(id uint) error {
	resultado := s.db.Delete(&model.Produto{}, id)

	if resultado.Error != nil {
		return fmt.Errorf("service: falha ao remover produto %d: %w", id, resultado.Error)
	}
	// Delete de id inexistente não é erro no GORM: ele apaga zero linhas e
	// segue. Sem esta checagem, apagar duas vezes devolveria sucesso as duas.
	if resultado.RowsAffected == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// validar normaliza e recusa entrada inválida. Devolve os dados já limpos para
// que o chamador não use por engano a versão sem trim.
func validar(dados DadosProduto) (DadosProduto, error) {
	limpos := DadosProduto{
		Codigo:    strings.TrimSpace(dados.Codigo),
		Descricao: strings.TrimSpace(dados.Descricao),
		Saldo:     dados.Saldo,
	}

	if limpos.Codigo == "" {
		return limpos, fmt.Errorf("%w: código é obrigatório", ErrDadosInvalidos)
	}
	if limpos.Descricao == "" {
		return limpos, fmt.Errorf("%w: descrição é obrigatória", ErrDadosInvalidos)
	}
	if limpos.Saldo < 0 {
		return limpos, fmt.Errorf("%w: saldo não pode ser negativo", ErrDadosInvalidos)
	}
	return limpos, nil
}

// traduzirErroDeEscrita converte a violação de UNIQUE do banco num erro de
// domínio. Checar o banco antes de inserir não resolveria: entre a consulta e o
// insert cabe outra requisição criando o mesmo código. Quem garante a unicidade
// é o índice UNIQUE — aqui só se traduz a recusa dele.
func traduzirErroDeEscrita(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrCodigoEmUso
	}
	return fmt.Errorf("service: falha ao gravar produto: %w", err)
}
