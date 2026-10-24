<script>
  // Traço e tamanho únicos (DESIGN.md §4): quem usa escolhe só o ícone e,
  // em botões compactos, um tamanho menor.
  export let icone;
  export let tamanho = 16;
  export let cor = 'currentColor';
</script>

<svelte:component this={icone} size={tamanho} strokeWidth={1.75} color={cor} aria-hidden="true" />
