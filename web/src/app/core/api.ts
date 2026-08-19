/**
 * Endereços dos dois microsserviços.
 *
 * São dois hosts diferentes de propósito: o frontend fala com cada serviço
 * diretamente, sem um gateway na frente. Produtos vêm do estoque, notas vêm do
 * faturamento — a separação de responsabilidade aparece até aqui.
 */
export const API = {
  estoque: 'http://localhost:7080',
  faturamento: 'http://localhost:7081',
} as const;
