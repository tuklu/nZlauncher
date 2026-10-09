package engine

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Minimal arithmetic evaluator: + - * / % ^ and parens, shunting-yard.
var exprPattern = regexp.MustCompile(`^[0-9.\s()+\-*/%^]+$`)

func evalExpr(s string) (string, bool) {
	if !exprPattern.MatchString(s) || !strings.ContainsAny(s, "+*/%^") &&
		!regexp.MustCompile(`\d\s*-`).MatchString(s) {
		return "", false
	}
	if !strings.ContainsAny(s, "0123456789") {
		return "", false
	}

	tokens, ok := tokenize(s)
	if !ok || len(tokens) < 3 {
		return "", false
	}
	rpn, ok := toRPN(tokens)
	if !ok {
		return "", false
	}
	v, ok := evalRPN(rpn)
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
		return "", false
	}
	return strconv.FormatFloat(v, 'f', -1, 64), true
}

func tokenize(s string) ([]string, bool) {
	var tokens []string
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ':
			i++
		case c >= '0' && c <= '9' || c == '.':
			j := i
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
				j++
			}
			tokens = append(tokens, s[i:j])
			i = j
		case strings.ContainsRune("+-*/%^()", rune(c)):
			// unary minus: fold into a zero subtraction
			if c == '-' && (len(tokens) == 0 || isOp(tokens[len(tokens)-1]) || tokens[len(tokens)-1] == "(") {
				tokens = append(tokens, "0")
			}
			tokens = append(tokens, string(c))
			i++
		default:
			return nil, false
		}
	}
	return tokens, true
}

func isOp(t string) bool {
	return len(t) == 1 && strings.Contains("+-*/%^", t)
}

var precedence = map[string]int{"+": 2, "-": 2, "*": 3, "/": 3, "%": 3, "^": 4}

func toRPN(tokens []string) ([]string, bool) {
	var out, ops []string
	for _, t := range tokens {
		switch {
		case isOp(t):
			for len(ops) > 0 && isOp(ops[len(ops)-1]) &&
				(precedence[ops[len(ops)-1]] > precedence[t] ||
					precedence[ops[len(ops)-1]] == precedence[t] && t != "^") {
				out = append(out, ops[len(ops)-1])
				ops = ops[:len(ops)-1]
			}
			ops = append(ops, t)
		case t == "(":
			ops = append(ops, t)
		case t == ")":
			for len(ops) > 0 && ops[len(ops)-1] != "(" {
				out = append(out, ops[len(ops)-1])
				ops = ops[:len(ops)-1]
			}
			if len(ops) == 0 {
				return nil, false
			}
			ops = ops[:len(ops)-1]
		default:
			out = append(out, t)
		}
	}
	for len(ops) > 0 {
		if ops[len(ops)-1] == "(" {
			return nil, false
		}
		out = append(out, ops[len(ops)-1])
		ops = ops[:len(ops)-1]
	}
	return out, true
}

func evalRPN(rpn []string) (float64, bool) {
	var stack []float64
	for _, t := range rpn {
		if isOp(t) {
			if len(stack) < 2 {
				return 0, false
			}
			b, a := stack[len(stack)-1], stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			var v float64
			switch t {
			case "+":
				v = a + b
			case "-":
				v = a - b
			case "*":
				v = a * b
			case "/":
				v = a / b
			case "%":
				v = math.Mod(a, b)
			case "^":
				v = math.Pow(a, b)
			}
			stack = append(stack, v)
		} else {
			f, err := strconv.ParseFloat(t, 64)
			if err != nil {
				return 0, false
			}
			stack = append(stack, f)
		}
	}
	if len(stack) != 1 {
		return 0, false
	}
	return stack[0], true
}
