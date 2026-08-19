import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';

import { API } from '../core/api';
import { Nota } from '../core/modelos';

/**
 * Fala com o serviço de faturamento.
 *
 * Repare que a chave de idempotência não aparece aqui. Ela é gerada e guardada
 * pelo backend, na própria nota, antes de o estoque ser chamado — se o frontend
 * gerasse a chave, um F5 no meio da impressão criaria uma chave nova e o saldo
 * seria debitado duas vezes.
 */
@Injectable({ providedIn: 'root' })
export class NotaService {
  private readonly http = inject(HttpClient);
  private readonly url = `${API.faturamento}/notas`;

  listar(): Observable<Nota[]> {
    return this.http.get<Nota[]>(this.url);
  }

  buscar(id: number): Observable<Nota> {
    return this.http.get<Nota>(`${this.url}/${id}`);
  }

  criar(): Observable<Nota> {
    return this.http.post<Nota>(this.url, {});
  }

  adicionarItem(notaId: number, produtoId: number, quantidade: number): Observable<Nota> {
    return this.http.post<Nota>(`${this.url}/${notaId}/itens`, {
      produto_id: produtoId,
      quantidade,
    });
  }

  removerItem(notaId: number, itemId: number): Observable<Nota> {
    return this.http.delete<Nota>(`${this.url}/${notaId}/itens/${itemId}`);
  }

  /** Fecha a nota e debita o estoque. É a única chamada que atravessa os dois serviços. */
  imprimir(notaId: number): Observable<Nota> {
    return this.http.post<Nota>(`${this.url}/${notaId}/imprimir`, {});
  }
}
