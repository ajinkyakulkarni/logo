package main

const (
	screenModeText = iota
	screenModeSplit
	screenModeGraphic
)

const splitScreenSize = 4

type Screen struct {
	screen     Window
	w, h       int
	ws         *Workspace
	screenMode int
	channel    *Channel
}

func initScreen(workspace *Workspace) *Screen {

	ss := newWindow(workspace.broker)
	w := ss.W()
	h := ss.H()

	s := &Screen{ss, w, h, workspace, screenModeSplit, workspace.broker.Subscribe(MT_UpdateText, MT_UpdateGfx)}

	workspace.registerBuiltIn("FULLSCREEN", "FS", 0, _s_Fullscreen)
	workspace.registerBuiltIn("TEXTSCREEN", "TS", 0, _s_Textscreen)
	workspace.registerBuiltIn("SPLITSCREEN", "SS", 0, _s_Splitscreen)

	return s
}

func (this *Screen) Open() {
	go this.Update()
}

func (this *Screen) Update() {

	gm := this.ws.glyphMap
	t := this.ws.turtle

	for m := this.channel.Wait(); m != nil; m = this.channel.Wait() {
		switch m.MessageType() {
		case MT_UpdateGfx:
			{
				rm := (m).(*RegionMessage)

				if this.screenMode == screenModeText {
					continue
				}

				if this.screenMode == screenModeSplit {
					th := gm.charHeight * splitScreenSize

					this.screen.SetClipRect(0, 0, this.w, this.h-th)
				}

				for _, r := range rm.regions {
					this.screen.ClearRect(t.screenColor, r.x, r.y, r.w, r.h)
					this.screen.DrawSurfacePart(r.x, r.y, rm.surface, r.x, r.y, r.w, r.h)
				}
				if t.turtleState == turtleStateShown {
					this.DrawTurtle()
				}

				this.screen.ClearClipRect()

				this.screen.Update()
			}

		case MT_UpdateText:
			{
				t := this.ws.turtle

				gm := this.ws.glyphMap
				c := this.ws.console
				cs := c.Surface()
				switch this.screenMode {
				case screenModeText:
					this.screen.DrawSurface(0, 0, cs)
				case screenModeSplit:
					th := gm.charHeight * splitScreenSize

					this.screen.DrawSurfacePart(0, this.h-th, cs,
						0, (1+c.FirstLineOfSplitScreen())*gm.charHeight, this.w, th)

					if t.turtleState == turtleStateShown {
						this.DrawTurtle()
					}
				}

				this.screen.Update()
			}
		}
	}
}

func (this *Screen) DrawTurtle() {
	t := this.ws.turtle
	x := int(t.x+float64(this.w/2)) - turtleSize
	y := int(-t.y+float64(this.h/2)) - turtleSize

	this.screen.DrawSurface(x, y, t.sprite)
}

type Region struct {
	x, y, w, h int
}

func (this *Region) Area() int {
	return this.w * this.h
}

func intMin(n1, n2 int) int {
	if n1 < n2 {
		return n1
	}
	return n2
}

func intMax(n1, n2 int) int {
	if n1 < n2 {
		return n2
	}
	return n1
}

func (this *Region) CombinedArea(other *Region) int {

	w := intMax(this.x, other.x) - intMin(this.x, other.x)
	h := intMax(this.y, other.y) - intMin(this.y, other.y)

	return w * h
}

func (this *Region) Contains(other *Region) bool {
	return this.x < other.x && this.y < other.y &&
		this.x+this.w > other.x+other.w &&
		this.y+this.h > other.y+other.h
}

func (this *Region) Combine(other *Region) {

	x1 := intMin(this.x, other.x)
	y1 := intMin(this.y, other.y)
	x2 := intMax(this.x, other.x)
	y2 := intMax(this.y, other.y)

	this.x = x1
	this.y = y1
	this.w = x2 - x1
	this.h = y2 - y1
}

type RegionMessage struct {
	MessageBase
	surface Surface
	regions []*Region
}

func newRegionMessage(messageType int, surface Surface, regions []*Region) *RegionMessage {
	return &RegionMessage{MessageBase{messageType}, surface, regions}
}

func _s_Fullscreen(frame Frame, parameters []Node) (Node, error) {

	ws := frame.workspace()
	ws.screen.screenMode = screenModeGraphic

	ws.broker.PublishId(MT_UpdateGfx)

	return nil, nil
}

func _s_Textscreen(frame Frame, parameters []Node) (Node, error) {

	ws := frame.workspace()
	ws.screen.screenMode = screenModeText

	ws.broker.PublishId(MT_UpdateGfx)

	return nil, nil
}

func _s_Splitscreen(frame Frame, parameters []Node) (Node, error) {

	ws := frame.workspace()
	ws.screen.screenMode = screenModeSplit

	ws.broker.PublishId(MT_UpdateGfx)

	return nil, nil
}
