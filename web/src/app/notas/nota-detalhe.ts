import { Component, OnDestroy, OnInit, computed, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { Subject, finalize, map, switchMap, takeUntil } from 'rxjs';

import { ehIndisponivel, mensagemDeErro } from '../core/erro';
import { Nota, Produto } from '../core/modelos';
import { ProdutoService } from '../produtos/produto.service';
import { NotaService } from './nota.service';

@Component({
  selector: 'app-nota-detalhe',
  imports: [
    ReactiveFormsModule,
    RouterLink,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressBarModule,
    MatProgressSpinnerModule,
    MatSelectModule,
    MatTableModule,
  ],
  templateUrl: './nota-detalhe.html',
  styleUrl: './nota-detalhe.css',
})
export class NotaDetalhe implements OnInit, OnDestroy {
  private readonly service = inject(NotaService);
  private readonly produtoService = inject(ProdutoService);
  private readonly rota = inject(ActivatedRoute);
  private readonly fb = inject(FormBuilder);
  private readonly aviso = inject(MatSnackBar);

  private readonly destruido$ = new Subject<void>();

  readonly nota = signal<Nota | null>(null);
  readonly produtos = signal<Produto[]>([]);
  readonly carregando = signal(false);
  readonly adicionando = signal(false);

  /** Controla o indicador de processamento exigido pelo enunciado. */
  readonly imprimindo = signal(false);

  /**
   * Erro da impressão fica separado do snackbar: ele precisa continuar visível
   * na tela, com o botão de tentar de novo ao lado, e não sumir em 5 segundos.
   */
  readonly erroImpressao = signal<string | null>(null);
  readonly podeTentarNovamente = signal(false);

  readonly colunas = ['codigo', 'descricao', 'quantidade', 'acoes'];

  /**
   * computed deriva estado a partir de outros signals e recalcula sozinho quando
   * qualquer um deles muda. Aqui centraliza a regra "nota fechada não se mexe",
   * que o template consulta em quatro lugares.
   */
  readonly estaAberta = computed(() => this.nota()?.status === 'ABERTA');
  readonly temItens = computed(() => (this.nota()?.itens.length ?? 0) > 0);
  readonly podeImprimir = computed(() => this.estaAberta() && this.temItens() && !this.imprimindo());

  readonly form = this.fb.nonNullable.group({
    produtoId: [0, [Validators.required, Validators.min(1)]],
    quantidade: [1, [Validators.required, Validators.min(1)]],
  });

  ngOnInit(): void {
    this.carregarProdutos();

    this.carregando.set(true);

    /**
     * paramMap é um Observable, não um valor: se o usuário navegar de /notas/1
     * para /notas/2 sem sair da tela, o Angular reaproveita o componente e o
     * ngOnInit não roda de novo — só o paramMap emite.
     *
     * switchMap cancela a busca anterior quando um id novo chega. Com mergeMap,
     * duas respostas poderiam voltar fora de ordem e a tela mostraria a nota
     * errada.
     */
    this.rota.paramMap
      .pipe(
        map((parametros) => Number(parametros.get('id'))),
        // O finalize vai DENTRO do switchMap, na chamada HTTP.
        //
        // Colocado fora, ele nunca dispararia: paramMap é um Observable de vida
        // longa que não completa enquanto a tela existe, e o switchMap absorve a
        // conclusão da chamada interna sem propagá-la. O indicador de progresso
        // só desligaria no ngOnDestroy — ou seja, ficaria girando a visita toda.
        switchMap((id) =>
          this.service.buscar(id).pipe(finalize(() => this.carregando.set(false))),
        ),
        takeUntil(this.destruido$),
      )
      .subscribe({
        next: (nota) => this.nota.set(nota),
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  ngOnDestroy(): void {
    this.destruido$.next();
    this.destruido$.complete();
  }

  private carregarProdutos(): void {
    this.produtoService
      .listar()
      .pipe(takeUntil(this.destruido$))
      .subscribe({
        next: (produtos) => this.produtos.set(produtos),
        // Falha aqui não trava a tela: a nota ainda pode ser vista e impressa,
        // só não dá para escolher produto novo.
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  adicionarItem(): void {
    const nota = this.nota();
    if (!nota || this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }

    const { produtoId, quantidade } = this.form.getRawValue();
    this.adicionando.set(true);

    this.service
      .adicionarItem(nota.id, produtoId, quantidade)
      .pipe(
        finalize(() => this.adicionando.set(false)),
        takeUntil(this.destruido$),
      )
      .subscribe({
        // O backend devolve a nota inteira atualizada, então não é preciso
        // recarregar nem remontar a lista na mão.
        next: (atualizada) => {
          this.nota.set(atualizada);
          this.form.reset({ produtoId: 0, quantidade: 1 });
        },
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  removerItem(itemId: number): void {
    const nota = this.nota();
    if (!nota) {
      return;
    }

    this.service
      .removerItem(nota.id, itemId)
      .pipe(takeUntil(this.destruido$))
      .subscribe({
        next: (atualizada) => this.nota.set(atualizada),
        error: (erro) => this.avisar(mensagemDeErro(erro)),
      });
  }

  /**
   * Imprimir fecha a nota e debita o estoque numa única ação do usuário.
   *
   * Os dois desfechos de erro são tratados diferente de propósito: 503 significa
   * que o estoque está fora e a nota continua ABERTA, então oferecemos tentar de
   * novo; 409 significa que o estoque respondeu e recusou (sem saldo), e aí
   * tentar de novo não adianta — o usuário precisa mudar a nota.
   */
  imprimir(): void {
    const nota = this.nota();
    if (!nota) {
      return;
    }

    this.imprimindo.set(true);
    this.erroImpressao.set(null);
    this.podeTentarNovamente.set(false);

    this.service
      .imprimir(nota.id)
      .pipe(
        finalize(() => this.imprimindo.set(false)),
        takeUntil(this.destruido$),
      )
      .subscribe({
        next: (atualizada) => {
          this.nota.set(atualizada);
          this.avisar('Nota ' + atualizada.numero + ' impressa. Estoque baixado.');
        },
        error: (erro) => {
          this.erroImpressao.set(mensagemDeErro(erro));
          this.podeTentarNovamente.set(ehIndisponivel(erro));
        },
      });
  }

  private avisar(mensagem: string): void {
    this.aviso.open(mensagem, 'Fechar', { duration: 5000 });
  }
}
