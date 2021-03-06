package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"
)

var promptPrimary = "? "
var promptSecondary = "> "
var greeting = "\nWelcome to Logo\n\n"

type Workspace struct {
	rootFrame    *RootFrame
	procedures   map[string]Procedure
	traceEnabled bool
	broker       *MessageBroker
	files        *Files
	screen       *Screen
	turtle       *Turtle
	glyphMap     *GlyphMap
	console      *ConsoleScreen
	editor       *Editor
	currentFrame Frame
}

func CreateWorkspace() *Workspace {

	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	ws := &Workspace{nil, make(map[string]Procedure, 100), false, nil, nil, nil, nil, nil, nil, nil, nil}
	ws.rootFrame = &RootFrame{ws, nil, nil, newVarList()}
	ws.currentFrame = ws.rootFrame
	ws.broker = CreateMessageBroker()
	ws.files = CreateFiles(path.Join(u.HomeDir, "logo"))
	registerBuiltInProcedures(ws)

	go ws.listen()

	return ws
}

func (this *Workspace) OpenScreen(w, h int) {

	this.screen = initScreen(this, w, h)
	this.turtle = initTurtle(this)
	this.glyphMap = initGlyphMap()
	this.console = initConsole(this, this.screen.screen.W(), this.screen.screen.H())
	this.editor = initEditor(this, this.screen.screen.W(), this.screen.screen.H())

	this.files.defaultFile = this.console
	this.files.writer = this.console
	this.files.reader = this.console

	this.screen.Open()
}

func (this *Workspace) listen() {

	c := this.broker.Subscribe("Workspace", MT_KeyPress, MT_EditStart, MT_EditStop)

	listening := true
	for m := c.Wait(); c != nil; m = c.Wait() {
		switch rm := m.(type) {
		case *MessageBase:
			switch m.MessageType() {
			case MT_EditStart:
				listening = false
			case MT_EditStop:
				listening = true
			}
		case *KeyMessage:
			switch rm.Sym {
			case K_ESCAPE:
				if listening && this.currentFrame != nil {
					procFrame, _ := findInterpretedFrame(this.currentFrame)
					if procFrame != nil {
						procFrame.abort()
					}
				}
			}
		}
	}
}

func (this *Workspace) Screen() *Screen { return this.screen }

func (this *Workspace) exit() {
	os.Exit(0)
}

func (this *Workspace) print(text string) error {

	err := this.files.writer.Write(text)
	return err
}

func (this *Workspace) setTrace(trace bool) {
	this.traceEnabled = trace
}

func (this *Workspace) trace(depth int, text string) error {

	if !this.traceEnabled {
		return nil
	}
	err := this.files.writer.Write(strings.Repeat(" ", depth) + "(" + fmt.Sprint(depth) + "):" + text + "\n")
	return err
}
func (this *Workspace) addProcedure(proc *InterpretedProcedure) {
	this.procedures[proc.name] = proc
}

func (this *Workspace) findProcedure(name string) Procedure {

	p, _ := this.procedures[name]

	return p
}

func (this *Workspace) registerBuiltIn(longName, shortName string, paramCount int, f evaluator) {
	p := &BuiltInProcedure{longName, paramCount, false, f}

	this.procedures[longName] = p
	if shortName != "" {
		this.procedures[shortName] = p
	}
}

func (this *Workspace) registerBuiltInWithVarParams(longName, shortName string, paramCount int, f evaluator) {
	p := &BuiltInProcedure{longName, paramCount, true, f}

	this.procedures[longName] = p
	if shortName != "" {
		this.procedures[shortName] = p
	}
}

func (this *Workspace) evaluate(source string) error {

	n, err := ParseString(source)
	if err != nil {
		return err
	}

	this.rootFrame.node = n
	defer func() {
		this.rootFrame.node = nil
	}()

	rv := this.rootFrame.eval(make([]Node, 0, 0))
	if rv != nil && rv.hasError() {
		return rv.err
	}
	return nil
}

func (this *Workspace) RunInterpreter() {
	this.print(greeting)

	go this.readFile()

	l := this.broker.Subscribe("Interpreter", MT_Quit)
	l.Wait()
}

func (this *Workspace) readString(text string) error {
	b := bytes.NewBufferString(text)
	s := bufio.NewScanner(b)

	fw := this.files.writer
	partial := ""
	definingProc := false
	for s.Scan() {
		line := s.Text()
		lu := strings.ToUpper(line)

		if definingProc {
			partial += "\n" + line
			if lu == keywordEnd {
				fn, err := ParseString(partial)
				if err != nil {
					return err
				} else {
					proc, _, err := readInterpretedProcedure(fn)
					if err != nil {
						return err
					} else {
						proc.source = partial
						this.addProcedure(proc)
						fw.Write(proc.name + " defined.\n")
					}
					partial = ""
					definingProc = false
				}
			}
		} else {
			if line == "" {
				continue
			}
			if strings.HasPrefix(lu, keywordTo) {
				definingProc = true
				partial = line
			} else {

				err := this.evaluate(line)
				if err != nil {
					return err
				}
			}
		}
	}

	return s.Err()
}

func (this *Workspace) readFile() error {
	prompt := promptPrimary
	definingProc := false
	partial := ""

	for {
		fw := this.files.writer
		fr := this.files.reader
		if fr.IsInteractive() {
			fw.Write(prompt)
		}
		line, err := fr.ReadLine()
		if err != nil {
			return err
		}
		lu := strings.ToUpper(line)

		if definingProc {
			partial += "\n" + line
			if lu == keywordEnd {
				fn, err := ParseString(partial)
				if err != nil {
					fw.Write(err.Error())
					fw.Write("\n")
				} else {
					proc, _, err := readInterpretedProcedure(fn)
					if err != nil {
						fw.Write(err.Error())
						fw.Write("\n")
					} else {
						proc.source = partial
						this.addProcedure(proc)
						fw.Write(proc.name + " defined.\n")
					}
					partial = ""
					prompt = promptPrimary
					definingProc = false
				}
			}
		} else {
			if line == "" {
				continue
			}
			if strings.HasPrefix(lu, keywordTo) {
				definingProc = true
				prompt = promptSecondary
				partial = line
			} else {
				if partial != "" {
					line = partial + "\n" + line
				}

				if strings.HasSuffix(lu, "~") {
					partial = line[0 : len(line)-1]
					prompt = promptSecondary
				} else {
					err = this.evaluate(line)
					partial = ""
					prompt = promptPrimary
					if err != nil {
						fw.Write(err.Error())
						fw.Write("\n")
					}
				}
			}
		}
	}
}
