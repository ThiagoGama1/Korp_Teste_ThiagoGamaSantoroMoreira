package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// Remover apaga uma nota inteira. Passa pela mesma guarda das outras escritas:
// nota fechada nao se apaga, e nota cuja baixa ja foi confirmada tambem nao.
//
// A segunda parte e a que importa. Uma nota com baixa confirmada ja tirou saldo
// do estoque; apagar ela deixaria o saldo debitado sem nenhum documento que
// justifique o debito, e nao existe como devolver — o estoque e outro servico,
// com outro banco, e a baixa la e definitiva.
//
// Os itens somem junto pelo ON DELETE CASCADE declarado na migration.
func (s *Nota) Remover(notaID uint) error {
	if _, err := s.exigirNotaAberta(notaID); err != nil {
		return err
	}

	resultado := s.db.Delete(&model.Nota{}, notaID)
	if resultado.Error != nil {
		return fmt.Errorf("service: falha ao remover a nota %d: %w", notaID, resultado.Error)
	}
	// Delete de id inexistente nao e erro no GORM: apaga zero linhas e devolve
	// sucesso. Sem esta checagem, apagar duas vezes daria certo as duas.
	if resultado.RowsAffected == 0 {
		return ErrNaoEncontrado
	}
	return nil
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
	// Status ABERTA nao basta: se o estoque ja debitou e o fechamento falhou, a
	// nota fica aberta com a baixa confirmada. Deixar incluir item aqui faria o
	// estoque replayar a confirmacao antiga na proxima impressao, e a nota
	// fecharia com itens que nunca sairam do estoque.
	if nota.BaixaConfirmada {
		return nil, ErrBaixaPendente
	}
	return nota, nil
}

// gravarItem soma a quantidade quando o produto ja esta na nota.
//
// Feito com ON CONFLICT em vez de consultar-e-decidir: a consulta seguida de
// insercao tem uma janela entre os dois passos, e dois cliques simultaneos
// passariam os dois pelo "nao existe". O ON CONFLICT delega a decisao ao indice
// unico, que e avaliado dentro da propria insercao.
//
// A soma usa nota_itens.quantidade + ? e nao um valor calculado em Go, para o
// incremento partir do valor que estiver no banco na hora da escrita.
func (s *Nota) gravarItem(notaID uint, produto *cliente.Produto, quantidade int) error {
	item := model.NotaItem{
		NotaID:           notaID,
		ProdutoID:        produto.ID,
		ProdutoCodigo:    produto.Codigo,
		ProdutoDescricao: produto.Descricao,
		Quantidade:       quantidade,
	}

	err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "nota_id"}, {Name: "produto_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"quantidade": gorm.Expr("nota_itens.quantidade + ?", quantidade),
		}),
	}).Create(&item).Error

	if err != nil {
		return fmt.Errorf("service: falha ao gravar o item: %w", err)
	}
	return nil
}

// Imprimir é o coração do teste: fecha a nota no faturamento e debita o saldo no
// estoque. São dois bancos em dois serviços, então não existe transação cobrindo
// os dois. A consistência vem da idempotência, não de compensação.
//
// A ordem importa:
//
//  1. valida (aberta, sem baixa pendente, com itens)
//  2. gera e GRAVA a chave antes de qualquer chamada de rede
//  3. debita no estoque com essa chave
//  4. marca a baixa como confirmada
//  5. só então fecha a nota
//
// Os passos 4 e 5 são gravações separadas de propósito. Se fossem uma só, o
// estado intermediário — débito feito, nota ainda aberta — não existiria, e não
// haveria como distinguir "o estoque nunca foi chamado" de "o estoque debitou e
// o fechamento falhou". São situações com tratamentos opostos.
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
		s.descartarChaveSeNadaFoiDebitado(nota, err)
		return nil, err
	}

	if err := s.marcarBaixaConfirmada(nota.ID); err != nil {
		return nil, err
	}
	if err := s.fechar(nota.ID); err != nil {
		return nil, err
	}
	return s.Buscar(notaID)
}

// descartarChaveSeNadaFoiDebitado apaga a chave quando o estoque RECUSOU o
// pedido.
//
// Recusa por saldo significa que nada foi debitado — e o estoque guardou esse
// "não" associado à chave. Se ela ficasse gravada na nota, toda nova tentativa
// reenviaria a mesma chave e receberia o mesmo 409 de volta, mesmo depois de o
// estoque ser reposto ou a nota ser reduzida: a nota ficaria travada para sempre.
//
// Apagando, a próxima impressão gera chave nova e vale como um pedido novo — que
// é o correto, porque não há efeito anterior a proteger.
//
// Já um erro de indisponibilidade mantém a chave: aí não se sabe se o estoque
// chegou a debitar antes de a conexão cair, e reenviar a mesma chave é
// justamente o que evita o débito em dobro.
func (s *Nota) descartarChaveSeNadaFoiDebitado(nota *model.Nota, errDaBaixa error) {
	if !errors.Is(errDaBaixa, cliente.ErrSaldoInsuficiente) {
		return
	}

	// O WHERE inclui a própria chave, e não só o id.
	//
	// Sem esse predicado, um descarte atrasado apagaria uma chave nova gravada
	// por outra tentativa no intervalo — e a impressão seguinte geraria uma
	// terceira chave, fazendo o estoque debitar de novo. É a mesma guarda que o
	// garantirChave usa, na direção oposta.
	if err := s.db.Model(&model.Nota{}).
		Where("id = ? AND idempotency_key = ?", nota.ID, *nota.IdempotencyKey).
		Update("idempotency_key", nil).Error; err != nil {
		// Não propaga: o erro que interessa ao usuário é o da recusa. A chave
		// órfã só custa uma nova tentativa dando o mesmo 409.
		log.Printf("faturamento: falha ao descartar a chave da nota %d: %v", nota.ID, err)
		return
	}
	nota.IdempotencyKey = nil
}

// garantirChave grava a chave no banco ANTES de o estoque ser chamado.
//
// Se ela fosse gerada na hora da chamada, cada nova tentativa criaria uma chave
// diferente, o estoque trataria como pedidos distintos e debitaria de novo — que
// é exatamente o bug que a idempotência existe para impedir. A chave é da nota,
// não da tentativa.
//
// A gravação usa `WHERE idempotency_key IS NULL` e confere RowsAffected em vez de
// um UPDATE direto. Sem essa condição, dois cliques simultâneos passariam os dois
// pela verificação de nil, gerariam chaves aleatórias DIFERENTES, e o estoque
// veria dois pedidos distintos — debitando duas vezes. A condição no UPDATE torna
// "verificar e gravar" um único passo indivisível: só uma das requisições altera
// a linha, e a outra descobre isso pelo RowsAffected zerado.
func (s *Nota) garantirChave(nota *model.Nota) (string, error) {
	if nota.IdempotencyKey != nil && *nota.IdempotencyKey != "" {
		return *nota.IdempotencyKey, nil
	}

	chave, err := gerarChave(nota.ID)
	if err != nil {
		return "", err
	}

	resultado := s.db.Model(&model.Nota{}).
		Where("id = ? AND idempotency_key IS NULL", nota.ID).
		Update("idempotency_key", chave)

	if resultado.Error != nil {
		return "", fmt.Errorf("service: falha ao gravar a chave de idempotência: %w", resultado.Error)
	}

	// Zero linhas: outra requisição gravou a chave entre a leitura e agora. A
	// chave válida é a dela — a desta tentativa é descartada.
	if resultado.RowsAffected == 0 {
		return s.relerChave(nota)
	}

	nota.IdempotencyKey = &chave
	return chave, nil
}

func (s *Nota) relerChave(nota *model.Nota) (string, error) {
	var atual model.Nota
	if err := s.db.Select("idempotency_key").First(&atual, nota.ID).Error; err != nil {
		return "", fmt.Errorf("service: falha ao reler a chave da nota %d: %w", nota.ID, err)
	}
	if atual.IdempotencyKey == nil || *atual.IdempotencyKey == "" {
		return "", fmt.Errorf("service: a chave da nota %d sumiu durante a impressão", nota.ID)
	}

	nota.IdempotencyKey = atual.IdempotencyKey
	return *atual.IdempotencyKey, nil
}

// marcarBaixaConfirmada registra que o estoque já debitou, antes de a nota ser
// fechada. É esse registro que impede a nota de voltar a ser editável num
// intervalo em que o saldo já saiu.
func (s *Nota) marcarBaixaConfirmada(notaID uint) error {
	err := s.db.Model(&model.Nota{}).Where("id = ?", notaID).
		Update("baixa_confirmada", true).Error
	if err != nil {
		return fmt.Errorf("service: falha ao confirmar a baixa da nota %d: %w", notaID, err)
	}
	return nil
}

// fechar usa Updates com colunas explícitas em vez de Save(nota): Save tentaria
// gravar também os itens carregados pelo Preload, e nesse ponto eles não mudaram.
func (s *Nota) fechar(notaID uint) error {
	err := s.db.Model(&model.Nota{}).Where("id = ?", notaID).
		Update("status", model.StatusFechada).Error
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
