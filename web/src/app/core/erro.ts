import { HttpErrorResponse } from '@angular/common/http';
import { RespostaErro } from './modelos';

/**
 * Traduz a falha HTTP na frase que o usuário vê.
 *
 * A distinção que importa é entre "o sistema recusou" e "o sistema não
 * respondeu". O backend devolve 409 no primeiro caso e 503 no segundo, com
 * mensagens já prontas em português — então aqui basta repassá-las.
 *
 * `status === 0` é o caso em que nem houve resposta: o serviço está fora ou o
 * navegador bloqueou. Aí a mensagem é nossa, porque não existe corpo para ler.
 */
export function mensagemDeErro(erro: unknown): string {
  if (!(erro instanceof HttpErrorResponse)) {
    return 'Ocorreu um erro inesperado.';
  }

  if (erro.status === 0) {
    return 'Não foi possível falar com o servidor. Verifique se os serviços estão no ar.';
  }

  const corpo = erro.error as RespostaErro | null;
  if (corpo?.erro?.mensagem) {
    return corpo.erro.mensagem;
  }

  return `Erro ${erro.status} ao processar a requisição.`;
}

/** Alguns erros pedem ação diferente na tela — 503 ganha botão de tentar de novo. */
export function ehIndisponivel(erro: unknown): boolean {
  return erro instanceof HttpErrorResponse && (erro.status === 503 || erro.status === 0);
}
