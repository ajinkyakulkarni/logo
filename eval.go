package main

import (
	"strings"
)

var infixOps []string = []string{"+", "-", "*", "/", "(", ")", "=", "<>", "<", ">", "<=", ">=", "OR", "AND"}
var infixProc []string = []string{"SUM", "DIFFERENCE", "PRODUCT", "QUOTIENT", "", "", "EQUALP", "NOTEQUALP", "LESSP", "GREATERP", "LESSEQUALP", "GREATEREQUALP", "EITHER", "BOTH"}
var infixPrec []int = []int{1, 1, 2, 2, 3, 4, 4, 0, 0, 0, 0, 0, 0, 0, 0}

func isProcedure(wn *WordNode, ws *Workspace) bool {
	if wn.value[0] == ':' {
		return true
	}
	procName := strings.ToUpper(wn.value)
	return ws.findProcedure(procName) != nil
}

func callProcedure(frame Frame, node Node, withInfix bool) (*CallResult, Node) {

	wn := node.(*WordNode)
	if wn == nil {
		return errorResult(errorWordExpected(node)), nil
	}

	var proc Procedure
	var parameters []Node
	var procName string
	if wn.value[0] == ':' {
		proc = frame.workspace().findProcedure(keywordThing)
		procName = keywordThing
		c, r := wn.position()
		parameters = []Node{newWordNode(c, r, string(wn.value[1:]), true)}
		node = node.next()
	} else {
		if wn.isLiteral {
			return errorResult(errorProcedureExpected(node)), nil
		}
		procName = strings.ToUpper(wn.value)
		proc = frame.workspace().findProcedure(procName)

		if proc == nil {
			return errorResult(errorProcedureNotFound(node, wn.value)), nil
		}
		var err error
		if proc.parameterCount() > 0 {
			paramCount := proc.parameterCount()
			if proc.allowVarParameters() && wn.isFirstOfGroup {
				paramCount = -1
			}
			parameters, node, err = fetchParameters(frame, wn, node.next(), paramCount, withInfix)
			if err != nil {
				return errorResult(err), nil
			}
		} else {
			parameters = make([]Node, 0, 0)
			node = node.next()
		}
	}

	subFrame := proc.createFrame(frame, wn)
	frame.workspace().currentFrame = subFrame
	defer func() {
		frame.workspace().currentFrame = frame
	}()

	rv := subFrame.eval(parameters)

	if rv != nil {
		if rv.hasError() {
			return rv, nil
		}
	}
	return rv, node
}

func fetchParameters(frame Frame, caller *WordNode, firstNode Node, paramCount int, withInfix bool) ([]Node, Node, error) {

	var params []Node
	if paramCount > 0 {
		params = make([]Node, 0, paramCount)
	} else {
		params = make([]Node, 0, 2)
	}

	n := firstNode
	var rv *CallResult
	ix := 0
	for {
		switch nn := n.(type) {
		case *WordNode:
			rv, n = evaluateNode(frame, nn, withInfix)
			if rv != nil {
				if rv.hasError() {
					return nil, nil, rv.err
				}
				params = append(params, rv.returnValue)
			}

		case *GroupNode:
			rv, n = evaluateNode(frame, nn, withInfix)
			if rv != nil {
				if rv.hasError() {
					return nil, nil, rv.err
				}
				params = append(params, rv.returnValue)
			}

		case *ListNode:
			params = append(params, nn)
			n = nn.next()
		}

		ix++
		if (paramCount > 0 && ix == paramCount) || n == nil {
			break
		}
	}

	if ix < paramCount {
		return nil, nil, errorNotEnoughParameters(caller, firstNode)
	}

	return params, n, nil
}

func evalInstructionList(frame Frame, node Node, canReturn bool) *CallResult {

	switch ln := node.(type) {
	case *WordNode:
		return errorResult(errorListExpected(node))
	case *ListNode:
		return evalNodeStream(frame, ln.firstChild, canReturn)
	case *GroupNode:
		return evalNodeStream(frame, ln.firstChild, canReturn)
	}

	return nil
}

func getInfixOp(nodeVal string) int {
	if len(nodeVal) > 3 {
		return -1
	}
	uv := strings.ToUpper(nodeVal)
	for ix := 0; ix < len(infixOps); ix++ {
		if infixOps[ix] == uv {
			return ix
		}
	}

	return -1
}

func evaluateExpression(frame Frame, n Node) (*CallResult, Node) {

	nl := make([]Node, 0, 2)
	ops := make([]string, 0, 2)
	expectOp := false
	exit := false
	var prevIx int = -2
	for !exit && n != nil {

		switch nn := n.(type) {
		case *GroupNode:
			var rv *CallResult
			rv = evalNodeStream(frame, nn.firstChild, true)
			if rv != nil {
				if rv.shouldStop() {
					return rv, nil
				}
				nl = append(nl, rv.returnValue)
			}
			n = n.next()
		case *WordNode:
			ix := getInfixOp(nn.value)
			if ix >= 0 {
				if nn.value == "-" && (prevIx == -2 || (prevIx >= 0 && prevIx != 5)) && nn.next() != nil {
					// Looks like a unary minus
					var rv *CallResult
					rv, n = evaluateNode(frame, nn.next(), true)
					if rv.shouldStop() {
						return rv, nil
					}
					v, err := evalToNumber(rv.returnValue)
					if err != nil {
						return errorResult(err), nil
					}

					nwn := createNumericNode(0.0 - v).(*WordNode)
					nwn.isInfix = true
					nl = append(nl, nwn)
					expectOp = true
				} else {
					prec := infixPrec[ix]
					if len(ops) > 0 {
						pop := ops[len(ops)-1]
						popIx := getInfixOp(pop)
						for prec <= infixPrec[popIx] {
							l, c := nn.position()
							procName := infixProc[popIx]
							if procName != "" {
								nwn := newWordNode(l, c, procName, false)
								nwn.isInfix = true
								nl = append(nl, nwn)
							}
							ops = ops[0 : len(ops)-1]
							if len(ops) == 0 {
								break
							}
							pop = ops[len(ops)-1]
							popIx = getInfixOp(pop)
						}
					}
					ops = append(ops, nn.value)
					n = n.next()
					expectOp = false
				}
			} else {
				if expectOp {
					exit = true
				} else if !nn.isLiteral {
					var rv *CallResult
					rv, n = callProcedure(frame, nn, true)
					if rv != nil {
						if rv.shouldStop() {
							return rv, nil
						}
						if rv.returnValue != nil {
							nl = append(nl, rv.returnValue.clone())
							expectOp = true
						}
					}
				} else {
					nl = append(nl, nn.clone())
					n = n.next()
					expectOp = true
				}
			}
			prevIx = ix
		case *ListNode:
			exit = true
			break
		}
	}

	for len(ops) > 0 {
		pop := ops[len(ops)-1]
		popIx := getInfixOp(pop)

		procName := infixProc[popIx]
		if procName != "" {
			nwn := newWordNode(-1, -1, procName, false)
			nwn.isInfix = true
			nl = append(nl, nwn)
		}
		ops = ops[0 : len(ops)-1]
	}

	var rv *CallResult
	if len(nl) == 1 {
		rv, _ = evaluateNode(frame, nl[0], false)
	} else if len(nl) > 1 {

		ix := len(nl) - 1
		fn := nl[ix]
		nn := fn
		ix--
		for ix >= 0 {
			nn.addNode(nl[ix])
			nn = nn.next()
			ix--
		}

		rv, _ = evaluateNode(frame, fn, false)
	}
	if rv != nil {
		if rv.shouldStop() {
			return rv, nil
		}
		if rv.returnValue != nil {
			rv.returnValue.addNode(n)
		}
	}
	return rv, n
}

func evaluateNode(frame Frame, node Node, withInfix bool) (*CallResult, Node) {

	switch nn := node.(type) {
	case *WordNode:
		if withInfix {
			return evaluateExpression(frame, node)
		} else {
			if !nn.isLiteral {
				rv, node := callProcedure(frame, node, withInfix)
				if rv != nil && rv.shouldStop() {
					return rv, nil
				}
				return rv, node
			} else {
				return returnResult(nn), nn.next()
			}
		}
	case *GroupNode:
		return evalNodeStream(frame, nn.firstChild, true), node.next()

	case *ListNode:
		return returnResult(nn), node.next()
	}

	return nil, nil
}

func evalNodeStream(frame Frame, node Node, canReturnValue bool) *CallResult {

	intFrame, _ := findInterpretedFrame(frame)

	var lastValue Node = nil
	var rv *CallResult = nil

	for node != nil {
		switch n := node.(type) {
		case *ListNode:
			return errorResult(errorWordExpected(n))
		case *WordNode:
			rv, node = evaluateNode(frame, n, true)
			if rv != nil {
				if rv.hasError() {
					return rv
				}
				lastValue = rv.returnValue
			}
		case *GroupNode:
			rv = evalNodeStream(frame, n.firstChild, true)
			if rv != nil {
				if rv.hasError() {
					return rv
				}
				lastValue = rv.returnValue
			}
			node = node.next()
		}

		if rv != nil && rv.shouldStop() {
			break
		}

		if intFrame != nil && intFrame.stopped {
			break
		}
	}

	if canReturnValue && lastValue != nil {
		return returnResult(lastValue)
	}
	return nil
}