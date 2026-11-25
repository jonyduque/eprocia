// Pacote atualizacao: verificação e troca de versão do aplicativo.
//
// É o único pacote do eprocIA que fala com a rede (e só com o host e o
// prefixo configurados nas Regras). Também é um contrato entre versões: o que
// uma versão grava ou espera de outra (formato de versão, nomes de arquivos)
// não pode mudar sem cuidado.
package atualizacao
