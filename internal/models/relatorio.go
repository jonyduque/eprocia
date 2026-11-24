package models

// LimpezaRelatorio resume o efeito da limpeza sobre o texto dos documentos
// lidos: bytes e tokens estimados antes e depois, e os bytes das páginas que
// viraram referência de duplicata (H7). Fora das somas ficam páginas sem OCR,
// documentos omitidos e não lidos.
type LimpezaRelatorio struct {
	Nivel           string `json:"nivel"`
	BytesAntes      int64  `json:"bytes_antes"`
	BytesDepois     int64  `json:"bytes_depois"`
	BytesDuplicatas int64  `json:"bytes_duplicatas"`
	TokensAntes     int    `json:"tokens_antes"`
	TokensDepois    int    `json:"tokens_depois"`
}
