import { Routes } from '@angular/router';

/**
 * loadComponent carrega o componente sob demanda: o código de cada tela vira um
 * pacote separado, baixado só quando a rota é acessada. Numa aplicação deste
 * tamanho o ganho é pequeno, mas é o padrão que escala.
 */
export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'produtos' },
  {
    path: 'produtos',
    title: 'Produtos',
    loadComponent: () => import('./produtos/produtos').then((m) => m.Produtos),
  },
  {
    path: 'notas',
    title: 'Notas fiscais',
    loadComponent: () => import('./notas/notas').then((m) => m.Notas),
  },
  {
    path: 'notas/:id',
    title: 'Nota fiscal',
    loadComponent: () => import('./notas/nota-detalhe').then((m) => m.NotaDetalhe),
  },
  { path: '**', redirectTo: 'produtos' },
];
