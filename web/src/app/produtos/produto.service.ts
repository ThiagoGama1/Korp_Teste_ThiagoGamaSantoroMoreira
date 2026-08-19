import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { API } from '../core/api';
import { Produto } from '../core/modelos';

/**
 * Fala com o serviço de estoque.
 *
 * `providedIn: 'root'` registra o service no injetor da aplicação: uma única
 * instância compartilhada, criada sob demanda. É a injeção de dependência do
 * Angular — nenhum componente dá `new` nele.
 *
 * Todo método devolve Observable e nenhum deles chama `.subscribe()`. Isso é
 * deliberado: o service descreve a requisição, e quem decide quando executá-la
 * (e quando cancelar) é o componente. Um Observable só dispara quando alguém se
 * inscreve — diferente de uma Promise, que já sai correndo ao ser criada.
 */
@Injectable({ providedIn: 'root' })
export class ProdutoService {
  private readonly http = inject(HttpClient);
  private readonly url = `${API.estoque}/produtos`;

  listar(): Observable<Produto[]> {
    return this.http.get<Produto[]>(this.url);
  }

  criar(dados: Pick<Produto, 'codigo' | 'descricao' | 'saldo'>): Observable<Produto> {
    return this.http.post<Produto>(this.url, dados);
  }

  atualizar(id: number, dados: Pick<Produto, 'codigo' | 'descricao' | 'saldo'>): Observable<Produto> {
    return this.http.put<Produto>(`${this.url}/${id}`, dados);
  }

  remover(id: number): Observable<void> {
    return this.http.delete<void>(`${this.url}/${id}`);
  }
}
