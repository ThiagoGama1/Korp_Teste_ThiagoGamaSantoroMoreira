package cliente

import (
	"testing"
	"time"
)

// Testes unitários: o disjuntor é lógica pura de estado, sem rede nem banco.
// Rodam sem o compose no ar.

func TestDisjuntorAbreDepoisDoLimite(t *testing.T) {
	d := NovoDisjuntor(3, time.Minute)

	for i := 1; i <= 2; i++ {
		d.RegistrarFalha()
		if !d.Permitir() {
			t.Fatalf("com %d falhas (limite 3) o disjuntor deveria continuar passando", i)
		}
	}

	d.RegistrarFalha()
	if d.Permitir() {
		t.Error("atingido o limite de falhas, o disjuntor deveria bloquear")
	}
}

func TestDisjuntorSucessoZeraContagem(t *testing.T) {
	d := NovoDisjuntor(3, time.Minute)

	d.RegistrarFalha()
	d.RegistrarFalha()
	d.RegistrarSucesso()

	// A contagem precisa ser de falhas *seguidas*: duas antes de um sucesso não
	// podem se somar a duas depois.
	d.RegistrarFalha()
	d.RegistrarFalha()

	if !d.Permitir() {
		t.Error("o sucesso deveria ter zerado a contagem de falhas seguidas")
	}
}

func TestDisjuntorLiberaUmaSondagem(t *testing.T) {
	// Espera curtíssima para o teste não precisar dormir de verdade.
	d := NovoDisjuntor(1, time.Millisecond)

	d.RegistrarFalha()
	if d.Permitir() {
		t.Fatal("deveria estar aberto")
	}

	time.Sleep(5 * time.Millisecond)

	if !d.Permitir() {
		t.Fatal("passado o tempo de espera, uma sondagem deveria ser liberada")
	}
	// Só UMA: enquanto a sondagem não tem desfecho, o resto continua bloqueado.
	if d.Permitir() {
		t.Error("a segunda chamada não deveria passar durante a sondagem")
	}
}

func TestDisjuntorFechaQuandoSondagemFunciona(t *testing.T) {
	d := NovoDisjuntor(1, time.Millisecond)

	d.RegistrarFalha()
	time.Sleep(5 * time.Millisecond)
	d.Permitir() // entra em meio-aberto

	d.RegistrarSucesso()

	if !d.Permitir() || !d.Permitir() {
		t.Error("após uma sondagem bem-sucedida o disjuntor deveria fechar e liberar tudo")
	}
}

// TestDisjuntorNaoTravaQuandoSondagemEhCancelada cobre a regressão: o
// cancelamento vindo do usuário não passa por RegistrarSucesso nem por
// RegistrarFalha, e meio-aberto não tem saída por tempo. Sem o AbortarSonda o
// disjuntor ficaria preso recusando tudo até o processo reiniciar.
func TestDisjuntorNaoTravaQuandoSondagemEhCancelada(t *testing.T) {
	d := NovoDisjuntor(1, 5*time.Millisecond)

	d.RegistrarFalha()
	time.Sleep(10 * time.Millisecond)

	if !d.Permitir() {
		t.Fatal("a sondagem deveria ter sido liberada")
	}

	// A sondagem morre sem resposta — usuário fechou a aba.
	d.AbortarSonda()

	time.Sleep(10 * time.Millisecond)

	if !d.Permitir() {
		t.Error("o disjuntor ficou preso em meio-aberto: nenhuma sondagem nova foi liberada")
	}
}

// AbortarSonda com o disjuntor fechado não pode ter efeito nenhum — cancelamento
// em operação normal é rotina e não deve mexer no estado.
func TestAbortarSondaNaoAfetaDisjuntorFechado(t *testing.T) {
	d := NovoDisjuntor(3, time.Minute)

	d.AbortarSonda()

	if !d.Permitir() {
		t.Error("um cancelamento com o disjuntor fechado não deveria bloquear nada")
	}
}
