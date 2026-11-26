package etl

// Simulação do lexer de github.com/dslipak/pdf v0.0.2 (lex.go e o laço de
// Interpret em ps.go), só para prever se ele termina. No fim dos dados o
// readByte da biblioteca devolve '\n' para sempre, e um dicionário, array,
// string literal ou string hex sem fecho repete esse byte sem fim (a string
// literal ainda cresce a memória). A simulação segue as mesmas regras de
// tokens sobre os bytes já lidos e desiste quando lê o fim dos dados vezes
// demais. Ao atualizar a biblioteca, reveja esta cópia contra o lexer novo.

// leiturasNoFimMaximas é quantas vezes a simulação aceita ler além do fim dos
// dados: um stream que termina bem lê o fim no máximo umas poucas vezes.
const leiturasNoFimMaximas = 64

type lexKw string
type lexNome string
type lexOutro struct{}
type lexFim struct{}

type lexSim struct {
	dados     []byte
	pos       int
	noFim     int  // leituras feitas além do fim
	ultimaFim bool // o último readByte foi além do fim (unreadByte não volta)
	pendentes []any
	esgotado  bool
}

// lexerTermina diz se o lexer da biblioteca chega ao fim de dados quando
// Interpret os percorre.
func lexerTermina(dados []byte) (termina bool) {
	s := &lexSim{dados: dados}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(lexFim); !ok {
				panic(r)
			}
			termina = false
		}
	}()
	s.interpretar()
	return true
}

func (s *lexSim) lerByte() byte {
	if s.pos >= len(s.dados) {
		s.esgotado, s.ultimaFim = true, true
		if s.noFim++; s.noFim > leiturasNoFimMaximas {
			panic(lexFim{})
		}
		return '\n'
	}
	s.ultimaFim = false
	c := s.dados[s.pos]
	s.pos++
	return c
}

func (s *lexSim) devolverByte() {
	if !s.ultimaFim && s.pos > 0 {
		s.pos--
	}
}

func lexEspaco(c byte) bool {
	switch c {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

func lexDelim(c byte) bool {
	switch c {
	case '<', '>', '(', ')', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func lexHex(c byte) int {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0')
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10
	case 'A' <= c && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (s *lexSim) token() any {
	if n := len(s.pendentes); n > 0 {
		t := s.pendentes[n-1]
		s.pendentes = s.pendentes[:n-1]
		return t
	}
	c := s.lerByte()
	for {
		if lexEspaco(c) {
			if s.esgotado {
				return lexFim{}
			}
			c = s.lerByte()
		} else if c == '%' {
			for c != '\r' && c != '\n' {
				c = s.lerByte()
			}
		} else {
			break
		}
	}
	switch c {
	case '<':
		if s.lerByte() == '<' {
			return lexKw("<<")
		}
		s.devolverByte()
		s.hex()
		return lexOutro{}
	case '(':
		s.literal()
		return lexOutro{}
	case '[', ']', '{', '}':
		return lexKw(string(c))
	case '/':
		s.nome()
		return lexNome("")
	case '>':
		if s.lerByte() == '>' {
			return lexKw(">>")
		}
		s.devolverByte()
		return lexOutro{} // delimitador inesperado vira string de erro
	}
	if lexDelim(c) {
		return lexOutro{}
	}
	s.devolverByte()
	p := s.palavra()
	switch {
	case p == "true", p == "false", lexNumero(p):
		return lexOutro{}
	}
	return lexKw(p)
}

// nome segue readName: lê até um delimitador ou espaço, mas um '#' consome os
// dois bytes seguintes, sejam eles quais forem ("/A#]]" é um nome só).
func (s *lexSim) nome() {
	for {
		c := s.lerByte()
		if lexDelim(c) || lexEspaco(c) {
			s.devolverByte()
			return
		}
		if c == '#' {
			s.lerByte()
			s.lerByte()
		}
	}
}

// palavra segue readKeyword: lê até um delimitador ou espaço.
func (s *lexSim) palavra() string {
	ini := s.pos
	for {
		c := s.lerByte()
		if lexDelim(c) || lexEspaco(c) {
			s.devolverByte()
			break
		}
	}
	return string(s.dados[ini:s.pos])
}

// lexNumero segue isInteger e isReal da biblioteca.
func lexNumero(p string) bool {
	if len(p) > 0 && (p[0] == '+' || p[0] == '-') {
		p = p[1:]
	}
	if p == "" {
		return false
	}
	ponto := false
	for i := 0; i < len(p); i++ {
		switch c := p[i]; {
		case c == '.' && !ponto:
			ponto = true
		case c < '0' || c > '9':
			return false
		}
	}
	return p != "."
}

func (s *lexSim) hex() {
	for {
		c := s.lerByte()
		if c == '>' {
			return
		}
		if lexEspaco(c) {
			continue
		}
		c2 := s.lerByte()
		for lexEspaco(c2) {
			c2 = s.lerByte()
		}
		if lexHex(c)<<4|lexHex(c2) < 0 {
			return
		}
	}
}

func (s *lexSim) literal() {
	nivel := 1
	for {
		switch s.lerByte() {
		case '(':
			nivel++
		case ')':
			if nivel--; nivel == 0 {
				return
			}
		case '\\':
			switch c := s.lerByte(); c {
			case '\r':
				if s.lerByte() != '\n' {
					s.devolverByte()
				}
			case '0', '1', '2', '3', '4', '5', '6', '7':
				for range 2 {
					if c = s.lerByte(); c < '0' || c > '7' {
						s.devolverByte()
						break
					}
				}
			}
		}
	}
}

// objeto segue readObject com allowObjptr falso: ok=false é o erro que faz
// quem chamou desistir.
func (s *lexSim) objeto() (ok bool) {
	t := s.token()
	if kw, e := t.(lexKw); e {
		switch kw {
		case "null":
			return true
		case "<<":
			s.dicionario()
			return true
		case "[":
			s.array()
			return true
		}
		return false
	}
	return true
}

func (s *lexSim) dicionario() {
	for {
		t := s.token()
		if t == lexKw(">>") {
			return
		}
		if _, nome := t.(lexNome); !nome {
			continue
		}
		if !s.objeto() {
			return
		}
	}
}

func (s *lexSim) array() {
	for {
		t := s.token()
		if t == lexKw("]") {
			return
		}
		s.pendentes = append(s.pendentes, t)
		if !s.objeto() {
			return
		}
	}
}

func (s *lexSim) interpretar() {
	for {
		t := s.token()
		if _, fim := t.(lexFim); fim {
			return
		}
		if kw, e := t.(lexKw); e {
			switch kw {
			case "null", "[", "]", "<<", ">>":
			default:
				continue // operador
			}
		}
		s.pendentes = append(s.pendentes, t)
		if !s.objeto() {
			return
		}
	}
}
