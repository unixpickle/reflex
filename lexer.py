from dataclasses import dataclass


@dataclass
class Token:
    typ: str
    val: str
    line: int
    col: int


class LexError(Exception):
    pass


class Tokenizer:
    def __init__(self, src: str):
        self.src = src
        self.i = 0
        self.line = 1
        self.col = 1
        punctuation = set("{}[]().=,")
        self.singles = dict(zip(punctuation, punctuation)) | {
            "^": "PARENT",
            "@": "SELF",
            "\n": "NEWLINE",
            "!": "UNWRAP",
        }
        self.whitespace = set(" \t\r")

    def tokens(self) -> list[Token]:
        result = []
        while self.cur:
            if x := self.next_token():
                result.append(x)
        result.append(Token("EOF", "", self.line, self.col))
        return result

    def next_token(self) -> Token | None:
        ch = self.cur

        if ch in self.whitespace:
            self.adv()
            return None
        elif ch == "^" and self.peek() == "^":  # must run before singles
            res = Token("ANCESTOR", "^^", self.line, self.col)
            self.adv(2)
            return res
        elif ch == ":" and self.peek() == "=":  # must run before singles
            res = Token(":=", ":=", self.line, self.col)
            self.adv(2)
            return res
        elif ch in self.singles:
            res = Token(self.singles[ch], ch, self.line, self.col)
            self.adv()
            return res
        elif ch == '"' or ch == "'":
            return self.parse_string_lit()
        elif ch == "#":
            while (x := self.cur) and x != "\n":
                self.adv()
            return None
        elif ch.isdigit() or (ch == "-" and self.peek().isdigit()):
            return self.parse_int_lit()
        elif ch.isalpha() or ch == "_":
            return self.parse_identifier()
        else:
            raise LexError(f"Unexpected {ch!r} at {self.line}:{self.col}")

    @property
    def cur(self) -> str:
        if self.i < len(self.src):
            return self.src[self.i]
        else:
            return ""

    def adv(self, n: int = 1):
        for _ in range(n):
            if not self.cur:
                break
            if self.cur == "\n":
                self.line += 1
                self.col = 1
            else:
                self.col += 1
            self.i += 1

    def peek(self) -> str:
        if self.i + 1 >= len(self.src):
            return ""
        return self.src[self.i + 1]

    def parse_string_lit(self) -> Token:
        ch = self.cur
        assert ch == '"' or ch == "'"

        start_line = self.line
        start_col = self.col

        quote = ch
        buf = []
        escape = False
        while c := self.peek():
            self.adv()
            if escape:
                if c == "n":
                    buf.append("\n")
                elif c == "t":
                    buf.append("\t")
                elif c == "r":
                    buf.append("\r")
                elif c == "\\":
                    buf.append("\\")
                elif c == '"':
                    buf.append('"')
                elif c == "'":
                    buf.append("'")
                else:
                    buf.append("\\")
                    buf.append(c)
                escape = False
            if c == "\\":
                escape = True
            if c == quote:
                self.adv()  # go past closing quote
                return Token("STRING", "".join(buf), start_line, start_col)
            if c == "\n":
                raise LexError(
                    f"Unterminated string starting at {start_line}:{start_col}"
                )
            else:
                buf.append(c)
        raise LexError(f"Unterminated string starting at {start_line}:{start_col}")

    def parse_int_lit(self) -> Token:
        start_line = self.line
        start_col = self.col
        assert self.cur.isdigit() or (self.cur == "-" and self.peek().isdigit())
        buf = [self.cur]
        self.adv()
        while self.cur.isdigit():
            buf.append(self.cur)
            self.adv()
        return Token("INT", "".join(buf), start_line, start_col)

    def parse_identifier(self) -> Token:
        start_line = self.line
        start_col = self.col
        assert self.cur.isalpha() or self.cur == "_"
        buf = [self.cur]
        self.adv()
        while self.cur.isalnum() or self.cur == "_":
            buf.append(self.cur)
            self.adv()
        return Token("IDENT", "".join(buf), start_line, start_col)


def tokenize(src: str):
    return Tokenizer(src).tokens()
