import { Component, OnDestroy, OnInit, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { Subject, finalize, takeUntil } from 'rxjs';

import { mensagemDeErro } from '../core/erro';
import { Produto } from '../core/modelos';
import { ProdutoService } from './produto.service';

@Component({
  selector: 'app-produtos',
  imports: [
    ReactiveFormsModule,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressBarModule,
    MatTableModule,
  ],
  templateUrl: './produtos.html',
  styleUrl: './produtos.css',
})
export class Produtos implements OnInit, OnDestroy {
  private readonly service = inject(ProdutoService);
  private readonly fb = inject(FormBuilder);
  private readonly aviso = inject(MatSnackBar);

  /**
   * Emite uma vez quando o componente é destruído. Todo Observable desta classe
   * passa por takeUntil(this.destruido$), então sair da tela cancela as
   * requisições em voo em vez de deixá-las escrevendo num componente que não
   * existe mais.
   */
  private readonly destruido$ = new Subject<void>();

  /**
   * Estado em signals, não em propriedades comuns.
   *
   * Esta aplicação roda em modo zoneless (Angular 22 não usa mais zone.js por
   * padrão): a detecção de mudanças só é disparada por signals e pelo async
   * pipe. Uma propriedade simples atualizada dentro de um subscribe mudaria o
   * valor sem a tela redesenhar.
   */
  readonly produtos = signal<Produto[]>([]);
  readonly carregando = signal(false);
  readonly salvando = signal(false);

  readonly colunas = ['codigo', 'descricao', 'saldo', 'acoes'];

  readonly form = this.fb.nonNullable.group({
    codigo: ['', Validators.required],
    descricao: ['', Validators.required],
    saldo: [0, [Validators.required, Validators.min(0)]],
  });

  /** Primeiro dos dois ciclos de vida usados: dispara a carga inicial. */
  ngOnInit(): void {
    this.carregar();
  }

  /**
   * Segundo ciclo de vida: encerra o Subject, o que faz todo takeUntil desta
   * classe completar. Sem isso, cada visita à tela deixaria uma inscrição viva —
   * vazamento clássico em Angular.
   */
  ngOnDestroy(): void {
    this.destruido$.next();
    this.destruido$.complete();
  }

  carregar(): void {
    this.carregando.set(true);

    this.service
      .listar()
      .pipe(
        // finalize roda tanto no sucesso quanto no erro. Sem ele, uma falha
        // deixaria a barra de progresso girando para sempre.
        finalize(() => this.carregando.set(false)),
        takeUntil(this.destruido$),
      )
      .subscribe({
        next: (produtos) => this.produtos.set(produtos),
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  salvar(): void {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    this.salvando.set(true);

    this.service
      .criar(this.form.getRawValue())
      .pipe(
        finalize(() => this.salvando.set(false)),
        takeUntil(this.destruido$),
      )
      .subscribe({
        next: (produto) => {
          // update em vez de set: cresce a lista a partir do valor atual, sem
          // precisar recarregar tudo do servidor.
          this.produtos.update((lista) => [...lista, produto]);
          this.form.reset({ codigo: '', descricao: '', saldo: 0 });
          this.avisar('Produto ' + produto.codigo + ' cadastrado.');
        },
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  remover(produto: Produto): void {
    this.service
      .remover(produto.id)
      .pipe(takeUntil(this.destruido$))
      .subscribe({
        next: () => {
          this.produtos.update((lista) => lista.filter((p) => p.id !== produto.id));
          this.avisar('Produto ' + produto.codigo + ' removido.');
        },
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  private avisar(mensagem: string): void {
    this.aviso.open(mensagem, 'Fechar', { duration: 5000 });
  }
}
