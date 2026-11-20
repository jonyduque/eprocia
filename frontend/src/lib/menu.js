// Teclas do menu de opções (MenuNivel): índice da opção que recebe o foco,
// ou null quando a tecla não move.
export function moverFoco(atual, tecla, n) {
  if (n <= 0) return null;
  switch (tecla) {
    case 'ArrowDown':
      return (atual + 1) % n;
    case 'ArrowUp':
      return (atual - 1 + n) % n;
    case 'Home':
      return 0;
    case 'End':
      return n - 1;
    default:
      return null;
  }
}
