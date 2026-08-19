package cliente

import (
	"log"
	"sync"
	"time"
)

// Disjuntor evita martelar um serviço que já está fora.
//
// Sem ele, cada usuário espera o timeout inteiro (3s) para receber erro, e o
// serviço caído continua recebendo carga enquanto tenta se recuperar. Com ele,
// depois de N falhas seguidas as chamadas falham na hora, e passado um tempo
// uma única tentativa é liberada para testar se o serviço voltou.
//
// É a diferença entre "deu erro" e "o sistema detectou a queda, se protegeu e
// se recuperou sozinho" — que é o que o requisito 2 pede.
type Disjuntor struct {
	// Mutex e não RWMutex: Permitir() também escreve (muda o estado quando o
	// tempo de espera vence), então não existe caminho só de leitura.
	mu sync.Mutex

	estado          estado
	falhasSeguidas  int
	limiteDeFalhas  int
	esperaAteTentar time.Duration
	reabreEm        time.Time
}

type estado int

const (
	fechado    estado = iota // operação normal, tudo passa
	aberto                   // serviço fora, nada passa
	meioAberto               // uma tentativa liberada para sondar
)

func NovoDisjuntor(limiteDeFalhas int, esperaAteTentar time.Duration) *Disjuntor {
	return &Disjuntor{
		estado:          fechado,
		limiteDeFalhas:  limiteDeFalhas,
		esperaAteTentar: esperaAteTentar,
	}
}

// Permitir diz se a chamada pode sair. Em meioAberto devolve true uma única vez:
// a partir daí bloqueia de novo até saber o resultado daquela sondagem.
func (d *Disjuntor) Permitir() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.estado == fechado {
		return true
	}
	if d.estado == aberto && time.Now().After(d.reabreEm) {
		d.estado = meioAberto
		log.Printf("faturamento: disjuntor meio-aberto, sondando o estoque")
		return true
	}
	return false
}

func (d *Disjuntor) RegistrarSucesso() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.estado != fechado {
		log.Printf("faturamento: disjuntor fechado, estoque respondendo de novo")
	}
	d.estado = fechado
	d.falhasSeguidas = 0
}

func (d *Disjuntor) RegistrarFalha() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.falhasSeguidas++

	// A sondagem falhou: volta a bloquear e recomeça a contagem do tempo.
	if d.estado == meioAberto {
		d.abrir()
		return
	}
	if d.estado == fechado && d.falhasSeguidas >= d.limiteDeFalhas {
		d.abrir()
	}
}

// AbortarSonda desfaz uma sondagem que não chegou a ter resposta.
//
// Existe porque meioAberto é um estado sem saída por tempo: Permitir() só
// transita de fechado e de aberto, então quem entra em meioAberto só sai por
// RegistrarSucesso ou RegistrarFalha. Se a única tentativa liberada terminar sem
// nenhum dos dois — caso do cancelamento vindo do próprio usuário — o disjuntor
// ficaria preso recusando tudo, com o estoque saudável, até o processo reiniciar.
//
// Não conta como falha: a sondagem não foi respondida nem recusada, apenas não
// aconteceu. O disjuntor volta a aberto com prazo novo e tenta de novo depois.
func (d *Disjuntor) AbortarSonda() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.estado != meioAberto {
		return
	}

	d.estado = aberto
	d.reabreEm = time.Now().Add(d.esperaAteTentar)
	log.Printf("faturamento: sondagem cancelada antes da resposta, nova tentativa em %s", d.esperaAteTentar)
}

// abrir assume que o mutex já está travado por quem chamou.
func (d *Disjuntor) abrir() {
	d.estado = aberto
	d.reabreEm = time.Now().Add(d.esperaAteTentar)
	log.Printf("faturamento: disjuntor ABERTO após %d falhas, nova tentativa em %s",
		d.falhasSeguidas, d.esperaAteTentar)
}
