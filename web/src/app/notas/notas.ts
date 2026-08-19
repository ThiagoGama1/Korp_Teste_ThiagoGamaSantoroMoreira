import { Component, OnDestroy, OnInit, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatChipsModule } from '@angular/material/chips';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { Router } from '@angular/router';
import { Subject, finalize, takeUntil } from 'rxjs';

import { mensagemDeErro } from '../core/erro';
import { Nota } from '../core/modelos';
import { NotaService } from './nota.service';

@Component({
  selector: 'app-notas',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatChipsModule,
    MatIconModule,
    MatProgressBarModule,
    MatTableModule,
  ],
  templateUrl: './notas.html',
  styleUrl: './notas.css',
})
export class Notas implements OnInit, OnDestroy {
  private readonly service = inject(NotaService);
  private readonly router = inject(Router);
  private readonly aviso = inject(MatSnackBar);

  private readonly destruido$ = new Subject<void>();

  readonly notas = signal<Nota[]>([]);
  readonly carregando = signal(false);
  readonly criando = signal(false);

  readonly colunas = ['numero', 'status', 'itens', 'acoes'];

  ngOnInit(): void {
    this.carregar();
  }

  ngOnDestroy(): void {
    this.destruido$.next();
    this.destruido$.complete();
  }

  carregar(): void {
    this.carregando.set(true);

    this.service
      .listar()
      .pipe(
        finalize(() => this.carregando.set(false)),
        takeUntil(this.destruido$),
      )
      .subscribe({
        next: (notas) => this.notas.set(notas),
        error: (erro) => this.aviso.open(mensagemDeErro(erro), 'Fechar', { duration: 5000 }),
      });
  }

  /** Cria a nota já vazia e leva direto para o detalhe, que é onde o trabalho acontece. */
  criar(): void {
    this.criando.set(true);

    this.service
      .criar()
      .pipe(
        finalize(() => this.criando.set(false)),
        takeUntil(this.destruido$),
      )
      .subscribe({
        next: (nota) => this.router.navigate(['/notas', nota.id]),
        error: (erro) => this.aviso.open(mensagemDeErro(erro), 'Fechar', { duration: 5000 }),
      });
  }

  abrir(nota: Nota): void {
    this.router.navigate(['/notas', nota.id]);
  }

  totalDeItens(nota: Nota): number {
    return nota.itens.reduce((soma, item) => soma + item.quantidade, 0);
  }
}
