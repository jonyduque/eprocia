// Estado do painel de reprocessar de uma linha: aberto, OCR sugerido ('' =
// o valor do item/config) e geração. O ReprocessarItem lê o OCR inicial só
// ao ser criado; a geração entra como chave ({#key}) para remontá-lo.
export const ESTADO_REPROCESSAR_INICIAL = Object.freeze({ aberto: false, ocr: '', geracao: 0 });

// Clique em "Reprocessar com OCR": sempre painel aberto, OCR em 'normal',
// qualquer que seja o estado anterior (fechado, aberto, ou já remontado antes
// e com o select trocado pelo usuário).
export function aplicarReprocessarComOCR(estado) {
  return { aberto: true, ocr: 'normal', geracao: (estado?.geracao ?? 0) + 1 };
}

// Ao fechar o formulário com o foco dentro dele: volta ao botão que o abriu
// (o da linha ou o "Reprocessar com OCR" do painel de atividade), se ainda
// estiver na página; senão, ao Reprocessar da linha.
export function alvoAoFecharReprocessar(origem, botaoLinha) {
  if (origem?.isConnected) return origem;
  return botaoLinha ?? null;
}
